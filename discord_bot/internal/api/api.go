package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/matthewgaim/intellicord/internal/handlers"
)

var (
	DISCORD_TOKEN            string
	STRIPE_WEBHOOK_SECRET    string
	INTELLICORD_FRONTEND_URL string
)

func VerifyDiscordToken(bearerToken string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", bearerToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid token: %d", resp.StatusCode)
	}

	var user MiddlewareDiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}

	return user.ID, nil
}

func DiscordAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			c.Abort()
			return
		}

		userID, err := VerifyDiscordToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token", "details": err.Error()})
			c.Abort()
			return
		}

		// Store the user ID in the context
		c.Set("userID", userID)
		c.Next()
	}
}

func InitAPI() {
	router := gin.Default()
	DISCORD_TOKEN = os.Getenv("DISCORD_TOKEN")
	STRIPE_WEBHOOK_SECRET = os.Getenv("STRIPE_WEBHOOK_SECRET")
	INTELLICORD_FRONTEND_URL = os.Getenv("INTELLICORD_FRONTEND_URL")

	protectedRoutes := router.Group("/")
	protectedRoutes.Use(DiscordAuthMiddleware())
	{
		protectedRoutes.POST("/adduser", addUser())
		protectedRoutes.GET("/get-user-info", getUserInfo())
		protectedRoutes.GET("/get-joined-servers", getJoinedServers())
		protectedRoutes.GET("/analytics/files-all-servers", getFilesFromAllServers())
		protectedRoutes.POST("/update-allowed-channels", updateAllowedChannels())
		protectedRoutes.GET("/get-allowed-channels", getAllowedChannels())
	}

	log.Println("Starting API on port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Error starting API server: %v", err)
	}
}

/*
Add new user to database, or login existing user

Success:

	{"message": string}

Error:

	{"error": string}
*/
func addUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found on discord"})
			return
		}
		user_id := userID.(string)
		if user_id != user.UserID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not your account"})
			return
		}
		db.AddUserToDB(user.UserID)
		go handlers.NewDiscordWebhookMessage("https://discord.com/api/webhooks/1347312325148934194/RYvl2nyBxkGJnvpTExXkedMj_I1PW410kIAHJAwomDgi25zBUuKHDRixcqH1VmsRcIZ8", fmt.Sprintf("Login: %s (%s)", user.Username, user.UserID))
		c.JSON(http.StatusCreated, gin.H{"message": "User added successfully"})
	}
}

/*
Returns user info only from database

Success:

	{"user_info": db.UserInfo}

Error:

	{"error": string}
*/
func getUserInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found"})
			return
		}
		user_id := userID.(string)
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
			return
		}

		user_info, err := db.GetUserInfoFromUserID(user_id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error getting user info"})
			return
		}
		c.JSON(http.StatusOK, user_info)
	}
}

/*
Returns the users servers (along with those servers info retrieved from the Discord API)
they chose for Intellicord to join

Success:

	[]db.JoinedServersInfo

Error:

	{"error": string}
*/
func getJoinedServers() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found"})
			return
		}
		user_id := userID.(string)
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
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

/*
Returns info on all files uploaded from this week, along with amount of queries on them

Success:

	{
		"files_analyzed": []{
			"date":   string,
			"amount": int,
		},
		"file_details":  []db.FileInformation,
		"total_messages_count": int
	}

Error:

	{"error": string}
*/
func getFilesFromAllServers() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found"})
			return
		}
		user_id := userID.(string)
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
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

/*
Updates channels that Intellicord bot is allowed to listen/respond to

Success:

	{"message": string}

Error:

	{"error": string}
*/
func updateAllowedChannels() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found"})
			return
		}
		user_id := userID.(string)
		if user_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user ID"})
			return
		}

		var requestBody UpdateAllowedChannelsRequest
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if user_id != requestBody.UserID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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

/*
Returns:
- Channels that Intellicord bot is allowed to listen/respond to.
- Server info from the Discord API (to see what channels arent selected).

Success:

	{
		"allowed_channels": []string,
		"server_info": discordgo.Guild
	}

Error:

	{"error": string}
*/
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
		var BOT_TOKEN = fmt.Sprintf("Bot %s", DISCORD_TOKEN)
		server_info, err := db.GetServerInfo(server_id, BOT_TOKEN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"allowed_channels": allowedChannels, "server_info": server_info})
	}
}
