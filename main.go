package main

import (
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type substitution struct {
	regex        *regexp.Regexp
	substitution string
}

var (
	substitutions = []*substitution{
		{
			// Twitter
			regex:        regexp.MustCompile(`(?m)https://(x\.com|twitter\.com)/([a-zA-Z0-9_]{4,15}/status)`),
			substitution: "https://vxtwitter.com/$2",
		},
		{
			// Reddit
			regex:        regexp.MustCompile(`(?m)https://(www\.)?reddit\.com/(.*)`),
			substitution: "https://vxreddit.com/$2",
		},
		{
			// Instagram
			regex:        regexp.MustCompile(`(?m)https://(www\.)?instagram\.com/p/(.*)`),
			substitution: "https://eeinstagram.com/p/$2",
		},
		{
			// Bilibili
			regex:        regexp.MustCompile(`(?m)https://(www\.)?bilibili\.com/video/([a-zA-Z0-9_]+)`),
			substitution: "https://vxbilibili.com/video/$2",
		},
	}
	commands = []*discordgo.ApplicationCommand{
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
		{
			Name: "Delete Message",
			Type: discordgo.MessageApplicationCommand,
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
		"Delete Message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			commandData := i.ApplicationCommandData()
			message := commandData.Resolved.Messages[commandData.TargetID]
			log.Printf("Member %s requested to delete message %s", i.Member.User.Username, message.ID)

			if message.Author.Username == i.Member.User.Username {
				if err := s.ChannelMessageDelete(message.ChannelID, message.ID); err != nil {
					log.Println("Error deleting message: ", err)
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Failed to delete message",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				} else {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Message deleted",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}
			} else {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "You can only delete your own messages",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
		},
	}
)

func substituteTwitterLinks(input string) string {
	result := input
	for _, sub := range substitutions {
		if sub.regex != nil {
			result = sub.regex.ReplaceAllString(result, sub.substitution)
		}
	}
	return result
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

	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

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

	// Webhooks can't be created on threads, so use the parent channel's
	// webhook and target the thread when executing it
	webhookChannelID := m.ChannelID
	threadID := ""
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		channel, err = s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Error getting channel: ", err)
			return
		}
	}
	if channel.IsThread() {
		webhookChannelID = channel.ParentID
		threadID = channel.ID
	}

	webhook, err := getWebhook(s, webhookChannelID)
	if err != nil {
		log.Println("Error getting webhook: ", err)
		return
	}

	// Get author profile
	authorProfile, err := s.User(m.Author.ID)
	if err != nil {
		log.Println("Error getting user profile: ", err)
		return
	}

	// Send the message
	webhookMessage, err := s.WebhookThreadExecute(webhook.ID, webhook.Token, true, threadID, &discordgo.WebhookParams{
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

func getWebhook(s *discordgo.Session, channelID string) (*discordgo.Webhook, error) {
	webhooks, err := s.ChannelWebhooks(channelID)
	if err != nil {
		return nil, err
	}

	// Use any existing webhook we can execute. Channel follower webhooks and
	// webhooks owned by other applications don't expose a token.
	for _, w := range webhooks {
		if w.Type == discordgo.WebhookTypeIncoming && w.Token != "" {
			return w, nil
		}
	}

	// Create a webhook for the channel
	return s.WebhookCreate(channelID, "Twitter Substitution", "")
}
