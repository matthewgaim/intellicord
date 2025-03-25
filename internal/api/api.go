package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/matthewgaim/intellicord/internal/handlers"
	"github.com/stripe/stripe-go/v81"
	billingPortalSession "github.com/stripe/stripe-go/v81/billingportal/session"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
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

func StartAPIServer() {
	router := gin.Default()
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	protectedRoutes := router.Group("/")
	protectedRoutes.Use(DiscordAuthMiddleware())
	{
		protectedRoutes.POST("/adduser", addUser())
		protectedRoutes.GET("/get-user-info", getUserInfo())
		protectedRoutes.GET("/get-joined-servers", getJoinedServers())
		protectedRoutes.GET("/analytics/files-all-servers", getFilesFromAllServers())
		protectedRoutes.POST("/update-allowed-channels", updateAllowedChannels())
		protectedRoutes.GET("/get-allowed-channels", getAllowedChannels())
		protectedRoutes.POST("/create-checkout-session", createCheckoutSession())
		protectedRoutes.GET("/session-status", retrieveCheckoutSession())
		protectedRoutes.GET("/get-stripe-portal", createPortalSession())
	}
	router.POST("/webhook", handleStripeWebhook())

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
		go handlers.NewDiscordWebhookMessage("https://discord.com/api/webhooks/1347312325148934194/RYvl2nyBxkGJnvpTExXkedMj_I1PW410kIAHJAwomDgi25zBUuKHDRixcqH1VmsRcIZ8", fmt.Sprintf("Login: %s (%s)", user.Username, user.UserID))
		c.JSON(http.StatusCreated, gin.H{"message": "User added successfully"})
	}
}

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

func createCheckoutSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		price_id := c.Query("price_id")
		if price_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "price_id is missing"})
			return
		}
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
		domain := os.Getenv("INTELLICORD_FRONTEND_URL")
		params := &stripe.CheckoutSessionParams{
			UIMode:    stripe.String("embedded"),
			ReturnURL: stripe.String(domain + "/dashboard/pricing/return?session_id={CHECKOUT_SESSION_ID}"),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Price:    stripe.String(price_id),
					Quantity: stripe.Int64(1),
				},
			},
			SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
				Metadata: map[string]string{
					"discord_id": user_id,
				},
			},
			Mode:         stripe.String(string(stripe.CheckoutSessionModeSubscription)),
			AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
		}
		s, err := session.New(params)

		if err != nil {
			log.Printf("session.New: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, ClientSecret{
			ClientSecret: s.ClientSecret,
		})
	}
}

func retrieveCheckoutSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		session_id := c.Query("session_id")
		if session_id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing session_id"})
			return
		}
		s, err := session.Get(session_id, nil)
		if err != nil {
			log.Printf("session.Get error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, CheckoutSessionType{
			Status: string(s.Status),
			Name:   string(s.CustomerDetails.Name),
			Email:  string(s.CustomerDetails.Email),
		})
	}
}

func createPortalSession() gin.HandlerFunc {
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
		row := db.DbPool.QueryRow(context.Background(),
			`SELECT stripe_customer_id FROM users WHERE discord_id = $1`, user_id)
		var stripe_customer_id string
		if err := row.Scan(&stripe_customer_id); err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting customer ID"})
			return
		}
		if stripe_customer_id == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "This user is not subscribed yet"})
			return
		}

		domain := os.Getenv("INTELLICORD_FRONTEND_URL")
		returnURL := fmt.Sprintf("%s/dashboard/profile", domain)
		params := &stripe.BillingPortalSessionParams{
			Customer:  stripe.String(stripe_customer_id),
			ReturnURL: stripe.String(returnURL),
		}

		// Create the portal session
		s, err := billingPortalSession.New(params)
		if err != nil {
			log.Printf("session.New: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": s.URL})
	}
}

func handleStripeWebhook() gin.HandlerFunc {
	return func(c *gin.Context) {
		const MaxBodyBytes = int64(65536)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			c.Status(http.StatusServiceUnavailable)
			return
		}

		stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
		event, err := webhook.ConstructEvent(payload, c.GetHeader("Stripe-Signature"), stripeWebhookSecret)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying webhook signature: %v\n", err)
			c.Status(http.StatusBadRequest) // Return a 400 error on a bad signature
			return
		}
		log.Printf("Event type: %s", event.Type)

		// REMEMBER TO UPDATE ON WEBSITE IF YOU ADD MORE
		switch event.Type {
		case "invoice.payment_succeeded":
			err, errType := invoicePaymentSucceeded(event)
			if err != nil {
				c.JSON(errType, gin.H{"error": err.Error()})
			}
			return
		case "customer.subscription.deleted":
			err, errType := customerSubscriptionDeleted(event)
			if err != nil {
				c.JSON(errType, gin.H{"error": err.Error()})
			}
			return
		case "customer.subscription.updated":
			err, errType := customerSubscriptionUpdated(event)
			if err != nil {
				c.JSON(errType, gin.H{"error": err.Error()})
			}
			return
		default:
			log.Printf("Unhandled event type: %s\n", event.Type)
		}

		c.Status(http.StatusOK)
	}
}
