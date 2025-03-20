package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/internal/ai"
	"github.com/redis/go-redis/v9"
)

func AddGuildToDB(guildID string, guildOwnerID string) {
	_, err := ai.DbPool.Exec(context.Background(), `
		INSERT INTO joined_servers (discord_server_id, owner_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, guildID, guildOwnerID)
	if err != nil {
		log.Printf("Error adding guild to DB: %v", err)
	}
}

func RemoveGuildFromDB(guildID string) {
	_, err := ai.DbPool.Exec(context.Background(), `
	DELETE FROM joined_servers WHERE discord_server_id = $1`, guildID)
	if err != nil {
		log.Printf("Error removing guild from DB: %v", err)
	}

	_, err = ai.DbPool.Exec(context.Background(), `
	DELETE FROM chunks WHERE discord_server_id = $1`, guildID)
	if err != nil {
		log.Printf("Error removing chunks from DB: %v", err)
	}
}

func AddUserToDB(userID string) {
	now := time.Now()
	renewalDate := now.AddDate(0, 1, 0) // 1 month from now

	_, err := ai.DbPool.Exec(context.Background(), `
        INSERT INTO users (
            discord_id, 
            plan, 
            plan_monthly_start_date, 
            plan_renewal_date
        ) VALUES (
            $1, 'free', $2, $3
        ) ON CONFLICT DO NOTHING
    `, userID, now, renewalDate)

	if err != nil {
		log.Printf("Error adding user to DB: %v", err)
	}
}

func GetRegisteredServers(userID string) ([]JoinedServersInfo, error) {
	rows, err := ai.DbPool.Query(context.Background(), `
    SELECT id, discord_server_id, joined_at FROM joined_servers WHERE owner_id = $1
    `, userID)
	if err != nil {
		log.Printf("Error getting joined servers: %v", err)
		return nil, err
	}
	defer rows.Close()
	var BOT_TOKEN = fmt.Sprintf("Bot %s", os.Getenv("DISCORD_TOKEN"))

	var serversInfo []JoinedServersInfo
	for rows.Next() {
		var id int
		var discord_server_id string
		var joined_at time.Time

		err := rows.Scan(&id, &discord_server_id, &joined_at)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		// Fetch guild info only if necessary
		server_info, err := GetServerInfo(discord_server_id, BOT_TOKEN)
		if err != nil {
			log.Printf("Error getting server info: %v", err)
			return []JoinedServersInfo{}, err
		}

		serversInfo = append(serversInfo, JoinedServersInfo{
			ID:              id,
			DiscordServerID: discord_server_id,
			JoinedAt:        joined_at.String(),
			Name:            server_info.Name,
			MemberCount:     server_info.ApproximateMemberCount,
			OnlineCount:     server_info.ApproximatePresenceCount,
			Icon:            server_info.IconURL("512x512"),
			PremiumTier:     int(server_info.PremiumTier),
			Banner:          server_info.BannerURL("512x512"),
		})
	}

	return serversInfo, nil
}

func FileAnalysisAllServers(user_id string) ([]map[string]interface{}, []FileInformation, int, error) {
	rows, err := ai.DbPool.Query(context.Background(), `
        SELECT DATE(uf.uploaded_at) AS upload_date, COUNT(uf.id) AS total_files, uf.title, uf.file_size
        FROM uploaded_files uf
        JOIN joined_servers js ON uf.discord_server_id = js.discord_server_id
        WHERE js.owner_id = $1 AND uf.uploaded_at >= NOW() - INTERVAL '7 days'
        GROUP BY upload_date, uf.title, uf.file_size
        ORDER BY upload_date DESC
    `, user_id)
	if err != nil {
		log.Printf("Error fetching total files per day: %v", err)
		return nil, nil, 0, err
	}
	defer rows.Close()

	var filesPerDay []map[string]interface{}
	var fileDetails []FileInformation
	dateFileUploadedData := make(map[string]int) // To track days with file uploads

	for rows.Next() {
		var uploadDate time.Time
		var totalFiles int
		var title string
		var fileSize int64
		if err := rows.Scan(&uploadDate, &totalFiles, &title, &fileSize); err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, nil, 0, err
		}

		dateStr := uploadDate.Format("01/02")
		dateFileUploadedData[dateStr] += totalFiles

		titleSplit := strings.Split(title, ".")
		fileType := strings.ToUpper(titleSplit[len(titleSplit)-1])

		fileDetails = append(fileDetails, FileInformation{
			Name:         title,
			Type:         fileType,
			Size:         fileSize,
			AnalyzedDate: dateStr,
		})
	}

	// Generate the last 7 days and fill missing days with 0
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i)
		formatted := date.Format("01/02")

		amount, exists := dateFileUploadedData[formatted]
		if !exists {
			amount = 0
		}

		filesPerDay = append(filesPerDay, map[string]interface{}{
			"date":   formatted,
			"amount": amount,
		})
	}
	slices.Reverse(filesPerDay)

	// total messages across all owned servers
	var totalMessages int
	err = ai.DbPool.QueryRow(context.Background(), `
        SELECT COUNT(ml.id) 
        FROM message_logs ml
        JOIN joined_servers js ON ml.discord_server_id = js.discord_server_id
        WHERE js.owner_id = $1
    `, user_id).Scan(&totalMessages)

	if err != nil {
		log.Printf("Error fetching total messages: %v", err)
		return filesPerDay, fileDetails, 0, err
	}

	return filesPerDay, fileDetails, totalMessages, nil
}

func GetServerInfo(discord_server_id string, BOT_TOKEN string) (discordgo.Guild, error) {
	guildUrl := fmt.Sprintf("https://discord.com/api/v10/guilds/%s?with_counts=true", discord_server_id)
	req, err := http.NewRequest("GET", guildUrl, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return discordgo.Guild{}, err
	}
	req.Header.Set("Authorization", BOT_TOKEN)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting guild info: %v", err)
		return discordgo.Guild{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Discord API error: %d", resp.StatusCode)
		return discordgo.Guild{}, err
	}

	var target discordgo.Guild
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		log.Printf("Error decoding response: %v", err)
		return discordgo.Guild{}, err
	}

	return target, nil
}

func AddMessageLog(message_id string, discord_server_id string, channel_id string, user_id string) {
	_, err := ai.DbPool.Exec(context.Background(), `
	INSERT INTO message_logs 
		(message_id, discord_server_id, channel_id, user_id)
	VALUES
		($1, $2, $3, $4)
	`, message_id, discord_server_id, channel_id, user_id)
	if err != nil {
		log.Printf("Error logging message: %v", err)
	}
}

func UpdateAllowedChannels(allowedChannels []string, serverID string) error {
	query := `
		UPDATE joined_servers
		SET allowed_channels = $1
		WHERE discord_server_id = $2
		RETURNING allowed_channels
	`

	_, err := ai.DbPool.Exec(context.Background(), query, allowedChannels, serverID)
	if err != nil {
		return err
	}
	return nil
}

func GetAllowedChannels(serverID string) ([]string, error) {
	var allowedChannels []string
	query := `SELECT allowed_channels FROM joined_servers WHERE discord_server_id = $1`
	err := ai.DbPool.QueryRow(context.Background(), query, serverID).Scan(&allowedChannels)
	if err != nil {
		return nil, err
	}

	return allowedChannels, nil
}

func UpdateUsersPaidPlanStatus(userID string, priceID string, planName string, subStartDate time.Time, subRenewalDate time.Time, customerID string) error {
	_, err := ai.DbPool.Exec(context.Background(), `
		UPDATE users
		SET price_id = $1, plan = $2, plan_monthly_start_date = $3, plan_renewal_date = $4, stripe_customer_id = $5
		WHERE discord_id = $6`,
		priceID, planName, subStartDate, subRenewalDate, customerID, userID)
	if err != nil {
		return err
	} else {
		userInfo := UserInfo{
			PriceID:              priceID,
			Plan:                 planName,
			PlanMonthlyStartDate: subStartDate,
			PlanRenewalDate:      subRenewalDate,
		}
		UpdateToRedis(userID, userInfo)
		return nil
	}
}

func GetUserInfoFromUserID(discordID string) (UserInfo, error) {
	row := ai.DbPool.QueryRow(context.Background(), `
        SELECT price_id, plan, plan_monthly_start_date, plan_renewal_date, joined_at 
        FROM users 
        WHERE discord_id = $1
    `, discordID)

	var priceID string
	var plan string
	var planMonthlyStartDate time.Time
	var planRenewalDate time.Time
	var joinedAt time.Time

	if err := row.Scan(&priceID, &plan, &planMonthlyStartDate, &planRenewalDate, &joinedAt); err != nil {
		return UserInfo{}, err
	}

	return UserInfo{
		PriceID:              priceID,
		Plan:                 plan,
		PlanMonthlyStartDate: planMonthlyStartDate,
		PlanRenewalDate:      planRenewalDate,
		JoinedAt:             joinedAt,
	}, nil
}

// Map of plan names to their limits
var planLimitsMap = map[string]PlanLimits{
	"free": {
		MaxFileUploads: 10,
		MaxMessages:    100,
	},
	"Intellicord Basic": {
		MaxFileUploads: 50,
		MaxMessages:    500,
	},
	"Intellicord Premium": {
		MaxFileUploads: 500,
		MaxMessages:    5000,
	},
}

func CheckOwnerLimits(ownerID string) (bool, bool, error) {
	cached, redis_err := ai.RedisClient.Get(context.Background(), ownerID).Result()
	cachedByteArr := []byte(cached)
	var userInfo UserInfo

	err := json.Unmarshal(cachedByteArr, &userInfo)
	if redis_err == redis.Nil || err != nil { // just not found in cache, not a real error
		log.Println("Owner limits not found in Redis, searching DB")
		userInfo, err = GetUserInfoFromUserID(ownerID)
		if err != nil {
			log.Printf("Error getting user info: %v", err)
			return false, false, err
		}
		err = UpdateToRedis(ownerID, userInfo)
		if err != nil {
			log.Printf("Failed to set data in Redis: %v", err)
		}
	} else {
		log.Println("Cache hit")
	}
	if userInfo.Plan == "free" {
		// If current time is past the renewal date, update the monthly start date
		if time.Now().After(userInfo.PlanRenewalDate) {
			newStartDate := time.Now()
			newRenewalDate := newStartDate.AddDate(0, 1, 0)

			_, err := ai.DbPool.Exec(context.Background(), `
                UPDATE users 
                SET plan_monthly_start_date = $1, plan_renewal_date = $2 
                WHERE discord_id = $3
            `, newStartDate, newRenewalDate, ownerID)

			if err != nil {
				log.Printf("Error updating free user's billing period: %v", err)
				return false, false, err
			}

			userInfo.PlanMonthlyStartDate = newStartDate
			userInfo.PlanRenewalDate = newRenewalDate
		}
	}

	planLimits, ok := planLimitsMap[userInfo.Plan]
	if !ok {
		planLimits = planLimitsMap["free"]
	}

	monthlyStartDate := userInfo.PlanMonthlyStartDate

	// Count total file uploads within the current billing period
	var totalFileUploads int
	err = ai.DbPool.QueryRow(context.Background(), `
        SELECT COUNT(uf.id) 
        FROM uploaded_files uf
        JOIN joined_servers js ON uf.discord_server_id = js.discord_server_id
        WHERE js.owner_id = $1 AND uf.uploaded_at >= $2
    `, ownerID, monthlyStartDate).Scan(&totalFileUploads)

	if err != nil {
		log.Printf("Error counting file uploads: %v", err)
		return false, false, err
	}

	// Count total messages within the current billing period
	var totalMessages int
	err = ai.DbPool.QueryRow(context.Background(), `
        SELECT COUNT(ml.id) 
        FROM message_logs ml
        JOIN joined_servers js ON ml.discord_server_id = js.discord_server_id
        WHERE js.owner_id = $1 AND ml.created_at >= $2
    `, ownerID, monthlyStartDate).Scan(&totalMessages)

	if err != nil {
		log.Printf("Error counting messages: %v", err)
		return false, false, err
	}
	log.Printf("File Uploads: %d/%d", totalFileUploads, planLimits.MaxFileUploads)
	log.Printf("Messages: %d/%d", totalMessages, planLimits.MaxMessages)

	// Check if limits are reached
	fileUploadLimitReached := totalFileUploads >= planLimits.MaxFileUploads
	messageLimitReached := totalMessages >= planLimits.MaxMessages

	return fileUploadLimitReached, messageLimitReached, nil
}

func UpdateToRedis(key string, val any) error {
	marshalledVal, err := json.Marshal(val)
	if err != nil {
		return err
	}
	stringUserInfo := string(marshalledVal)
	_, err = ai.RedisClient.Set(context.Background(), key, stringUserInfo, 24*time.Hour).Result()
	if err != nil {
		return err
	}
	log.Printf("Updating key on Redis: %s", key)
	return nil
}
