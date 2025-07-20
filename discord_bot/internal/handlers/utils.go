package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
)

type ExtractedTextResponse struct {
	ExtractedText string `json:"extracted_text"`
	FileSize      int    `json:"file_size"`
}

type ExtractedErrorResponse struct {
	Error string `json:"error"`
}

const (
	THREAD_LIMIT = 20
)

func GetThreadMessages(s *discordgo.Session, threadID string, botID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	msgs, err := s.ChannelMessages(threadID, THREAD_LIMIT, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("error fetching messages: %w", err)
	}

	var history []openai.ChatCompletionMessageParamUnion
	var msg *discordgo.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		msg = msgs[i]
		if msg.Author.ID == botID {
			history = append(history, openai.AssistantMessage(msg.Content))
		} else {
			user_msg := fmt.Sprintf("%s: %s", msg.Author.Username, msg.Content)
			history = append(history, openai.UserMessage(user_msg))
		}
	}
	return history, nil
}

func getFileTextAndSize(pdfURL string) (string, int, error) {
	payload := map[string]string{"file_url": pdfURL}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create JSON payload: %v", err)
	}

	PARSER_API_URL := fmt.Sprintf("%s/extract_text", os.Getenv("PARSER_API_URL"))
	resp, err := http.Post(PARSER_API_URL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", 0, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var result ExtractedErrorResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return "", 0, fmt.Errorf("failed to parse JSON response: %v", err)
		}
		return "", 0, fmt.Errorf("%s", result.Error)
	}

	var result ExtractedTextResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return result.ExtractedText, result.FileSize, nil
}

func sendResponseInChannel(session *discordgo.Session, channelID string, response string) {
	if len(response) >= 2000 {
		msg := ""
		for ind, char := range response {
			if ind != 0 && ind%2000 == 0 {
				session.ChannelMessageSend(channelID, msg)
				msg = ""
			} else {
				msg += string(char)
			}
		}
		if msg != "" {
			session.ChannelMessageSend(channelID, msg)
		}
	} else {
		session.ChannelMessageSend(channelID, response)
	}
}

func getRootMessageOfThread(s *discordgo.Session, channel *discordgo.Channel) (message *discordgo.Message, err error) {
	parentMessage, err := s.ChannelMessage(channel.ParentID, channel.ID)
	if err != nil {
		return nil, err
	}

	if parentMessage.ReferencedMessage != nil {
		replyMessage, err := s.ChannelMessage(parentMessage.ChannelID, parentMessage.ReferencedMessage.ID)
		if err == nil {
			return replyMessage, nil
		}
	}
	return parentMessage, nil
}

func NewDiscordWebhookMessage(webhookURL string, message string) {
	payload := map[string]string{
		"content": message,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
}
