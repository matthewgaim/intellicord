package db

import (
	"context"
	"log"
	"time"

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
	OwnerID         string `json:"owner_id"`
	JoinedAt        string `json:"joined_at"`
}

func GetRegisteredServers(userID string) ([]JoinedServersInfo, error) {
	rows, err := ai.DbPool.Query(context.Background(), `
    SELECT * FROM joined_servers WHERE owner_id = $1
    `, userID)
	if err != nil {
		log.Printf("Error getting joined servers: %v", err)
		return nil, err
	}
	defer rows.Close()

	var serversInfo []JoinedServersInfo
	for rows.Next() {
		var id int
		var discord_server_id string
		var owner_id string
		var joined_at time.Time

		err := rows.Scan(&id, &discord_server_id, &owner_id, &joined_at)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		serversInfo = append(serversInfo, JoinedServersInfo{
			ID:              id,
			DiscordServerID: discord_server_id,
			OwnerID:         owner_id,
			JoinedAt:        joined_at.String(),
		})
	}

	return serversInfo, nil
}
