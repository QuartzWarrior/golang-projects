package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
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

// Structure of mcsrvstat.us v2 api response, soon to be my own api
type Debug struct {
	Ping          bool  `json:"ping"`
	Query         bool  `json:"query"`
	Srv           bool  `json:"srv"`
	QueryMismatch bool  `json:"querymismatch"`
	IpInSrv       bool  `json:"ipinsrv"`
	CnameInSrv    bool  `json:"cnameinsrv"`
	AnimatedMotd  bool  `json:"animatedmotd"`
	CacheHit      bool  `json:"cachehit"`
	CacheTime     int64 `json:"cachetime"`
	CacheExpire   int64 `json:"cacheexpire"`
	ApiVersion    int   `json:"apiversion"`
	DNS           struct {
		Srv []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Class       string `json:"class"`
			TTL         int    `json:"ttl"`
			RdLength    int    `json:"rdlength"`
			RData       string `json:"rdata"`
			Priority    int    `json:"priority"`
			Weight      int    `json:"weight"`
			Port        int    `json:"port"`
			Target      string `json:"target"`
			TypeCovered string `json:"typecovered,omitempty"`
			Algorithm   int    `json:"algorithm,omitempty"`
			Labels      int    `json:"labels,omitempty"`
			OrigTTL     int    `json:"origttl,omitempty"`
			SigExp      string `json:"sigexp,omitempty"`
			SigIncep    string `json:"sigincep,omitempty"`
			KeyTag      int    `json:"keytag,omitempty"`
			SignName    string `json:"signname,omitempty"`
			Signature   string `json:"signature,omitempty"`
		} `json:"srv"`
		SrvA []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Class       string `json:"class"`
			TTL         int    `json:"ttl"`
			RdLength    int    `json:"rdlength"`
			RData       string `json:"rdata"`
			CName       string `json:"cname,omitempty"`
			Address     string `json:"address,omitempty"`
			TypeCovered string `json:"typecovered,omitempty"`
			Algorithm   int    `json:"algorithm,omitempty"`
			Labels      int    `json:"labels,omitempty"`
			OrigTTL     int    `json:"origttl,omitempty"`
			SigExp      string `json:"sigexp,omitempty"`
			SigIncep    string `json:"sigincep,omitempty"`
			KeyTag      int    `json:"keytag,omitempty"`
			SignName    string `json:"signname,omitempty"`
			Signature   string `json:"signature,omitempty"`
		} `json:"srv_a"`
	} `json:"dns"`
	Error struct {
		Query string `json:"query"`
	} `json:"error"`
}

type Motd struct {
	Raw   []string `json:"raw"`
	Clean []string `json:"clean"`
	HTML  []string `json:"html"`
}

type Players struct {
	Online int `json:"online"`
	Max    int `json:"max"`
}

type MinecraftServer struct {
	IP           string  `json:"ip"`
	Port         int     `json:"port"`
	Debug        Debug   `json:"debug"`
	Motd         Motd    `json:"motd"`
	Players      Players `json:"players"`
	Version      string  `json:"version"`
	Online       bool    `json:"online"`
	Protocol     int     `json:"protocol"`
	ProtocolName string  `json:"protocol_name"`
	Hostname     string  `json:"hostname"`
	Icon         string  `json:"icon"`
	EulaBlocked  bool    `json:"eula_blocked"`
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
	req.SetRequestURI(fmt.Sprintf("https://api.mcsrvstat.us/2/%s", server))
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

	message := respBody.IP
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &message,
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title: fmt.Sprintf("Status for %s", server),
				Type:  discordgo.EmbedTypeRich,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: fmt.Sprintf("https://api.mcsrvstat.us/icon/%s.png", server),
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    i.Member.User.Username,
					IconURL: i.Member.User.AvatarURL("16"),
				},
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "IP",
						Value:  respBody.IP,
						Inline: true,
					},
					{
						Name:   "Port",
						Value:  fmt.Sprint(respBody.Port),
						Inline: true,
					},
					{
						Name:   "Version",
						Value:  respBody.Version,
						Inline: true,
					},
					{
						Name:   "List",
						Value:  fmt.Sprintf("%d/%d", respBody.Players.Online, respBody.Players.Max),
						Inline: true,
					},
					{
						Name:   "Motd",
						Value:  respBody.Motd.Raw[0],
						Inline: true,
					},
				},
			},
		},
	},
	)
	fasthttp.ReleaseResponse(resp)

}
