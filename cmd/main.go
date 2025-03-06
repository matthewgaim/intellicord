package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"github.com/matthewgaim/intellicord/internal/ai"
	"github.com/matthewgaim/intellicord/internal/api"
	"github.com/matthewgaim/intellicord/internal/guilds"
	"github.com/matthewgaim/intellicord/internal/handlers"
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

	// Bot added to new server
	dg.AddHandler(handlers.BotAddedToServerHandler())

	// Bot removed from server
	dg.AddHandler(handlers.BotRemovedFromServerHandler())

	// Respond to user in a bot-created thread
	dg.AddHandler(handlers.BotRespondToThreadHandler())

	// Listen for new attachments
	dg.AddHandler(handlers.StartThreadFromAttachmentUploadHandler())

	// Listen for attachments deleted
	dg.AddHandler(handlers.AttachmentDeletedHandler())

	// Start a thread from 'Message Reply' to a message with attachments
	dg.AddHandler(handlers.StartThreadFromReplyHandler())

	dg.Identify.Intents = discordgo.IntentsAll
	if err = dg.Open(); err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	fmt.Println("Waiting for bot initialization...")
	if dg.State.User == nil {
		log.Fatal("Bot user is not initialized")
	}

	go api.StartAPIServer()

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
