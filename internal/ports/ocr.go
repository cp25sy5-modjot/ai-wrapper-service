package ports

import "context"

type OCRPort interface {
	// Returns extracted text from image bytes (expects valid image formats).
	ExtractText(ctx context.Context, image []byte) (string, error)
}
