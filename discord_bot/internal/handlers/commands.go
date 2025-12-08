package handlers

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

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
			Name:        "showchannels",
			Description: "Shows all channels Intellicord is allowed",
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
											Name:  "GPT-5.1",
											Value: "gpt-5.1",
										},
										{
											Name:  "GPT-5",
											Value: "gpt-5",
										},
										{
											Name:  "GPT-4o Mini",
											Value: "gpt-4o-mini",
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
										{
											Name:  "Gemini 3 Pro Preview",
											Value: "gemini-3-pro-preview",
										},
									},
								},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
					Name:        "custom",
					Description: "Custom LLM configuration (Ollama, Cerebras, etc.)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionSubCommand,
							Name:        "model",
							Description: "Set the custom model name (e.g., llama3.2, gpt-oss-120b, etc.)",
							Options: []*discordgo.ApplicationCommandOption{
								{
									Type:        discordgo.ApplicationCommandOptionString,
									Name:        "name",
									Description: "Enter the model name to use",
									Required:    true,
								},
							},
						},
					},
				},
			},
		},
		{
			Name:        "showconfig",
			Description: "Show server's LLM config & allowed channels",
		},
		{
			Name:        "banuser",
			Description: "Ban a user from using Intellicord on this server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to ban",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for the ban (i.e. Spamming)",
					Required:    true,
				},
			},
		},
		{
			Name:        "unbanuser",
			Description: "Unban a user from using Intellicord",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to unban",
					Required:    true,
				},
			},
		},
	}
)

func InitCommands() {
	commandHandlers["ping"] = pingCommand()
	commandHandlers["ask"] = askCommand()
	commandHandlers["addchannel"] = addChannelCommand()
	commandHandlers["delchannel"] = removeChannelCommand()
	commandHandlers["config"] = updateLLMConfig()
	commandHandlers["showconfig"] = showConfigCommand()
	commandHandlers["banuser"] = banUserCommand()
	commandHandlers["unbanuser"] = unbanUserCommand()
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
		sendResponseInChannel(s, thread.ID, response)
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

func showConfigCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}

		// 1. Fetch LLM Config
		company, model, err := db.GetServersLLMConfig(i.GuildID)
		if err != nil {
			log.Println("Error fetching LLM config:", err)
			company = "Error"
			model = "Error"
		}

		// 2. Fetch Allowed Channels
		allowedChannelIDs, err := db.GetAllowedChannels(i.GuildID)
		if err != nil {
			log.Println("Error fetching allowed channels:", err)
			allowedChannelIDs = nil
		}

		// 3. Prepare Channel List for Embed
		var channelList string
		if len(allowedChannelIDs) == 0 {
			channelList = "Intellicord is currently **disabled** in all channels. Use `/addchannel` to enable it."
		} else {
			var channelMentions []string
			for _, channelID := range allowedChannelIDs {
				channelMentions = append(channelMentions, fmt.Sprintf("<#%s>", channelID))
			}
			channelList = strings.Join(channelMentions, ", ")
			if len(channelList) > 1024 {
				channelList = "Too many channels to display. Total allowed: " + fmt.Sprintf("%d", len(allowedChannelIDs))
			}
		}

		embed := &discordgo.MessageEmbed{
			Title:       "⚙️ Intellicord Server Configuration",
			Color:       16776960,
			Description: fmt.Sprintf("Current settings for **%s**.", guild.Name),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "🤖 LLM Provider",
					Value:  fmt.Sprintf("**Company:** %s\n**Model:** %s", company, model),
					Inline: false,
				},
				{
					Name:   fmt.Sprintf("📢 Allowed Channels (%d total)", len(allowedChannelIDs)),
					Value:  channelList,
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Server ID: %s | Run by %s", i.GuildID, i.Member.User.Username),
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// 5. Respond with the Embed
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Printf("Error responding to interaction: %v", err)
		}
	}
}

func banUserCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}

		// Owner check
		if i.Member.User.ID != guild.OwnerID {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You are not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		options := i.ApplicationCommandData().Options
		userOption := options[0]
		reasonOption := options[1]

		userID := userOption.Value.(string)
		reason := reasonOption.StringValue()

		if userID == guild.OwnerID {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You can't ban the owner.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		err = db.AddNewBannedUser(userID, i.GuildID, reason)
		if err != nil {
			log.Printf("Error banning user %s: %v", userID, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to ban user. Database error.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		// Construct and send success response
		responseMessage := fmt.Sprintf("✅ User <@%s> has been banned from using Intellicord on this server for: **%s**", userID, reason)

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
	}
}

func unbanUserCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		guild, err := s.Guild(i.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}

		// Owner check (Restricting to the server owner)
		if i.Member.User.ID != guild.OwnerID {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You are not the owner!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		options := i.ApplicationCommandData().Options
		// The "user" option is the first one
		userOption := options[0]

		// Get the user ID from the user option
		userID := userOption.Value.(string)

		// Attempt to remove the ban from the database
		err = db.UnbanUser(userID, i.GuildID)
		if err != nil {
			log.Printf("Error unbanning user %s: %v", userID, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚨 Failed to unban user. Database error.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		// Construct and send success response
		responseMessage := fmt.Sprintf("✅ User <@%s> has been unbanned and can now use Intellicord on this server.", userID)

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
	}
}
