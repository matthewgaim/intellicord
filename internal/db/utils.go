package db

import (
	"context"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/matthewgaim/intellicord/internal/ai"
)

func getCountOfUploadsPerDayInWeek(user_id string) ([]map[string]interface{}, error) {
	rows, err := ai.DbPool.Query(context.Background(), `
		SELECT DATE(uf.uploaded_at) AS upload_date, COUNT(uf.id) AS total_files
		FROM uploaded_files uf
		JOIN joined_servers js ON uf.discord_server_id = js.discord_server_id
		WHERE js.owner_id = $1 AND uf.uploaded_at >= NOW() - INTERVAL '7 days'
		GROUP BY upload_date
		ORDER BY upload_date DESC
	`, user_id)
	if err != nil {
		log.Printf("Error fetching total files per day: %v", err)
		return nil, err
	}
	defer rows.Close()

	var filesPerDay []map[string]interface{}
	dateFileUploadedData := make(map[string]int) // To track days with file uploads

	for rows.Next() {
		var uploadDate time.Time
		var totalFiles int
		if err := rows.Scan(&uploadDate, &totalFiles); err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		dateStr := uploadDate.Format("01/02")
		dateFileUploadedData[dateStr] = totalFiles
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
	return filesPerDay, nil
}

func getFileDetails(user_id string) ([]FileInformation, error) {
	rows, err := ai.DbPool.Query(context.Background(), `
		SELECT uf.title, uf.file_size, DATE(uf.uploaded_at) AS upload_date
		FROM uploaded_files uf
		JOIN joined_servers js ON uf.discord_server_id = js.discord_server_id
		WHERE js.owner_id = $1 AND uf.uploaded_at >= NOW() - INTERVAL '7 days'
		ORDER BY upload_date DESC
	`, user_id)
	if err != nil {
		log.Printf("Error fetching file details: %v", err)
		return nil, err
	}
	defer rows.Close()

	var fileDetails []FileInformation

	for rows.Next() {
		var uploadDate time.Time
		var title string
		var fileSize int64

		if err := rows.Scan(&title, &fileSize, &uploadDate); err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		titleSplit := strings.Split(title, ".")
		fileType := strings.ToUpper(titleSplit[len(titleSplit)-1])

		fileDetails = append(fileDetails, FileInformation{
			Name:         title,
			Type:         fileType,
			Size:         fileSize,
			AnalyzedDate: uploadDate.Format("01/02"),
		})
	}
	return fileDetails, nil
}

func getTotalMessages(user_id string) (int, error) {
	var totalMessages int
	err := ai.DbPool.QueryRow(context.Background(), `
        SELECT COUNT(ml.id) 
        FROM message_logs ml
        JOIN joined_servers js ON ml.discord_server_id = js.discord_server_id
        WHERE js.owner_id = $1
    `, user_id).Scan(&totalMessages)
	if err != nil {
		log.Printf("Error fetching total messages: %v", err)
		return 0, err
	}
	return totalMessages, nil
}
