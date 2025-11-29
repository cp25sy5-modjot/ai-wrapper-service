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
	"github.com/rs/zerolog"
)
var logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

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
	decoder := json.NewDecoder(resp.Body)
	var fullText string

	for decoder.More() {
		var chunk OllamaChunk

		if err := decoder.Decode(&chunk); err != nil {
			logger.Error().Err(err).Msg("error decoding ollama stream chunk")
			break
		}

		fullText += chunk.Response
		// (optional) log each chunk:
		logger.Debug().Str("chunk", chunk.Response).Msg("ollama chunk")
	}

	logger.Info().
		Str("full_response", fullText).
		Msg("ollama streamed response")

	if fullText == "" {
		return nil, errors.New("stream closed without receiving any response content")
	}

	// 4. Final processing: Parse the concatenated JSON string
	var finalJSON domain.Transaction
	if err := json.Unmarshal([]byte(fullText), &finalJSON); err != nil {
		fmt.Printf("\nError parsing final JSON structure: %v\n", err)
		fmt.Printf("Raw text received: %s\n", fullText)
		return nil, err
	}

	fmt.Println("\n✅ Successfully Parsed Final JSON Object.")
	return &finalJSON, nil
}
