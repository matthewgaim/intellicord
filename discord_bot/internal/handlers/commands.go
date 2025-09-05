package handlers

import (
	"fmt"
	"log"
	"slices"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/internal/ai"
	"github.com/matthewgaim/intellicord/internal/db"
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
					MaxLength:   100,
				},
			},
		},
		{
			Name:        "addchannel",
			Description: "Allow This Channel to Use Intellicord",
		},
		{
			Name:        "delchannel",
			Description: "Don't Let This Channel Use Intellicord",
		},
	}
)

func InitCommands() {
	commandHandlers["ping"] = pingCommand()
	commandHandlers["ask"] = askCommand()
	commandHandlers["addchannel"] = addChannelCommand()
	commandHandlers["delchannel"] = removeChannelCommand()
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

		company, model, err := db.GetServersLLMConfig(i.GuildID)
		if err != nil {
			log.Println(err)
			sendResponseInChannel(s, thread.ID, "Can't find the LLM Model you chose.")
			return
		}

		var empty_history []*discordgo.Message
		response, err := ai.LlmGenerateText(empty_history, userMessage, company, s.State.User.ID, model)
		if err != nil {
			s.ChannelMessageSend(thread.ID, "Server error. Try again later")
		}
		_, err = s.ChannelMessageSend(thread.ID, response)
		if err != nil {
			fmt.Println("Error sending message in thread:", err)
		}
	}
}

func addChannelCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}

		selected_channel := i.ChannelID
		if i.Member.User.ID == guild.OwnerID {
			allowed_channels, err := db.GetAllowedChannels(i.GuildID)
			if err != nil {
				log.Println(err.Error())
				return
			}
			if slices.Contains(allowed_channels, selected_channel) {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Channel already allowed",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			} else {
				allowed_channels = append(allowed_channels, selected_channel)
				db.UpdateAllowedChannels(allowed_channels, guild.ID)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "✅ Channel can now use Intellicord",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
		} else {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You're not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}
}

func removeChannelCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}

		selected_channel := i.ChannelID
		if i.Member.User.ID == guild.OwnerID {
			allowed_channels, err := db.GetAllowedChannels(i.GuildID)
			if err != nil {
				log.Println(err.Error())
				return
			}
			if slices.Contains(allowed_channels, selected_channel) {
				allowed_channels = slices.DeleteFunc(allowed_channels, func(channel string) bool {
					return channel == selected_channel
				})
				db.UpdateAllowedChannels(allowed_channels, guild.ID)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "✅ Channel is no longer using Intellicord",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			} else {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Channel wasn't being used by Intellicord",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
		} else {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You're not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}
}
