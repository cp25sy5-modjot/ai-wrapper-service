package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
)

type OllamaAdapter struct {
	baseURL    string
	modelName  string
	httpClient *http.Client
}

func NewOllamaAdapter() *OllamaAdapter {
	hostIp := os.Getenv("HOST_IP")
	if hostIp == "" {
		log.Fatal("missing HOST_IP")
	}
	baseURL := fmt.Sprintf("http://%s:11434", hostIp)

	return &OllamaAdapter{
		baseURL:    baseURL,
		modelName:  "modjot-ai",
		httpClient: &http.Client{},
	}
}

func (o *OllamaAdapter) ParseOcrResponseToJson(ctx context.Context, text string, categories []string) (*domain.Transaction, error) {
	buildedPrompt := buildPrompt(text, categories)
	log.Printf("Ollama Prompt: %s", buildedPrompt)
	payload := AIRequest{
		Model:  o.modelName,
		Prompt: buildedPrompt,
		Stream: false,
		Format: "json",
	}
	raw, err := o.sendRequest(payload)
	if err != nil {
		return nil, err
	}

	defer raw.Body.Close()

	return streamOllamaResponse(raw)
}

func (o *OllamaAdapter) sendRequest(payload AIRequest) (*http.Response, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return nil, fmt.Errorf("internal error")
	}

	url := fmt.Sprintf("%s%s", o.baseURL, "/api/generate")
	ollamaTimeout := 5 * time.Minute

	ollamaCtx, ollamaCancel := context.WithTimeout(context.Background(), ollamaTimeout)
	defer ollamaCancel()

	req, err := http.NewRequestWithContext(ollamaCtx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	raw, err := o.httpClient.Do(req)
	if err != nil {
		log.Println("Error connecting to Ollama API:", err)
		return nil, fmt.Errorf("ollama API connection error")
	}

	if raw.StatusCode != http.StatusOK {
		// IMPORTANT: If status is not 200, we MUST close the body here.
		defer raw.Body.Close()
		body, _ := io.ReadAll(raw.Body)
		return nil, fmt.Errorf("ollama API error: %d - %s", raw.StatusCode, string(body))
	}

	return raw, nil
}

func buildPrompt(text string, categories []string) string {
	prompt := fmt.Sprintf("Categories Available: %v\nOCR Text:\n%s", categories, text)
	return prompt
}

func streamOllamaResponse(resp *http.Response) (*domain.Transaction, error) {
	body, _ := io.ReadAll(resp.Body)
	var fullResponseText string

	var chunk OllamaChunk

	// 3. Unmarshal the chunk. Critical: Do not skip unmarshal errors silently.
	if err := json.Unmarshal([]byte(body), &chunk); err != nil {
		// Log the raw line for debugging. This line might be an unhandled error message.
		fmt.Printf("\n❌ WARNING: Failed to unmarshal JSON chunk. Raw line: [%s], Error: %v\n", body, err)
		// If it's not JSON, assume it's a critical error message and stop.
		return nil, fmt.Errorf("invalid JSON or unexpected message received in stream: %s", body)
	}

	// Concatenate the response text
	fullResponseText += chunk.Response
	fmt.Print("response chunk: " + fullResponseText)

	if fullResponseText == "" {
		return nil, errors.New("stream closed without receiving any response content")
	}

	// 4. Final processing: Parse the concatenated JSON string
	var finalJSON domain.Transaction
	if err := json.Unmarshal([]byte(fullResponseText), &finalJSON); err != nil {
		fmt.Printf("\nError parsing final JSON structure: %v\n", err)
		fmt.Printf("Raw text received: %s\n", fullResponseText)
		return nil, err
	}

	fmt.Println("\n✅ Successfully Parsed Final JSON Object.")
	return &finalJSON, nil
}
