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
				Required:    true,
			},
		},
	})

	if err != nil {
		fmt.Println("error registering command", err)
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"status": statusHandler,
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

func statusHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	server := i.ApplicationCommandData().Options[0].StringValue()
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

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: respBody.IP,
			Embeds: []*discordgo.MessageEmbed{
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
	})
	if err != nil {
		fmt.Println(err)
	}
	fasthttp.ReleaseResponse(resp)

}
