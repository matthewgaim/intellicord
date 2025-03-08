package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
		INSERT INTO users (discord_id) VALUES ($1) ON CONFLICT DO NOTHING
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

func AnalysisAllServers(user_id string) ([]map[string]interface{}, []FileInformation, int, error) {
	var err error
	filesPerDay, err := getCountOfUploadsPerDayInWeek(user_id)
	if err != nil {
		log.Printf("Error getCountOfUploadsPerDayInWeek: %v", err)
	}
	fileDetails, err := getFileDetails(user_id)
	if err != nil {
		log.Printf("Error getFileDetails: %v", err)
	}
	totalMessages, err := getTotalMessages(user_id)
	if err != nil {
		log.Printf("Error getTotalMessages: %v", err)
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
