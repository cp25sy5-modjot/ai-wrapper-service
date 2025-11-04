package ocr

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

type TyphoonOCR struct{}

func NewTyphoonOCR() *TyphoonOCR { return &TyphoonOCR{} }

// Validates image bytes (PNG/JPEG). Returns placeholder text.
func (n *TyphoonOCR) ExtractText(ctx context.Context, img []byte) (string, error) {
	// Validate it's a decodable image
	r := bytes.NewReader(img)
	if _, _, err := image.Decode(r); err != nil {
		return "", err
	}
	// TODO: integrate real OCR (Tesseract/OpenCV/Cloud Vision) here
	return "EXTRACTED_TEXT_PLACEHOLDER", nil
}
