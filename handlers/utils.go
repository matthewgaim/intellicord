package handlers

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
)

func GetThreadMessages(s *discordgo.Session, threadID string, botID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	msgs, err := s.ChannelMessages(threadID, 10, "", "", "")
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
