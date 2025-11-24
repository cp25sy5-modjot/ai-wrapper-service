package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
)

type OllamaAdapter struct {
	baseURL   string
	modelName string
}

func NewOllamaAdapter() *OllamaAdapter {
	hostIp := os.Getenv("HOST_IP")
	if hostIp == "" {
		log.Fatal("missing HOST_IP")
	}
	baseURL := fmt.Sprintf("http://%s:11434%s", hostIp, "/api/generate")

	return &OllamaAdapter{
		baseURL:   baseURL,
		modelName: "modjot-ai",
	}
}

func (o *OllamaAdapter) ParseOcrResponseToJson(text string, categories []string) (*domain.Transaction, error) {
	buildedPrompt := buildPrompt(text, categories)
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
	return parseResponseToJSON(raw)
}

func parseResponseToJSON(data []byte) (*domain.Transaction, error) {
	var resp domain.Transaction
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OCR response: %w", err)
	}

	log.Printf("OCR extracted text: %v", resp)
	return &resp, nil
}

func (o *OllamaAdapter) sendRequest(payload AIRequest) ([]byte, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return nil, fmt.Errorf("internal error")
	}

	// 3. Send the HTTP POST request to the Docker container
	url := fmt.Sprintf("%s%s", o.baseURL, "/api/generate")
	raw, err := http.Post(
		url,
		"application/json",
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		log.Println("Error connecting to Ollama API:", err)
		return nil, fmt.Errorf("ollama API connection error")
	}
	defer raw.Body.Close()

	if raw.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(raw.Body)
		return nil, fmt.Errorf("ollama API error: %d - %s", raw.StatusCode, string(body))
	}

	return io.ReadAll(raw.Body)
}

func buildPrompt(text string, categories []string) string {
	prompt := fmt.Sprintf("Categories Available: %v\nOCR Text:\n%s", categories, text)
	return prompt
}
