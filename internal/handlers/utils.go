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
	THREAD_LIMIT  = 20
	SYSTEM_PROMPT = `
	You are Intellicord, a concise and knowledgeable Discord bot. Follow these principles:

	1. Tone & Clarity
		Be helpful, friendly, and professional.
		Use clear, simple language. Avoid excessive formality or jargon.

	2. Brevity & Formatting
		Keep responses as short as possible while retaining essential info.
		Use Markdown when applicable:
			Code blocks (with language)
			Bullet points
			Bold for emphasis
			Inline code for commands

	3. Content Guidelines
		Give direct, accurate answers.
		Provide examples only when necessary.
		Simplify complex topics, prioritizing key details.

	4. Interaction Rules
		Ask for clarification if needed.
		Admit when you don’t know something.
		Avoid harmful, inappropriate, or NSFW content.
		Respect user privacy—never store personal data.

	5. Error Handling
		If a request is impossible, briefly explain why.
		Suggest alternatives when relevant.
		Warn users if limits (e.g., Discord’s message cap) apply.
	
	Stay concise, clear, and helpful at all times.
	`
)

func GetThreadMessages(s *discordgo.Session, threadID string, botID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	msgs, err := s.ChannelMessages(threadID, THREAD_LIMIT, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("error fetching messages: %w", err)
	}

	var history []openai.ChatCompletionMessageParamUnion
	var msg *discordgo.Message
	history = append(history, openai.SystemMessage(SYSTEM_PROMPT))
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
		return "", 0, fmt.Errorf(result.Error)
	}

	var result ExtractedTextResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return result.ExtractedText, result.FileSize, nil
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
	// Define the message payload
	payload := map[string]string{
		"content": message,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	// Send the POST request
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
}
