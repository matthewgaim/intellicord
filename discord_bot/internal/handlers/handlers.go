package handlers

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/internal/ai"
	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/matthewgaim/intellicord/internal/guilds"
	"github.com/openai/openai-go"
)

func CommandLookupHandler() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	}
}

func BotReadyRegisterCommandsHandler(dg *discordgo.Session) func(s *discordgo.Session, r *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		for _, g := range r.Guilds {
			log.Printf("Registering commands for existing server: %s\n", g.ID)
			go guilds.RegisterCommandsForGuild(dg, g.ID, commands)
		}
		dg.UpdateCustomStatus("Upload a file, or type /ask to use Intellicord")
	}
}

func BotAddedToServerHandler() func(s *discordgo.Session, g *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		// needed bc GuildCreate is triggered when joining guild AND bot startup
		if time.Since(g.JoinedAt) < time.Minute {
			guildName := g.Name
			guildID := g.ID
			guildOwnerID := g.OwnerID
			message := fmt.Sprintf("Joined a new server: %s %s (Owner ID: %s)", guildName, guildID, guildOwnerID)
			go NewDiscordWebhookMessage("https://discord.com/api/webhooks/1347311739573633064/rsRD8kaookveaDjAdp8DVUR20IdtsUyz6AV8_85YfB2n9MrKFsIgyS9q82nEGUXqPxyV", message)
			db.AddGuildToDB(guildID, guildOwnerID)
			guilds.RegisterCommandsForGuild(s, g.ID, commands)
		}
	}
}

func BotRemovedFromServerHandler() func(s *discordgo.Session, g *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		log.Printf("Removed from server: %s (ID: %s)", g.Guild.Name, g.Guild.ID)
		guilds.DeleteCommandsForGuild(s, g.Guild.ID)
		db.RemoveGuildFromDB(g.Guild.ID)
	}
}

func BotRespondToThreadHandler() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// dont let bot respond to itself or other bots
		if m.Author.Bot {
			return
		}

		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Error fetching channel:", err)
			return
		}

		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			// ignore message in a non-bot created thread
			if channel.OwnerID != s.State.User.ID {
				return
			}

			guild, err := s.Guild(m.GuildID)
			if err != nil {
				log.Println("Error getting guild")
				return
			}
			_, messageLimitReached, err := db.CheckOwnerLimits(guild.OwnerID)
			if err != nil {
				log.Printf("Error getting owners limits: %v", err)
			}
			if messageLimitReached {
				sendResponseInChannel(s, channel.ID, "Maximum message limit reached. Upgrade for more messages")
				return
			}

			s.ChannelTyping(channel.ID)

			// Don't recognize extra files in a thread
			if len(m.Attachments) > 0 {
				s.ChannelMessageDelete(channel.ID, m.Message.ID)
				sendResponseInChannel(s, channel.ID, "Attached document will not be recognized in context")
				return
			}

			history, err := GetThreadMessages(s, channel.ID, s.State.User.ID)
			if err != nil {
				log.Printf("Error getting thread messages: %v\n", err.Error())
				return
			}
			if channel.OwnerID == s.State.User.ID {
				rootMsg, err := getRootMessageOfThread(s, channel)
				if err != nil {
					log.Printf("Error getting root message: %v", err)
				}
				numOfAttachments := len(rootMsg.Attachments)
				rootMsgID := rootMsg.ID

				go db.AddMessageLog(m.Message.ID, m.GuildID, m.ChannelID, m.Author.ID)
				res := ai.QueryVectorDB(context.Background(), m.Content, rootMsgID, numOfAttachments)
				history = append(history, openai.SystemMessage(fmt.Sprintf("Additional Context:\n%s", res)))
				response, err := ai.LlmGenerateText(history, m.Content)
				if err != nil {
					sendResponseInChannel(s, m.ChannelID, "Server error. Try again later.")
				}
				sendResponseInChannel(s, m.ChannelID, response)
				if err != nil {
					log.Println("Error sending message in thread:", err)
				}
			}
		}
	}
}

