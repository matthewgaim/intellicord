package ai

import (
	"context"
	"log"

	"github.com/openai/openai-go"
)

var oai *openai.Client

func InitAI() {
	oai = openai.NewClient()
}

func LlmGenerateText(history []openai.ChatCompletionMessageParamUnion, userMessage string) string {
	history = append(history, openai.UserMessage(userMessage))
	chatCompletion, err := oai.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: openai.F(history),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})
	if err != nil {
		panic(err.Error())
	}
	response := chatCompletion.Choices[0].Message.Content
	log.Printf("New AI Message: %v\n", response)
	return response
}
