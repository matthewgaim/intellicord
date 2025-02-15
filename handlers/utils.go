package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
)

type ExtractedTextResponse struct {
	ExtractedText string `json:"extracted_text"`
}

var PARSER_API_URL = "https://gs88488cwckgkcwc8s04owco.getaroomy.com/extract_text"

func GetThreadMessages(s *discordgo.Session, threadID string, botID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	msgs, err := s.ChannelMessages(threadID, 20, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("error fetching messages: %w", err)
	}
	var history []openai.ChatCompletionMessageParamUnion
	log.Println("\n\nThread History")
	var msg *discordgo.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		msg = msgs[i]
		if msg.Author.ID == botID {
			log.Printf("Bot: %v\n", msg.Content)
			history = append(history, openai.AssistantMessage(msg.Content))
		} else {
			log.Printf("User: %v\n", msg.Content)
			history = append(history, openai.UserMessage(msg.Content))
		}
	}
	return history, nil
}

func getFileType(file string) string {
	split := strings.Split(file, ".")
	fileType := split[len(split)-1]
	return fileType
}

func getFileText(pdfURL string) (string, error) {
	payload := map[string]string{"pdf_url": pdfURL}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON payload: %v", err)
	}

	resp, err := http.Post(PARSER_API_URL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from server: %s", body)
	}

	var result ExtractedTextResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return result.ExtractedText, nil
}
