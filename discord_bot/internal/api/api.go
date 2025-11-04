package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/matthewgaim/intellicord/internal/handlers"
)

var (
	DISCORD_TOKEN            string
	INTELLICORD_FRONTEND_URL string
	DISCORD_CLIENT_ID        string
	DISCORD_CLIENT_SECRET    string
	DISCORD_REDIRECT_URI     string
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
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5500", "https://intellicord.senarado.com"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           24 * time.Hour,
	}))

	DISCORD_TOKEN = os.Getenv("DISCORD_TOKEN")
	INTELLICORD_FRONTEND_URL = os.Getenv("INTELLICORD_FRONTEND_URL")
	DISCORD_CLIENT_ID = os.Getenv("DISCORD_CLIENT_ID")
	DISCORD_CLIENT_SECRET = os.Getenv("DISCORD_CLIENT_SECRET")
	DISCORD_REDIRECT_URI = os.Getenv("DISCORD_REDIRECT_URI")

	router.POST("/adduser", addUser())

	protectedRoutes := router.Group("/")
	protectedRoutes.Use(DiscordAuthMiddleware())
	{
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

func getDiscordAvatarURL(userID, avatarHash string) string {
	if avatarHash == "" {
		return "https://cdn.discordapp.com/embed/avatars/0.png"
	}

	extension := "png"
	if strings.HasPrefix(avatarHash, "a_") {
		extension = "gif"
	}

	avatarURL := fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s", userID, avatarHash, extension)
	return avatarURL
}

func setCrossSiteCookie(c *gin.Context, name, value string, httpOnly bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   7 * 24 * 60 * 60, // one week
		Path:     "/",
		Secure:   true,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteNoneMode,
	})
}

func addUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Code string `json:"code"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
			return
		}

		form := url.Values{}
		form.Add("client_id", DISCORD_CLIENT_ID)
		form.Add("client_secret", DISCORD_CLIENT_SECRET)
		form.Add("grant_type", "authorization_code")
		form.Add("code", body.Code)
		form.Add("redirect_uri", DISCORD_REDIRECT_URI)

		tokenResp, err := http.Post(
			"https://discord.com/api/oauth2/token",
			"application/x-www-form-urlencoded",
			bytes.NewBufferString(form.Encode()),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request token"})
			return
		}
		defer tokenResp.Body.Close()

		tokenBody, _ := io.ReadAll(tokenResp.Body)
		if tokenResp.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadRequest, gin.H{"error": string(tokenBody)})
			return
		}

		var token DiscordTokenResponse
		if err := json.Unmarshal(tokenBody, &token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token"})
			return
		}

		req, _ := http.NewRequest("GET", "https://discord.com/api/oauth2/@me", nil)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		client := &http.Client{}
		userResp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}
		defer userResp.Body.Close()

		userBody, _ := io.ReadAll(userResp.Body)
		var me DiscordMeResponse
		if err := json.Unmarshal(userBody, &me); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user info"})
			return
		}

		user := me.User
		fullTag := fmt.Sprintf("%s#%s", user.Username, user.Discriminator)
		avatar := getDiscordAvatarURL(user.ID, user.Avatar)

		db.AddUserToDB(user.ID)
		go handlers.NewDiscordWebhookMessage(
			"https://discord.com/api/webhooks/1347312325148934194/RYvl2nyBxkGJnvpTExXkedMj_I1PW410kIAHJAwomDgi25zBUuKHDRixcqH1VmsRcIZ8",
			fmt.Sprintf("Login: %s (%s)", user.GlobalName, user.ID),
		)

		// oneWeek := 7 * 24 * 60 * 60

		// c.SetCookie("discord_user_id", user.ID, oneWeek, "/", "", true, false)
		// c.SetCookie("discord_username", user.GlobalName, oneWeek, "/", "", true, false)
		// c.SetCookie("discord_avatar", avatar, oneWeek, "/", "", true, false)
		// c.SetCookie("access_token", token.AccessToken, oneWeek, "/", "", true, false)

		setCrossSiteCookie(c, "discord_user_id", user.ID, true)
		setCrossSiteCookie(c, "discord_username", user.GlobalName, true)
		setCrossSiteCookie(c, "discord_avatar", avatar, true)
		setCrossSiteCookie(c, "access_token", token.AccessToken, false)

		c.JSON(http.StatusOK, gin.H{
			"message": "Login successful",
			"user": gin.H{
				"id":       user.ID,
				"username": user.GlobalName,
				"avatar":   avatar,
				"tag":      fullTag,
			},
		})

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
