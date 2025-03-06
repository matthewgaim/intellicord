package handlers

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/internal/ai"
)

var (
	commandHandlers = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	commands        = []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Replies with pong!",
		},
		{
			Name:        "ask",
			Description: "AI generated response to your message",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "question",
					Description: "Your question",
					Required:    true,
				},
			},
		},
	}
)

func InitCommands() {
	commandHandlers["ping"] = pingCommand()
	commandHandlers["ask"] = askCommand()
}

func pingCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong!",
			},
		})
	}
}

func askCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		userMessage := options[0].StringValue()

		// Defer the response to avoid a timeout
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		if err != nil {
			log.Println("Error deferring response:", err.Error())
			return
		}

		// Send an initial message to act as the parent of the thread
		initialMsg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: userMessage,
		})
		if err != nil {
			log.Println("Error sending initial message:", err.Error())
			return
		}

		if initialMsg == nil || initialMsg.ID == "" {
			log.Println("Initial message is nil or missing an ID")
			return
		}

		thread, err := s.MessageThreadStart(i.ChannelID, initialMsg.ID, userMessage, 60)
		if err != nil {
			log.Println("Error creating thread:", err.Error())
			return
		}

		firstMessage := fmt.Sprintf("-# Initial Message: %s", userMessage)
		_, err = s.ChannelMessageSend(thread.ID, firstMessage)
		if err != nil {
			fmt.Println("Error sending message in thread:", err)
		}

		response := ai.LlmGenerateText(nil, userMessage)
		_, err = s.ChannelMessageSend(thread.ID, response)
		if err != nil {
			fmt.Println("Error sending message in thread:", err)
		}
	}
}
