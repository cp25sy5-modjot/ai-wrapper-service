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
	"regexp"
	"strings"
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
		modelName:  "modjot-ai-v2",
		httpClient: &http.Client{},
	}
}
func (o *OllamaAdapter) ParseOcrResponseToJson(ctx context.Context, text string, categories []string) (*domain.Transaction, error) {
	cleanOcr := CleanOCR(text)
	prompt := buildPrompt(cleanOcr, categories)
	log.Printf("Ollama Prompt: %s", prompt)

	payload := AIRequest{
		Model:  o.modelName,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	// pass ctx down so gRPC cancel/timeout propagates
	raw, err := o.sendRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer raw.Body.Close()

	return parseNonStreamOllamaResponse(raw)
}

func (o *OllamaAdapter) sendRequest(ctx context.Context, payload AIRequest) (*http.Response, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		logger.Error().Err(err).Msg("Error marshalling JSON")
		return nil, fmt.Errorf("internal error")
	}

	url := fmt.Sprintf("%s%s", o.baseURL, "/api/generate")
	ollamaTimeout := 5 * time.Minute

	// tie Ollama timeout to incoming ctx
	ollamaCtx, cancel := context.WithTimeout(ctx, ollamaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ollamaCtx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	raw, err := o.httpClient.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("Error connecting to Ollama API")
		return nil, fmt.Errorf("ollama API connection error")
	}

	if raw.StatusCode != http.StatusOK {
		defer raw.Body.Close()
		body, _ := io.ReadAll(raw.Body)
		return nil, fmt.Errorf("ollama API error: %d - %s", raw.StatusCode, string(body))
	}

	return raw, nil
}

func parseNonStreamOllamaResponse(resp *http.Response) (*domain.Transaction, error) {
	// 1) decode Ollama's response object
	var ollamaResp struct {
		Model    string `json:"model"`
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.Error().Err(err).Msg("failed to decode ollama response json")
		return nil, err
	}

	// 2) log raw text from Ollama (this is what you wanted)
	logger.Info().
		Str("model", ollamaResp.Model).
		Str("full_response", ollamaResp.Response).
		Msg("ollama full response")

	if ollamaResp.Response == "" {
		return nil, errors.New("ollama returned empty response")
	}

	// 3) parse the JSON string from `response` into your domain.Transaction
	var finalJSON domain.Transaction
	if err := json.Unmarshal([]byte(ollamaResp.Response), &finalJSON); err != nil {
		logger.Error().
			Err(err).
			Str("raw_text", ollamaResp.Response).
			Msg("failed to unmarshal transaction JSON from ollama")
		return nil, err
	}

	return &finalJSON, nil
}

func buildPrompt(text string, categories []string) string {
	prompt := fmt.Sprintf("Categories Available: %v\nOCR Text:\n%s", categories, text)
	return prompt
}

func CleanOCR(raw string) string {
	s := raw

	// collapse multiple newlines
	s = regexp.MustCompile(`\n{2,}`).ReplaceAllString(s, "\n")

	// remove weird spacing
	s = strings.TrimSpace(s)

	// remove duplicated spaces
	s = regexp.MustCompile(`\s{2,}`).ReplaceAllString(s, " ")

	// log.Printf("Cleaned OCR text: %q", s)
	return s
}
