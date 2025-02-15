package ai

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/openai/openai-go"
	"github.com/philippgille/chromem-go"
)

var oai *openai.Client
var chromemCollection *chromem.Collection

const (
	ChunkSize   = 512
	OverlapSize = 128
)

func InitAI() {
	var err error

	oai = openai.NewClient()
	db := chromem.NewDB()
	chromemCollection, err = db.CreateCollection("chunked-user-docs", nil, nil)
	if err != nil {
		panic(err)
	}
}

func PrepareDocForQuerying(ctx context.Context, doc string, title string) {
	addChunksToVectorDB(ctx, doc, title)
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

func QueryVectorDB(ctx context.Context, query string) string {
	res, err := chromemCollection.Query(ctx, query, 1, nil, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("ID: %v\nSimilarity: %v\nContent: %v\n", res[0].ID, res[0].Similarity, res[0].Content)
	return res[0].Content
}

func addChunksToVectorDB(ctx context.Context, doc string, title string) {
	chunks := chunkText(doc)
	var ids []string
	var metadatas []map[string]string
	var contents []string
	for i, chunk := range chunks {
		ids = append(ids, fmt.Sprintf("doc1_chunk_%d", i))
		meta := map[string]string{
			"title": title,
		}
		metadatas = append(metadatas, meta)
		contents = append(contents, chunk)
	}
	err := chromemCollection.Add(ctx, ids, nil, metadatas, contents)
	if err != nil {
		log.Fatalf("Failed to add chunks for '%v': %v", title, err)
	}

}

func chunkText(text string) []string {
	var chunks []string

	headingRegex := regexp.MustCompile(`(?m)^#.*`)
	sections := headingRegex.Split(text, -1)

	if len(sections) > 1 { // Chunk by headings
		for _, section := range sections {
			chunks = append(chunks, section)
		}
	} else { // Chunk by fixed length
		words := strings.Fields(text)
		for i := 0; i < len(words); i += (ChunkSize - OverlapSize) {
			end := i + ChunkSize
			if end > len(words) {
				end = len(words)
			}
			chunks = append(chunks, strings.Join(words[i:end], " "))
		}
	}
	return chunks
}
