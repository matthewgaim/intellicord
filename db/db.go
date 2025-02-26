package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/matthewgaim/intellicord/ai"
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

type JoinedServersInfo struct {
	ID              int    `json:"id"`
	DiscordServerID string `json:"discord_server_id"`
	JoinedAt        string `json:"joined_at"`
	Name            string `json:"name"`
	MemberCount     int    `json:"member_count"`
	Icon            string `json:"icon"`
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

		log.Println("Discord Server ID", discord_server_id)

		// Fetch guild info only if necessary
		server_info, err := getServerInfo(discord_server_id, BOT_TOKEN)
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
			Icon:            server_info.IconURL("512x512"),
		})
	}

	return serversInfo, nil
}

type FileInformation struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	AnalyzedDate string `json:"analyzed_date"`
}

func FileAnalysisAllServers(user_id string) ([]map[string]interface{}, []FileInformation, error) {
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
		return nil, nil, err
	}
	defer rows.Close()

	var filesPerDay []map[string]interface{}
	var fileDetails []FileInformation

	for rows.Next() {
		var uploadDate time.Time
		var totalFiles int
		var title string
		var fileSize int64
		if err := rows.Scan(&uploadDate, &totalFiles, &title, &fileSize); err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, nil, err
		}

		dateStr := uploadDate.Format("01/02")
		found := false

		// Check if the date already exists in the array
		for i, entry := range filesPerDay {
			if entry["date"] == dateStr {
				filesPerDay[i]["amount"] = entry["amount"].(int) + totalFiles
				found = true
				break
			}
		}

		// If not found, append a new entry
		if !found {
			filesPerDay = append(filesPerDay, map[string]interface{}{
				"date":   dateStr,
				"amount": totalFiles,
			})
		}

		titleSplit := strings.Split(title, ".")
		fileType := strings.ToUpper(titleSplit[len(titleSplit)-1])

		fileDetails = append(fileDetails, FileInformation{
			Name:         title,
			Type:         fileType,
			Size:         fileSize,
			AnalyzedDate: dateStr,
		})
	}
	if len(filesPerDay) < 7 {
		for i := len(filesPerDay); i < 8; i++ {
			filesPerDay = append(filesPerDay, map[string]interface{}{"amount": 0, "date": "N/A"})
		}
	}
	return filesPerDay, fileDetails, nil
}

func getServerInfo(discord_server_id string, BOT_TOKEN string) (discordgo.Guild, error) {
	guildUrl := fmt.Sprintf("https://discord.com/api/v10/guilds/%s?with_counts=true", discord_server_id)
	req, err := http.NewRequest("GET", guildUrl, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return discordgo.Guild{}, err
	}
	log.Println(BOT_TOKEN)
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
