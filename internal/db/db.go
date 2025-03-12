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
	_, err := ai.DbPool.Exec(context.Background(), `
		INSERT INTO users (discord_id, plan) VALUES ($1, 'free') ON CONFLICT DO NOTHING
	`, userID)
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

func UpdateUsersPaidPlanStatus(userID string, priceID string, planName string) error {
	log.Println(userID, priceID, planName)
	_, err := ai.DbPool.Exec(context.Background(), `
		UPDATE users
		SET price_id = $1, plan = $2
		WHERE discord_id = $3`,
		priceID, planName, userID)
	if err != nil {
		return err
	} else {
		return nil
	}
}

type UserInfo struct {
	PriceID  string    `json:"price_id"`
	Plan     string    `json:"plan"`
	JoinedAt time.Time `json:"joined_at"`
}

func GetUserInfo(discordID string) (UserInfo, error) {
	row := ai.DbPool.QueryRow(context.Background(), `
		SELECT price_id, plan, joined_at FROM users WHERE discord_id = $1
	`, discordID)
	var price_id string
	var plan string
	var joined_at time.Time

	if err := row.Scan(&price_id, &plan, &joined_at); err != nil {
		return UserInfo{}, err
	}
	return UserInfo{PriceID: price_id, Plan: plan, JoinedAt: joined_at}, nil
}
