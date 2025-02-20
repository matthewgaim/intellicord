package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go"
	"github.com/pgvector/pgvector-go"
)

const (
	ChunkSize   = 512
	OverlapSize = 128
)

var oai *openai.Client
var DbPool *pgxpool.Pool
var OpenAIAPIKey string

func InitAI() {
	var err error

	oai = openai.NewClient()
	DATABASE_URL := os.Getenv("DATABASE_URL")
	OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")

	DbPool, err = pgxpool.New(context.Background(), DATABASE_URL)
	if err != nil {
		log.Fatal("Unable to connect to database:", err)
	}

	_, err = DbPool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		log.Fatal("Error enabling pgvector:", err)
	}

	_, err = DbPool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS chunks (
		id SERIAL PRIMARY KEY,
		message_id TEXT,
		title TEXT,
		doc_url TEXT,
		content TEXT,
		embedding vector(1536) -- OpenAI embeddings are 1536-dimensional
	);`)
	if err != nil {
		log.Fatal("Error creating chunks table:", err)
	}
}

func ChunkAndVectorize(ctx context.Context, message_id string, doc string, title string, doc_url string) {
	chunks := chunkText(doc)
	for _, chunk := range chunks {
		embedding, err := getEmbedding(chunk)
		if err != nil {
			log.Fatalf("Error generating embedding for chunk: %v", err)
		}

		_, err = DbPool.Exec(ctx, "INSERT INTO chunks (message_id, title, doc_url, content, embedding) VALUES ($1, $2, $3, $4, $5)", message_id, title, doc_url, chunk, pgvector.NewVector(embedding))
		if err != nil {
			log.Fatalf("Failed to add chunk for '%v': %v", title, err)
		}
	}
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

func QueryVectorDB(ctx context.Context, query string, rootMsgID string, numOfAttachments int) string {
	queryVector, err := getEmbedding(query)
	if err != nil {
		log.Fatal("Error generating query embedding:", err)
	}
	fmt.Printf("Attachments to query: %d", numOfAttachments)
	rows, err := DbPool.Query(ctx, `
	SELECT content, title, embedding <-> $1 AS distance 
	FROM chunks 
	WHERE message_id = $2
	AND (embedding <-> $1) >= 0.5
	ORDER BY distance 
	LIMIT $3`, pgvector.NewVector(queryVector), rootMsgID, numOfAttachments)
	if err != nil {
		log.Fatal("Error querying nearest neighbors:", err)
	}
	defer rows.Close()
	var context []string
	if rows.Next() {
		var content string
		var title string
		var distance float32
		err := rows.Scan(&content, &title, &distance)
		if err != nil {
			log.Fatal("Error scanning row:", err)
		}
		content = fmt.Sprintf("%s: %s", title, content)
		fmt.Println(content[:50])
		context = append(context, content)
	} else {
		return "No additional context found."
	}
	result := strings.Join(context, "\n")
	return result
}

func getEmbedding(text string) ([]float32, error) {
	url := "https://api.openai.com/v1/embeddings"
	payload := strings.NewReader(fmt.Sprintf(`{"input": %q, "model": "text-embedding-ada-002"}`, text))

	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Set("Authorization", "Bearer "+OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return result.Data[0].Embedding, nil
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
	DbPool.Exec(ctx, "DELETE FROM chunks WHERE message_id = $1", message_id)
	log.Printf("Deleted chunks from message: %s", message_id)
}
