package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/openai/openai-go"
	"github.com/pgvector/pgvector-go"
)

const (
	ChunkSize     = 512
	OverlapSize   = 128
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
		Admit when you don't know something.
		Avoid harmful, inappropriate, or NSFW content.
		Respect user privacy—never store personal data.

	5. Error Handling
		If a request is impossible, briefly explain why.
		Suggest alternatives when relevant.
		Warn users if limits (e.g., Discord's message cap) apply.
	
	Stay concise, clear, and helpful at all times.
	`
)

var oai openai.Client
var OpenAIAPIKey string

func InitAI() {
	OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	oai = openai.NewClient()
}

func ChunkAndEmbed(ctx context.Context, message_id string, content string, title string, doc_url string, discord_server_id string, fileSize int, channelID string, uploader_id string) error {
	_, err := db.DbPool.Exec(ctx, `
	INSERT INTO uploaded_files (
		discord_server_id,
		channel_id,
		uploader_id,
		title,
		file_url,
		file_size
	) VALUES ($1, $2, $3, $4, $5, $6)
	`, discord_server_id, channelID, uploader_id, title, doc_url, fileSize)
	if err != nil {
		return fmt.Errorf("Error uploading to uploaded_files: %v", err)
	}

	chunks := chunkText(content)
	embedChan := make(chan EmbedChannelObject, len(chunks))
	errChan := make(chan error, len(chunks))
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go newEmbedding(chunk, embedChan, errChan, &wg)
	}

	go func() {
		wg.Wait()
		close(embedChan)
		close(errChan)
	}()

	for embedChan != nil || errChan != nil {
		select {
		case embedding, ok := <-embedChan:
			if !ok {
				embedChan = nil
				continue
			}
			_, err := db.DbPool.Exec(ctx,
				"INSERT INTO chunks (message_id, title, doc_url, content, embedding, discord_server_id) VALUES ($1, $2, $3, $4, $5, $6)",
				message_id, title, doc_url, embedding.Chunk, pgvector.NewVector(embedding.Vector), discord_server_id)
			if err != nil {
				log.Printf("Error inserting chunk: %v", err)
			}

		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			log.Printf("Embedding error: %v", err)
		}
	}
	return nil
}

func LlmGenerateText(history []openai.ChatCompletionMessageParamUnion, userMessage string) (string, error) {
	history = slices.Insert(history, 0, openai.SystemMessage(SYSTEM_PROMPT))
	history = append(history, openai.UserMessage(userMessage))
	chatCompletion, err := oai.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: history,
		Model:    openai.ChatModelGPT4_1Nano,
	})
	if err != nil {
		return "", err
	}
	response := chatCompletion.Choices[0].Message.Content
	return response, nil
}

func QueryVectorDB(ctx context.Context, query string, rootMsgID string, numOfAttachments int) string {
	var embedChan = make(chan EmbedChannelObject)
	var errChan = make(chan error)
	var wg sync.WaitGroup

	wg.Add(1)
	go newEmbedding(query, embedChan, errChan, &wg)

	go func() {
		wg.Wait()
		close(embedChan)
		close(errChan)
	}()
	var queryVector []float32
	for {
		select {
		case embedding, ok := <-embedChan:
			if !ok {
				embedChan = nil
			} else {
				queryVector = embedding.Vector
			}
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			} else {
				log.Printf("Embedding error: %v", err)
				return ""
			}
		}

		if embedChan == nil && errChan == nil {
			break
		}
	}

	rows, err := db.DbPool.Query(ctx, `
		SELECT id, content, title, embedding <-> $1 AS distance 
		FROM chunks 
		WHERE message_id = $2
		ORDER BY distance 
		LIMIT $3`, pgvector.NewVector(queryVector), rootMsgID, numOfAttachments+1)
	if err != nil {
		log.Println("Error querying nearest neighbors:", err)
		return ""
	}
	defer rows.Close()
	var context []string
	for rows.Next() {
		var content string
		var title string
		var distance float32
		var id int
		err := rows.Scan(&id, &content, &title, &distance)
		if err != nil {
			log.Println("Error scanning row:", err)
			return ""
		}
		content = fmt.Sprintf("%s: %s", title, content)
		context = append(context, content)
		fmt.Printf("Relevant chunk (#%d) Distance: %f\n", id, distance)
	}
	result := strings.Join(context, "\n")
	return result
}

type EmbedChannelObject struct {
	Chunk  string
	Vector []float32
}

func newEmbedding(text string, embedChannel chan EmbedChannelObject, errChannel chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	embeddingInput := openai.EmbeddingNewParamsInputUnion{
		OfArrayOfStrings: []string{text},
	}
	res, err := oai.Embeddings.New(context.Background(), openai.EmbeddingNewParams{
		Input: embeddingInput,
		Model: openai.EmbeddingModelTextEmbedding3Small,
	})
	if err != nil {
		log.Printf("Error embedding text: %v", err)
		errChannel <- err
	} else {
		// float64 to 32 conversion for pgvector
		var embedding32 []float32
		for _, ve := range res.Data[0].Embedding {
			embedding32 = append(embedding32, float32(ve))
		}
		embedChannel <- EmbedChannelObject{Chunk: text, Vector: embedding32}
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

func DeleteEmbeddings(ctx context.Context, message_id string) {
	db.DbPool.Exec(ctx, "DELETE FROM chunks WHERE message_id = $1", message_id)
	log.Printf("Deleted chunks from message: %s", message_id)
}
