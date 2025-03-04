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

	"github.com/matthewgaim/intellicord/internal/ai"
	"github.com/matthewgaim/intellicord/internal/db"
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

type UpdateAllowedChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids"`
	ServerID   string   `json:"server_id"`
	UserID     string   `json:"user_id"`
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

		totalFilesAnalyzed, fileNames, totalMessagesCount, err := db.FileAnalysisAllServers(user_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"files_analyzed": totalFilesAnalyzed, "file_details": fileNames, "total_messages_count": totalMessagesCount})
	})

	router.POST("/update-allowed-channels", func(c *gin.Context) {
		var requestBody UpdateAllowedChannelsRequest
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.UpdateAllowedChannels(requestBody.ChannelIDs, requestBody.ServerID); err != nil {
			log.Printf("Error updating allowed channels: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	router.GET("/get-allowed-channels", func(c *gin.Context) {
		server_id := c.Query("server_id")
		if server_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No server_id"})
			return
		}

		allowedChannels, err := db.GetAllowedChannels(server_id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var BOT_TOKEN = fmt.Sprintf("Bot %s", os.Getenv("DISCORD_TOKEN"))
		server_info, err := db.GetServerInfo(server_id, BOT_TOKEN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"allowed_channels": allowedChannels, "server_info": server_info})
	})

	log.Println("Starting API on port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Error starting API server: %v", err)
	}
}
