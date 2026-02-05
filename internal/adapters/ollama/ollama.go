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
		httpClient: &http.Client{},
	}
}
func (o *OllamaAdapter) ParseOcrResponseToJson(ctx context.Context, text string, categories []string) (*domain.Transaction, error) {
	preOCR := PreprocessOCR(text)
	payload := buildAIRequest(preOCR, categories)

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

func buildAIRequest(ocrText string, categories []string) AIRequest {
	prompt := fmt.Sprintf(
		`Return only minified JSON in one line. No comments. No markdown.

CRITICAL RULES:
- Categories MUST be exactly one of: %v. Never invent new categories.
- Only real purchased products may appear in items[].
- NEVER include store name, branch, receipt header, tax id, POS id, totals, VAT, CASH, Change, discount lines, or thank-you text.
- Any token where numbers touch letters (example: "470X", "3S") is a PRODUCT CODE, NOT a price.
- A price MUST be a standalone decimal number at the END of a product line.
- Lines containing quantity/unit patterns such as "@", "PCS", "หน่วย" are NOT products.
- Any line starting with a number followed by "@" is NEVER a product.
- Quantity/unit lines belong to the previous product and must be merged into that product.
- Prefer including uncertain items rather than dropping them unless clearly a header/total.
- Remove prefixes like "1P", "2P", "A#", "P#", "A ", "P ".
- Titles must be short product names only.
- date MUST be ISO-8601. Include time if present: YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS(+TZ)
- If a line appears to be a product but is messy OCR, still include it.
- Only drop lines that are clearly totals, VAT, CASH, Change, receipt numbers, or discounts.
- If price is unclear, infer from nearest decimal number.

OUTPUT JSON SCHEMA:
{"title":string,"date":string,"items":[{"title":string,"price":number,"category":string}]}

OCR TEXT:
%s`,
		categories,
		ocrText,
	)

	return AIRequest{
		Model:  "modjot-ai-v4",
		Prompt: prompt,
		Stream: false,
		Format: "json",
		Options: &AIOptions{
			NumPredict:  4096,
			Temperature: 0,
		},
	}
}


func CleanOCR(raw string) string {
	s := raw

	// remove ASCII table chars
	s = regexp.MustCompile(`[|]+`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`[-_=]{2,}`).ReplaceAllString(s, " ")

	// remove unicode box drawing
	s = regexp.MustCompile(`[│─┼┌┐└┘╔╗╚╝═]+`).ReplaceAllString(s, " ")

	// collapse multiple newlines
	s = regexp.MustCompile(`\n{2,}`).ReplaceAllString(s, "\n")

	s = strings.TrimSpace(s)

	// normalize spaces
	s = regexp.MustCompile(`\s{2,}`).ReplaceAllString(s, " ")

	return s
}

func MergeQtyLines(s string) string {
	lines := strings.Split(s, "\n")

	qtyRe := regexp.MustCompile(`^\s*\d+(\.\d+)?@\s*\d+(\.\d+)?`)

	var out []string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if qtyRe.MatchString(line) && len(out) > 0 {
			out[len(out)-1] = out[len(out)-1] + " " + line
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func FixThaiOCR(s string) string {
	for bad, good := range thaiFix {
		s = strings.ReplaceAll(s, bad, good)
	}
	return s
}

func NormalizeNumbers(s string) string {
	// remove spaces inside numbers: 1,     000 -> 1,000
	re := regexp.MustCompile(`(\d)[,\s]+(\d{3})`)
	return re.ReplaceAllString(s, `$1$2`)
}

func PreprocessOCR(raw string) string {
	s := CleanOCR(raw)
	s = NormalizeNumbers(s)
	s = MergeQtyLines(s)
	s = FixThaiOCR(s)

	return s
}
