package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"github.com/matthewgaim/intellicord/ai"
	"github.com/matthewgaim/intellicord/guilds"
	"github.com/matthewgaim/intellicord/handlers"
)

var dg *discordgo.Session

func main() {
	var err error = nil
	err = godotenv.Load(".env")
	if err != nil {
		log.Println("Error loading .env file")
	}
	DISCORD_TOKEN := os.Getenv("DISCORD_TOKEN")

	dg, err = discordgo.New("Bot " + DISCORD_TOKEN)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	ai.InitAI()

	// Command handlers
	handlers.InitCommands()

	// Command lookup
	dg.AddHandler(handlers.CommandLookupHandler())

	// Register commands when bot is ready
	dg.AddHandler(handlers.BotReadyRegisterCommandsHandler(dg))

	// Bot added to new server (doesnt work)
	dg.AddHandler(handlers.BotAddedToServerHandler())

	// Bot removed from server (doesnt work)
	dg.AddHandler(handlers.BotRemovedFromServerHandler())

	// Respond to user in a bot-created thread
	dg.AddHandler(handlers.BotRespondToThreadHandler())

	// Listen for attachments
	dg.AddHandler(handlers.GetAttachmentFromMessageHandler())

	dg.Identify.Intents = discordgo.IntentsGuildMessages
	if err = dg.Open(); err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	fmt.Println("Waiting for bot initialization...")
	if dg.State.User == nil {
		log.Fatal("Bot user is not initialized")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	defer dg.Close()
	for _, g := range dg.State.Guilds {
		guilds.DeleteCommandsForGuild(dg, g.ID)
	}
	log.Println("Gracefully shutting down.")
}
