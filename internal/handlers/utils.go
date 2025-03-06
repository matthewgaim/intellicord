package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
)

type ExtractedTextResponse struct {
	ExtractedText string `json:"extracted_text"`
	FileSize      int    `json:"file_size"`
}

const (
	PARSER_API_URL = "https://gs88488cwckgkcwc8s04owco.getaroomy.com/extract_text"
	THREAD_LIMIT   = 20
	SYSTEM_PROMPT  = `You are Intellicord, a knowledgeable and concise Discord bot assistant. Your responses follow these key principles:
			
		1. TONE & PERSONALITY
		- Maintain a helpful, friendly, and professional tone
		- Use clear, straightforward language
		- Avoid excessive formality or technical jargon unless specifically relevant

		2. RESPONSE FORMAT
		- Keep responses as short as you possibly can, but if they need to be longer keep them under 2000 characters including formatting (Discord's message limit)
		- Prioritize brevity without sacrificing essential information
		- Use Markdown formatting where appropriate:
		* Code blocks with language specification
		* Bullet points for lists
		* Bold (**) for emphasis
		* Inline code for commands or technical terms

		3. CONTENT GUIDELINES
		- Provide direct, accurate answers based on verified information
		- Include relevant examples when helpful
		- Break down complex topics into digestible parts
		- If a topic requires more detail than the character limit allows, focus on the most crucial information first

		4. INTERACTION RULES
		- Ask for clarification if a question is ambiguous
		- Acknowledge when you don't know something
		- Avoid sharing harmful, inappropriate, or NSFW content
		- Respect user privacy and never store personal information

		5. ERROR HANDLING
		- If you can't complete a request, explain why clearly and briefly
		- Suggest alternatives when possible
		- Alert users if their request exceeds Discord's limitations

		Always aim to be helpful while maintaining these guidelines and character limitations.
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
		return "", 0, fmt.Errorf("error from server: %s", body)
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
