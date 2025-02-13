package handlers

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/ai"
	"github.com/matthewgaim/intellicord/guilds"
)

func BotReadyRegisterCommandsHandler(dg *discordgo.Session) func(s *discordgo.Session, r *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		for _, g := range r.Guilds {
			log.Printf("Commands for Server: %s\n", g.ID)
			guilds.RegisterCommandsForGuild(dg, g.ID, commands)
		}
		dg.UpdateCustomStatus("Type /ask to use Intellicord")
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

		history, err := GetThreadMessages(s, channel.ID, s.State.User.ID)
		if err != nil {
			log.Printf("Error getting thread messages: %v\n", err.Error())
			return
		}

		if channel.Type == discordgo.ChannelTypeGuildPublicThread || channel.Type == discordgo.ChannelTypeGuildPrivateThread {
			if channel.OwnerID == s.State.User.ID {
				log.Println("Message received in bot-created thread:", m.Content)

				response := ai.LlmGenerateText(history, m.Content)
				_, err := s.ChannelMessageSend(m.ChannelID, response)
				if err != nil {
					log.Println("Error sending message in thread:", err)
				}
			}
		}
	}
}

func CommandLookupHandler() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	}
}
