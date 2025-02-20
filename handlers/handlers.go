package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/ai"
	"github.com/matthewgaim/intellicord/guilds"
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
			log.Printf("Commands for Server: %s\n", g.ID)
			guilds.RegisterCommandsForGuild(dg, g.ID, commands)
		}
		dg.UpdateCustomStatus("Upload a file to any channel, or type /ask to use Intellicord")
	}
}

func BotAddedToServerHandler() func(s *discordgo.Session, g *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		log.Printf("Joined a new server: %s (ID: %s)", g.Name, g.ID)
		guilds.RegisterCommandsForGuild(s, g.ID, commands)
	}
}

func BotRemovedFromServerHandler() func(s *discordgo.Session, g *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		log.Printf("Removed from server: %s (ID: %s)", g.Guild.Name, g.Guild.ID)
		guilds.DeleteCommandsForGuild(s, g.Guild.ID)
	}
}

func BotRespondToThreadHandler() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Error fetching channel:", err)
			return
		}

		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			// Don't recognize extra files in a thread
			if len(m.Attachments) > 0 {
				s.ChannelMessageDelete(channel.ID, m.Message.ID)
				s.ChannelMessageSend(channel.ID, "Attached document will not be recognized in context")
				return
			}

			history, err := GetThreadMessages(s, channel.ID, s.State.User.ID)
			if err != nil {
				log.Printf("Error getting thread messages: %v\n", err.Error())
				return
			}
			if channel.OwnerID == s.State.User.ID {
				log.Println("Message received in bot-created thread:", m.Content)

				rootMsg, err := getRootMessageOfThread(s, channel)
				if err != nil {
					log.Printf("Error getting root message: %v", err)
				}
				numOfAttachments := len(rootMsg.Attachments)
				rootMsgID := rootMsg.ID
				res := ai.QueryVectorDB(context.Background(), m.Content, rootMsgID, numOfAttachments)
				history = append(history, openai.SystemMessage(fmt.Sprintf("Additional Context:\n%s", res)))
				response := ai.LlmGenerateText(history, m.Content)
				_, err = s.ChannelMessageSend(m.ChannelID, response)
				if err != nil {
					log.Println("Error sending message in thread:", err)
				}
			}
		}
	}
}

func StartThreadFromAttachmentUploadHandler() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if len(m.Attachments) == 0 {
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
			Name: m.Attachments[0].Filename,
		}
		thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, data)
		if err != nil {
			log.Printf("Error creating thread: %v", err)
			return
		}

		for i, attachment := range m.Attachments {
			attachmentLink := attachment.URL
			filename := attachment.Filename
			log.Printf("Attachment %d: %s (%s)", i, filename, attachmentLink)

			fileText, err := getFileText(attachmentLink)
			if err != nil {
				log.Printf("Error getting file text: %v", err)
				continue
			}

			ai.ChunkAndVectorize(context.Background(), m.Message.ID, fileText, filename, attachmentLink)
			_, err = s.ChannelMessageSend(thread.ID, fmt.Sprintf("Processed content of %s", filename))
			if err != nil {
				log.Printf("Error sending message in thread: %v", err)
			}
		}
	}
}

func AttachmentDeletedHandler() func(s *discordgo.Session, m *discordgo.MessageDelete) {
	return func(s *discordgo.Session, m *discordgo.MessageDelete) {
		message_id := m.Message.ID
		ai.DeleteEmbeddings(context.Background(), message_id)
	}
}
