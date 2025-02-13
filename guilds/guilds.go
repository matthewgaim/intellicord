package guilds

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func RegisterCommandsForGuild(s *discordgo.Session, guildID string, commands []*discordgo.ApplicationCommand) {
	botID := s.State.User.ID
	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(botID, guildID, cmd)
		if err != nil {
			log.Printf("Cannot create '%s' command: %v", cmd.Name, err)
		} else {
			log.Printf("Registered command '%s'", cmd.Name)
		}
	}
}

func DeleteCommandsForGuild(s *discordgo.Session, guildID string) {
	botID := s.State.User.ID
	commands, err := s.ApplicationCommands(botID, guildID)
	if err != nil {
		log.Printf("Failed to fetch commands for guild %s: %v", guildID, err)
		return
	}

	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(botID, guildID, cmd.ID)
		if err != nil {
			log.Printf("Failed to delete command '%s' from guild %s: %v", cmd.Name, guildID, err)
		} else {
			log.Printf("Deleted command '%s' from guild %s", cmd.Name, guildID)
		}
	}
}