func StartThreadFromAttachmentUploadHandler() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if len(m.Attachments) == 0 || m.Author.Bot {
			return
		}

		allowedChannels, err := db.GetAllowedChannels(m.GuildID)
		if err != nil {
			log.Printf("Error fetching allowed channels: %v", err)
			return
		}

		// users can still upload files elsewhere, intellicord just wont do anything
		if !slices.Contains(allowedChannels, m.ChannelID) {
			return
		}

		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Error fetching channel:", err)
			return
		}
		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			log.Println("Documents wont be recognized in an existing thread")
			return
		}

		guild, err := s.Guild(m.GuildID)
		if err != nil {
			log.Println("Error getting guild")
			return
		}
		_, messageLimitReached, err := db.CheckOwnerLimits(guild.OwnerID)
		if err != nil {
			log.Printf("Error getting owners limits: %v", err)
		}
		if messageLimitReached {
			sendResponseInChannel(s, channel.ID, "Maximum message limit reached. Upgrade for more messages")
			return
		}

		data := &discordgo.ThreadStart{
			Name: m.Attachments[0].Filename,
		}
		thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, data)
		if err != nil {
			log.Printf("Error creating thread: %v", err)
			return
		}
		s.ChannelTyping(thread.ID)
		for i, attachment := range m.Attachments {
			attachmentLink := attachment.URL
			filename := attachment.Filename
			log.Printf("Attachment %d: %s (%s)", i, filename, attachmentLink)
			processingMessage, err := s.ChannelMessageSend(thread.ID, fmt.Sprintf("-# 🔎 Reading file: %s", filename))
			fileText, fileSize, err := getFileTextAndSize(attachmentLink)
			if err != nil {
				err_str := err.Error()
				if err_str == "Unsupported file type" {
					s.ChannelMessageEdit(thread.ID, processingMessage.ID, fmt.Sprintf("-# 🚨 File '%s' is an unsupported file type. It wont be analyzed.", filename))
					continue
				} else if err_str == "File too large" {
					s.ChannelMessageEdit(thread.ID, processingMessage.ID, fmt.Sprintf("-# 🚨 File '%s' is too big.", filename))
					continue
				} else {
					log.Printf("Error getting file text: %v", err)
					continue
				}
			}
			discord_server_id := m.GuildID
			err = ai.ChunkAndEmbed(context.Background(), m.Message.ID, fileText, filename, attachmentLink, discord_server_id, fileSize, thread.ID, m.Author.ID)
			if err != nil {
				s.ChannelMessageEdit(thread.ID, processingMessage.ID, fmt.Sprintf("-# 🚨 There was an error processing file '%s'", filename))
				continue
			}
			_, err = s.ChannelMessageEdit(thread.ID, processingMessage.ID, fmt.Sprintf("-# ✅ File '%s' is ready!", filename))
			if err != nil {
				log.Printf("Error sending message in thread: %v", err)
			}
		}

		// If user sent a message with the files
		if strings.Trim(m.Content, " ") != "" {
			s.ChannelTyping(thread.ID)
			go db.AddMessageLog(m.Message.ID, m.GuildID, m.ChannelID, m.Author.ID)
			numOfAttachments := len(m.Attachments)
			res := ai.QueryVectorDB(context.Background(), m.Content, m.ID, numOfAttachments)
			history := []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(fmt.Sprintf("Context:\n%s", res)),
				openai.UserMessage(fmt.Sprintf("%s: %s", m.Author.Username, m.Content)),
			}
			response, err := ai.LlmGenerateText(history, m.Content)
			if err != nil {
				sendResponseInChannel(s, thread.ID, "Server error. Try again later.")
			}
			_, err = s.ChannelMessageSend(thread.ID, response)
			if err != nil {
				log.Println("Error sending message in thread:", err)
			}
		}
	}
}

func AttachmentDeletedHandler() func(s *discordgo.Session, m *discordgo.MessageDelete) {
	return func(s *discordgo.Session, m *discordgo.MessageDelete) {
		// cant check for attachments since message is deleted
		message_id := m.Message.ID
		ai.DeleteEmbeddings(context.Background(), message_id)
	}
}

func StartThreadFromReplyHandler() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.Bot || m.Type != discordgo.MessageTypeReply {
			return
		}
		if m.Type == discordgo.MessageTypeReply && len(m.ReferencedMessage.Attachments) == 0 {
			return
		}

		discord_server_id := m.GuildID
		allowedChannels, err := db.GetAllowedChannels(discord_server_id)
		if err != nil {
			log.Printf("Error fetching allowed channels: %v", err)
			return
		}
		if !slices.Contains(allowedChannels, m.ChannelID) {
			return
		}

		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Error fetching channel:", err)
			return
		}
		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			log.Println("Documents wont be recognized in an existing thread")
			return
		}
		data := &discordgo.ThreadStart{
			Name: m.ReferencedMessage.Attachments[0].Filename,
		}
		thread, err := s.MessageThreadStartComplex(channel.ID, m.ID, data)
		if err != nil {
			log.Printf("Error creating thread: %v", err)
			return
		}
		log.Println("MessageThreadStartComplex")
		s.ChannelTyping(thread.ID)

		history, err := GetThreadMessages(s, thread.ID, s.State.User.ID)
		if err != nil {
			log.Printf("Error getting thread messages: %v\n", err.Error())
			return
		}
		res := ai.QueryVectorDB(context.Background(), m.Content, m.ReferencedMessage.ID, len(m.ReferencedMessage.Attachments))
		history = append(history, openai.SystemMessage(fmt.Sprintf("Additional Context:\n%s", res)))
		response, err := ai.LlmGenerateText(history, m.Content)
		if err != nil {
			s.ChannelMessageSend(thread.ID, "Server error. Try again later")
		}
		_, err = s.ChannelMessageSend(thread.ID, response)
		if err != nil {
			log.Println("Error sending message in thread:", err)
		}
	}
}
