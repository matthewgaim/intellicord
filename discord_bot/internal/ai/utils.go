package ai

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go/v2"
	"google.golang.org/genai"
)

func discordMessagesToOpenAIMessages(msgs []*discordgo.Message, botID string) []openai.ChatCompletionMessageParamUnion {
	var history []openai.ChatCompletionMessageParamUnion
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Author.ID == botID {
			history = append(history, openai.AssistantMessage(msg.Content))
		} else {
			user_msg := fmt.Sprintf("%s: %s", msg.Author.Username, msg.Content)
			history = append(history, openai.UserMessage(user_msg))
		}
	}
	return history
}

func discordMessagesToGeminiMessages(msgs []*discordgo.Message, botID string) []*genai.Content {
	var history []*genai.Content
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Author.ID == botID {
			history = append(history, genai.NewContentFromText(msg.Content, genai.RoleModel))
		} else {
			user_msg := fmt.Sprintf("%s: %s", msg.Author.Username, msg.Content)
			history = append(history, genai.NewContentFromText(user_msg, genai.RoleUser))
		}
	}
	return history
}
