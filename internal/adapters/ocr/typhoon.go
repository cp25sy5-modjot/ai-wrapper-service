package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

type RateLimitError struct{ Msg string }
func (e RateLimitError) Error() string { return e.Msg }

type ServerError struct{ Msg string }
func (e ServerError) Error() string { return e.Msg }

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
		apiKey:  apiKey,
		baseURL: "https://api.opentyphoon.ai/v1/ocr",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		defaultOcr: OcrParams{
			Model:             "typhoon-ocr",
			TaskType:          "default",
			MaxTokens:         16000,
			Temperature:       0.1,
			TopP:              0.6,
			RepetitionPenalty: 1.2,
		},
	}
}

func (t *TyphoonOCR) ExtractText(ctx context.Context, img []byte) (string, error) {

	backoffs := []time.Duration{
		2 * time.Second,
		5 * time.Second,
		30 * time.Second,
	}

	for attempt := 0; attempt <= len(backoffs); attempt++ {

		body, writer, err := buildMultipartRequest(img, t.defaultOcr)
		if err != nil {
			return "", err
		}

		raw, err := t.sendOcrRequest(ctx, body, writer)
		if err == nil {
			return parseOcrResponse(raw)
		}

		if attempt == len(backoffs) {
			return "", err
		}

		switch err.(type) {
		case RateLimitError, ServerError:
			wait := backoffs[attempt]
			log.Printf("OCR retry %d in %v", attempt+1, wait)

			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return "", ctx.Err()
			}

		default:
			return "", err
		}
	}

	return "", errors.New("unreachable")
}

func buildMultipartRequest(image []byte, params OcrParams) (*bytes.Buffer, *multipart.Writer, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, nil, err
	}

	if _, err = io.Copy(part, bytes.NewReader(image)); err != nil {
		return nil, nil, err
	}

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

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {

		switch resp.StatusCode {
		case 429:
			return nil, RateLimitError{string(raw)}

		case 500, 502, 503, 504:
			return nil, ServerError{string(raw)}

		default:
			return nil, fmt.Errorf("OCR API error: %d - %s", resp.StatusCode, string(raw))
		}
	}

	return raw, nil
}

func parseOcrResponse(raw []byte) (string, error) {
	var resp OcrResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal OCR response: %w", err)
	}

	return resp.ExtractText(), nil
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
