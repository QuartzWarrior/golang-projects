package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/valyala/fasthttp"
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

// Structure of my apis response, later to be direct ping and query
type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type Motd struct {
	Clean     string `json:"clean"`
	Html      string `json:"html"`
	Minecraft string `json:"minecraft"`
	Ansi      string `json:"ansi"`
	Raw       string `json:"raw"`
}

type Players struct {
	Online int      `json:"online"`
	Max    int      `json:"max"`
	List   []string `json:"list"`
}

type QueryPlayers struct {
	Online int      `json:"online"`
	Max    int      `json:"max"`
	List   []string `json:"list"`
}

type QuerySoftware struct {
	Version string   `json:"version"`
	Brand   string   `json:"brand"`
	Plugins []string `json:"plugins"`
}

type Query struct {
	Players  QueryPlayers  `json:"players"`
	Software QuerySoftware `json:"software"`
	Map      string        `json:"map"`
}

type MinecraftServer struct {
	Online             bool        `json:"online"`
	Players            Players     `json:"players"`
	Version            Version     `json:"version"`
	EnforcesSecureChat interface{} `json:"enforces_secure_chat"`
	Motd               Motd        `json:"motd"`
	Icon               string      `json:"icon"`
	Latency            float64     `json:"latency"`
	Query              *Query      `json:"query,omitempty"`
}

var client *fasthttp.Client

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
	server := ""
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
	client = &fasthttp.Client{}
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(fmt.Sprintf("http://85.215.55.208:3028/status?address=%s", server))
	req.Header.SetMethod("GET")
	resp := fasthttp.AcquireResponse()
	client.Do(req, resp)

	fasthttp.ReleaseRequest(req)

	var respBody MinecraftServer

	err := json.Unmarshal([]byte(string(resp.Body())), &respBody)
	if err != nil {
		fmt.Println(err)
	}

	if !respBody.Online {
		message := fmt.Sprintf("%s seems to be offline!", server)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	iconBytes, err := base64.StdEncoding.DecodeString(strings.Replace(respBody.Icon, "data:image/png;base64,", "", 1))
	if err != nil {
		fmt.Println("Error decoding base64 icon:", err)
	}

	iconReader := bytes.NewReader(iconBytes)

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Files: []*discordgo.File{
			{
				Name:        "icon.png",
				ContentType: "image/png",
				Reader:      iconReader,
			},
		},
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title: fmt.Sprintf("Status for %s", server),
				Type:  discordgo.EmbedTypeRich,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: "attachment://icon.png",
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    i.Member.User.Username,
					IconURL: i.Member.User.AvatarURL("16"),
				},
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Latency",
						Value:  fmt.Sprint(respBody.Latency),
						Inline: true,
					},
					{
						Name:   "Version",
						Value:  fmt.Sprint(respBody.Version.Name),
						Inline: true,
					},
					{
						Name:   "Players",
						Value:  fmt.Sprintf("%d/%d", respBody.Players.Online, respBody.Players.Max),
						Inline: false,
					},
					{
						Name:   "List",
						Value:  fmt.Sprint(respBody.Players.List),
						Inline: false,
					},
					{
						Name:   "Motd",
						Value:  respBody.Motd.Raw,
						Inline: false,
					},
				},
			},
		},
	},
	)
	fasthttp.ReleaseResponse(resp)

}
