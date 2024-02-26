package main

import (
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	regexTwitter      = regexp.MustCompile(`(?m)https://(x\.com|twitter\.com)/([a-zA-Z0-9_]{4,15}/status)`)
	regexSubstitution = "https://vxtwitter.com/$2"
	commands          = []*discordgo.ApplicationCommand{
		{
			Name:        "twemb",
			Description: "Enable and disable Twitter link embedding",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "enable",
					Description: "Enable or disable Twitter link embedding for yourself",
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Required:    true,
				},
			},
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"twemb": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			userId := i.Member.User.ID
			opt := i.ApplicationCommandData().Options[0].BoolValue()
			var responseMessage string
			if opt {
				includeUser(userId)
				responseMessage = "Twitter link embedding has been enabled for you."
			} else {
				excludeUser(userId)
				responseMessage = "Twitter link embedding has been disabled for you."
			}

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: responseMessage,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				log.Println("Error responding to interaction: ", err)
			}
		},
	}
)

func substituteTwitterLinks(input string) string {
	return regexTwitter.ReplaceAllString(input, regexSubstitution)
}

func main() {
	loadExclusions()

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN env var is not set")
	}

	session, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	session.AddHandler(messageCreate)
	session.AddHandler(interactionCreate)

	session.Identify.Intents = discordgo.IntentsGuildMessages

	err = session.Open()
	if err != nil {
		log.Fatal("Error opening Discord session: ", err)
	}

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := session.ApplicationCommandCreate(session.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	log.Printf("Bot is now running as %s. Press CTRL+C to exit.", session.State.User.Username)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-sc

	log.Println("Shutting down bot...")
	session.Close()
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
		h(s, i)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	fixed := substituteTwitterLinks(m.Content)
	if fixed == m.Content {
		return
	}

	if isUserExcluded(m.Author.ID) {
		log.Printf("User %s is excluded from Twitter link embedding", m.Author.ID)
		return
	}

	var webhook *discordgo.Webhook = nil
	webhooks, err := s.ChannelWebhooks(m.ChannelID)
	if err != nil {
		log.Println("Error getting webhooks: ", err)
		return
	}

	if len(webhooks) > 0 {
		// Use any existing webhook
		webhook = webhooks[0]
	} else {
		// Create a webhook for the channel
		webhook, err = s.WebhookCreate(m.ChannelID, "Twitter Substitution", "")
		if err != nil {
			log.Println("Error creating webhook: ", err)
			return
		}
	}

	// Get author profile
	authorProfile, err := s.User(m.Author.ID)
	if err != nil {
		log.Println("Error getting user profile: ", err)
		return
	}

	// Send the message
	webhookMessage, err := s.WebhookExecute(webhook.ID, webhook.Token, true, &discordgo.WebhookParams{
		Content:   fixed,
		Username:  authorProfile.Username,
		AvatarURL: authorProfile.AvatarURL(""),
	})
	if err != nil {
		log.Println("Error sending webhook message: ", err)
		return
	}

	log.Printf("Sent message %s", webhookMessage.ID)

	// Delete the original message
	err = s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
	if err != nil {
		log.Println("Error deleting message: ", err)
		return
	}
}
