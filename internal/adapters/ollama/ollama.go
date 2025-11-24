package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

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
	payload := AIRequest{
		Model:  o.modelName,
		Prompt: buildedPrompt,
		Stream: true,
		Format: "json",
	}
	raw, err := o.sendRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	return streamOllamaResponse(raw)
}

func (o *OllamaAdapter) sendRequest(ctx context.Context, payload AIRequest) (*http.Response, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return nil, fmt.Errorf("internal error")
	}

	url := fmt.Sprintf("%s%s", o.baseURL, "/api/generate")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	raw, err := o.httpClient.Do(req)
	if err != nil {
		log.Println("Error connecting to Ollama API:", err)
		return nil, fmt.Errorf("ollama API connection error")
	}
	defer raw.Body.Close()

	if raw.StatusCode != http.StatusOK {
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
	defer resp.Body.Close() // Ensure the body is closed when done

	reader := bufio.NewReader(resp.Body)
	var fullResponseText string

	fmt.Println("--- Streaming Response Start ---")

	for {
		line, err := reader.ReadBytes('\n')

		// 1. Check for EOF first, before checking line length.
		// EOF means we have consumed everything successfully.
		if err == io.EOF {
			break
		}

		// 2. Handle non-EOF errors (e.g., network disconnects, mid-stream failure)
		if err != nil {
			return nil, fmt.Errorf("error reading response stream: %w", err)
		}

		// Convert byte slice to string and trim leading/trailing whitespace
		lineStr := strings.TrimSpace(string(line))

		if lineStr == "" {
			continue // Skip empty lines cleanly
		}

		var chunk OllamaChunk

		// 3. Unmarshal the chunk. Critical: Do not skip unmarshal errors silently.
		if err := json.Unmarshal([]byte(lineStr), &chunk); err != nil {
			// Log the raw line for debugging. This line might be an unhandled error message.
			fmt.Printf("\n❌ WARNING: Failed to unmarshal JSON chunk. Raw line: [%s], Error: %v\n", lineStr, err)
			// If it's not JSON, assume it's a critical error message and stop.
			return nil, fmt.Errorf("invalid JSON or unexpected message received in stream: %s", lineStr)
		}

		// Concatenate the response text
		fullResponseText += chunk.Response
		fmt.Print(chunk.Response)

		// Check for the 'done' signal to break the loop
		if chunk.Done {
			break
		}
	}

	fmt.Println("\n--- Streaming Response End ---")

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

// func streamOllamaResponse(resp *http.Response) (*domain.Transaction, error) {
// 	reader := bufio.NewReader(resp.Body)
// 	var fullResponseText string

// 	fmt.Println("--- Streaming Response Start ---")

// 	for {
// 		// Read a line from the stream
// 		line, err := reader.ReadBytes('\n')
// 		if err != nil {
// 			// If it's EOF, we're done with the entire stream
// 			break
// 		}

// 		// Ollama chunks are a single JSON object per line (or chunk)
// 		if len(line) > 0 {
// 			var chunk OllamaChunk

// 			// Unmarshal the chunk into our struct
// 			if err := json.Unmarshal(line, &chunk); err != nil {
// 				// Handle potential parsing errors or empty lines
// 				continue
// 			}

// 			// Concatenate the response text
// 			fullResponseText += chunk.Response

// 			// Print token for streaming visualization
// 			fmt.Print(chunk.Response)

// 			// Check for the 'done' signal to break the loop
// 			if chunk.Done {
// 				break
// 			}
// 		}
// 	}

// 	fmt.Println("\n--- Streaming Response End ---")

// 	// 5. Final processing: Parse the concatenated JSON string
// 	var finalJSON domain.Transaction
// 	if err := json.Unmarshal([]byte(fullResponseText), &finalJSON); err != nil {
// 		fmt.Printf("\nError parsing final JSON: %v\n", err)
// 		// Print the raw text for debugging if parsing fails
// 		fmt.Printf("Raw text: %s\n", fullResponseText)
// 		return nil, err
// 	}

// 	// Output the final, correctly parsed JSON object
// 	fmt.Println("\n✅ Successfully Parsed Final JSON Object:")

// 	return &finalJSON, nil
// }
