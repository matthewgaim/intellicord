package api

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/matthewgaim/intellicord/internal/db"
)

type User struct {
	UserID string `json:"user_id"`
}

type UpdateAllowedChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids"`
	ServerID   string   `json:"server_id"`
	UserID     string   `json:"user_id"`
}

func VerifyDiscordToken(bearerToken string) (bool, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", bearerToken)
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, fmt.Errorf("invalid token: %d", resp.StatusCode)
}

func DiscordAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			c.Abort() // Stop request processing
			return
		}

		valid, err := VerifyDiscordToken(token)
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token", "details": err.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}

func StartAPIServer() {
	router := gin.Default()

	router.Use(DiscordAuthMiddleware())
	router.POST("/adduser", addUser())
	router.GET("/get-joined-servers", getJoinedServers())
	router.GET("/analytics/files-all-servers", getFilesFromAllServers())
	router.POST("/update-allowed-channels", updateAllowedChannels())
	router.GET("/get-allowed-channels", getAllowedChannels())

	log.Println("Starting API on port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Error starting API server: %v", err)
	}
}

func addUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		db.AddUserToDB(user.UserID)
		c.JSON(http.StatusCreated, gin.H{"message": "User added successfully"})
	}
}

func getJoinedServers() gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}

func getFilesFromAllServers() gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}

func updateAllowedChannels() gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}

func getAllowedChannels() gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}
