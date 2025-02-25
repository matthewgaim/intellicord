package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/matthewgaim/intellicord/ai"
	"github.com/matthewgaim/intellicord/db"
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

	// Bot added to new server
	dg.AddHandler(handlers.BotAddedToServerHandler())

	// Bot removed from server
	dg.AddHandler(handlers.BotRemovedFromServerHandler())

	// Respond to user in a bot-created thread
	dg.AddHandler(handlers.BotRespondToThreadHandler())

	// Listen for attachments
	dg.AddHandler(handlers.StartThreadFromAttachmentUploadHandler())

	dg.AddHandler(handlers.AttachmentDeletedHandler())

	dg.Identify.Intents = discordgo.IntentsAll
	if err = dg.Open(); err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	fmt.Println("Waiting for bot initialization...")
	if dg.State.User == nil {
		log.Fatal("Bot user is not initialized")
	}

	go startAPIServer()

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

type User struct {
	UserID string `json:"user_id"`
}

func startAPIServer() {
	router := gin.Default()

	router.POST("/adduser", func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		db.AddUserToDB(user.UserID)
		c.JSON(http.StatusCreated, gin.H{"message": "User added successfully"})
	})

	router.GET("/get-joined-servers", func(c *gin.Context) {
		user_id := c.Query("user_id")
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No user_id"})
			return
		}

		joinedServers, err := db.GetRegisteredServers(user_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, joinedServers)
	})

	router.GET("/analytics/files-all-servers", func(c *gin.Context) {
		user_id := c.Query("user_id")
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No user_id"})
			return
		}

		totalFilesAnalyzed, fileNames, err := db.FileAnalysisAllServers(user_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"files_analyzed": totalFilesAnalyzed, "file_details": fileNames})
	})

	log.Println("Starting API on port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Error starting API server: %v", err)
	}
}
