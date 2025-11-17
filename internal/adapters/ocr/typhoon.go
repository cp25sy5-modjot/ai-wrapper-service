package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

type TyphoonOCR struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	defaultOcr OcrParams
}

func NewTyphoonOCR() *TyphoonOCR {
	apiKey := os.Getenv("OPENTYPHOON_API_KEY")
	if apiKey == "" {
		log.Fatal("missing OPENTYPHOON_API_KEY")
	}

	return &TyphoonOCR{
		apiKey:     apiKey,
		baseURL:    "https://api.opentyphoon.ai/v1/ocr",
		httpClient: &http.Client{},
		defaultOcr: OcrParams{
			Model:             "typhoon-ocr",
			TaskType:          "default",
			MaxTokens:         16000,
			Temperature:       0.1,
			TopP:              0.6,
			RepetitionPenalty: 1.2,
			Pages:             nil, // all pages by default
		},
	}
}

func (t *TyphoonOCR) ExtractText(ctx context.Context, img []byte) (string, error) {
	body, writer, err := buildMultipartRequest(img, t.defaultOcr)
	if err != nil {
		return "", err
	}

	raw, err := t.sendOcrRequest(ctx, body, writer)
	if err != nil {
		return "", err
	}

	return parseOcrResponse(raw)
}

func (t *TyphoonOCR) ExtractTextWithParams(ctx context.Context, img []byte, params OcrParams) (string, error) {
	body, writer, err := buildMultipartRequest(img, params)
	if err != nil {
		return "", err
	}

	raw, err := t.sendOcrRequest(ctx, body, writer)
	if err != nil {
		return "", err
	}
	return parseOcrResponse(raw)
}

func buildMultipartRequest(image []byte, params OcrParams) (*bytes.Buffer, *multipart.Writer, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, nil, err
	}

	_, err = io.Copy(part, bytes.NewReader(image))
	if err != nil {
		return nil, nil, err
	}

	// Fields
	_ = writer.WriteField("task_type", params.TaskType)
	_ = writer.WriteField("max_tokens", strconv.Itoa(params.MaxTokens))
	_ = writer.WriteField("temperature", strconv.FormatFloat(params.Temperature, 'f', -1, 64))
	_ = writer.WriteField("top_p", strconv.FormatFloat(params.TopP, 'f', -1, 64))
	_ = writer.WriteField("repetition_penalty", strconv.FormatFloat(params.RepetitionPenalty, 'f', -1, 64))

	if len(params.Pages) > 0 {
		pagesJSON, _ := json.Marshal(params.Pages)
		_ = writer.WriteField("pages", string(pagesJSON))
	}

	_ = writer.Close()
	return &buf, writer, nil
}

func (t *TyphoonOCR) sendOcrRequest(ctx context.Context, body *bytes.Buffer, writer *multipart.Writer) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OCR API error: %d - %s", resp.StatusCode, string(raw))
	}

	return io.ReadAll(resp.Body)
}

func parseOcrResponse(raw []byte) (string, error) {
	var resp OcrResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal OCR response: %w", err)
	}

	text := resp.ExtractText()
	log.Printf("OCR extracted text: %q", text)
	return text, nil
}

func (r *OcrResponse) ExtractTextWithNaturalText() string {
	if len(r.Results) == 0 ||
		r.Results[0].Message == nil ||
		len(r.Results[0].Message.Choices) == 0 {
		return ""
	}

	return r.Results[0].Message.Choices[0].Message.Content
}

func (r *OcrResponse) ExtractText() string {
	if len(r.Results) == 0 ||
		r.Results[0].Message == nil ||
		len(r.Results[0].Message.Choices) == 0 {
		return ""
	}

	rawContent := r.Results[0].Message.Choices[0].Message.Content
	if rawContent == "" {
		return ""
	}

	// Try to parse {"natural_text": "..."} from content
	var inner struct {
		NaturalText string `json:"natural_text"`
	}

	if err := json.Unmarshal([]byte(rawContent), &inner); err == nil && inner.NaturalText != "" {
		return inner.NaturalText
	}

	// Fallback: if it's not JSON, just return the raw content
	return rawContent
}
