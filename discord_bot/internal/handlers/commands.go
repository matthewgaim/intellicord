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
			Description: "Allow this channel to use Intellicord",
		},
		{
			Name:        "delchannel",
			Description: "Don't let this channel use Intellicord",
		},
		{
			Name:        "config",
			Description: "Choose LLM company and model",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
					Name:        "openai",
					Description: "OpenAI LLM configuration",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionSubCommand,
							Name:        "model",
							Description: "Choose an OpenAI model",
							Options: []*discordgo.ApplicationCommandOption{
								{
									Type:        discordgo.ApplicationCommandOptionString,
									Name:        "name",
									Description: "The model to use",
									Required:    true,
									Choices: []*discordgo.ApplicationCommandOptionChoice{
										{
											Name:  "GPT-4.1 Nano",
											Value: "gpt-4.1-nano",
										},
										{
											Name:  "GPT-5",
											Value: "gpt-5",
										},
										{
											Name:  "GPT-3.5 Turbo",
											Value: "gpt-3.5-turbo",
										},
									},
								},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
					Name:        "google",
					Description: "Google LLM configuration",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionSubCommand,
							Name:        "model",
							Description: "Choose a Google model",
							Options: []*discordgo.ApplicationCommandOption{
								{
									Type:        discordgo.ApplicationCommandOptionString,
									Name:        "name",
									Description: "The model to use",
									Required:    true,
									Choices: []*discordgo.ApplicationCommandOptionChoice{
										{
											Name:  "Gemini 2.5 Flash Lite",
											Value: "gemini-2.5-flash-lite",
										},
										{
											Name:  "Gemini 2.5 Flash",
											Value: "gemini-2.5-flash",
										},
										{
											Name:  "Gemini 2.5 Pro",
											Value: "gemini-2.5-pro",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:        "showconfig",
			Description: "Show server's LLM chosen",
		},
	}
)

func InitCommands() {
	commandHandlers["ping"] = pingCommand()
	commandHandlers["ask"] = askCommand()
	commandHandlers["addchannel"] = addChannelCommand()
	commandHandlers["delchannel"] = removeChannelCommand()
	commandHandlers["config"] = updateLLMConfig()
	commandHandlers["showconfig"] = showLLMConfig()
}

func updateLLMConfig() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}
		if i.Member.User.ID == guild.OwnerID {
			options := i.ApplicationCommandData().Options
			subcommandGroup := options[0]

			companyName := subcommandGroup.Name
			subcommand := subcommandGroup.Options[0]
			modelOption := subcommand.Options[0]
			modelName := modelOption.Value.(string)

			if err = db.UpdateServersLLMConfig(guild.ID, companyName, modelName); err != nil {
				log.Println(err.Error())
				return
			}

			// Construct the response message
			responseMessage := fmt.Sprintf("LLM configuration updated!\nCompany: **%s**\nModel: **%s**", companyName, modelName)

			// Respond to the user's interaction
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: responseMessage,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				log.Printf("Error responding to interaction: %v", err)
			}
		} else {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You are not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}
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

func showLLMConfig() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}
		if i.Member.User.ID == guild.OwnerID {
			company, model, err := db.GetServersLLMConfig(i.GuildID)
			if err != nil {
				log.Println(err)
				return
			}

			// Construct the response message
			responseMessage := fmt.Sprintf("Your server is using %s's %s", company, model)

			// Respond to the user's interaction
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: responseMessage,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				log.Printf("Error responding to interaction: %v", err)
			}
		} else {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You are not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}
}
