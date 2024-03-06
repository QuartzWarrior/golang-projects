package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mcpinger "github.com/Raqbit/mc-pinger"
	"github.com/bwmarrin/discordgo"
)

var (
	token   = ""
	guildID = 0
)

// Structure for json file data
type serverData map[string]Server

type Server struct {
	Address string `json:"address"`
}

func main() {
	dg, err := discordgo.New(fmt.Sprintf("Bot %s", token))

	if err != nil {
		fmt.Println(err)
	}

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Printf("Logged in as: %v#%v\n", s.State.User.Username, s.State.User.Discriminator)
	})

	dg.Identify.Intents = discordgo.IntentsAll

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, fmt.Sprint(guildID), &discordgo.ApplicationCommand{
		Name:        "status",
		Description: "Check the status of Minecraft Server",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "server",
				Description: "The ip or hostname of the selected server",
				Required:    false,
			},
		},
	})

	if err != nil {
		fmt.Println("Error registering command: ", err)
	}

	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, fmt.Sprint(guildID), &discordgo.ApplicationCommand{
		Name:        "server",
		Description: "Set the default server for this guild.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "address",
				Description: "The ip or hostname of the server.",
				Required:    true,
			},
		},
	})

	if err != nil {
		fmt.Println("Error registering command: ", err)
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"status": statusHandler,
		"server": setServerHandler,
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()

}

func setServerHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	server := i.ApplicationCommandData().Options[0].StringValue()
	file, err := os.Open("data.json")
	if err != nil {
		fmt.Println("Error during file opening: ", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error opening the JSON file. Try again later.",
			},
		})
		return
	}
	var data serverData
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		fmt.Println("Error during json decoding: ", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error decoding the JSON file. Try again later.",
			},
		})
		return
	}

	data[i.GuildID] = Server{Address: server}

	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Println("Error during json formatting: ", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error formatting the JSON file. Try again later.",
			},
		})
		return
	}

	err = os.WriteFile("data.json", jsonData, 0644)
	if err != nil {
		fmt.Println("Error during json writing: ", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error writing to the JSON file. Try again later.",
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully set default server to %s", server),
		},
	})

}

func statusHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	startTotalTime := time.Now()
	server := ""
	port := uint16(25565)
	if len(i.ApplicationCommandData().Options) < 1 {
		file, err := os.Open("data.json")
		if err != nil {
			fmt.Println("Error during file opening: ", err)
			message := "There was an error opening the JSON file. Try again later."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &message,
			})
			return
		}
		var data serverData
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&data)
		if err != nil {
			fmt.Println("Error during json decoding: ", err)
			message := "There was an error decoding the JSON file. Try again later."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &message,
			})
			return
		}

		serverData, exists := data[i.GuildID]
		if exists {
			server = serverData.Address
		} else {
			message := fmt.Sprintf("Rerun this command using </status:%s>, this time providing the server ip.\n\nYou may also set the default server for this guild via /server.", i.Interaction.ID)
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &message,
			})
			return
		}
	} else {
		server = i.ApplicationCommandData().Options[0].StringValue()
	}
	if strings.Contains(server, ":") {
		portSplit := strings.Split(server, ":")
		server = portSplit[0]
		port64, _ := strconv.ParseUint(portSplit[1], 10, 16)
		port = uint16(port64)
	}

	serverPinger := mcpinger.New(server, port, mcpinger.WithTimeout(3*time.Second))

	start := time.Now()
	info, err := serverPinger.Ping()
	elapsed := time.Since(start)
	milliseconds := elapsed.Seconds() * 1000

	if err != nil {

		message := fmt.Sprintf("%s seems to be offline!", server)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	iconBytes, err := base64.StdEncoding.DecodeString(strings.Replace(info.Favicon, "data:image/png;base64,", "", 1))
	if err != nil {
		fmt.Println("Error decoding base64 icon:", err)
	}

	iconReader := bytes.NewReader(iconBytes)

	img, _, err := image.Decode(iconReader)
	if err != nil {
		fmt.Println("Error decoding image:", err)
		return
	}

	bounds := img.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	centerX := width / 2
	centerY := height / 2

	centerPixel := img.At(centerX, centerY)
	r, g, b, _ := centerPixel.RGBA()

	hexValue := (int(r>>8) << 16) + (int(g>>8) << 8) + int(b>>8)

	iconReader = bytes.NewReader(iconBytes)

	elapsedTotalTime := time.Since(startTotalTime)
	totalTime := elapsedTotalTime.Seconds() * 1000

	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Files: []*discordgo.File{
			{
				Name:        "icon.png",
				ContentType: "image/png",
				Reader:      iconReader,
			},
		},
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title:       fmt.Sprintf("Status for %s", server),
				Description: fmt.Sprintf("Stats retrieved asynchronously for %s. Took %.2fms total.", server, totalTime),
				Type:        discordgo.EmbedTypeRich,
				Color:       hexValue,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: "attachment://icon.png",
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    i.Member.User.Username,
					IconURL: i.Member.User.AvatarURL("16"),
				},
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf("http://85.215.55.208:3028/icon/%s?online=%d&max=%d", server, info.Players.Online, info.Players.Max),
				},
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Latency",
						Value:  fmt.Sprintf("%.2fms", milliseconds),
						Inline: true,
					},
					{
						Name:   "Version",
						Value:  fmt.Sprint(info.Version.Name),
						Inline: true,
					},
					{
						Name:   "Players",
						Value:  fmt.Sprintf("%d/%d", info.Players.Online, info.Players.Max),
						Inline: false,
					},
					{
						Name:   "List",
						Value:  fmt.Sprint(info.Players.Sample),
						Inline: false,
					},
					{
						Name:   "Motd",
						Value:  info.Description.Text,
						Inline: false,
					},
				},
			},
		},
	},
	)

	if err != nil {
		fmt.Println("There was an error sending the message: ", err)
		return
	}

}
