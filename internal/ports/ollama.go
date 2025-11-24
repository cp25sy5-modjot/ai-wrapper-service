package ports

import (
	"context"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
)

type OllamaPort interface {
	ParseOcrResponseToJson(ctx context.Context, text string, categories []string) (*domain.Transaction, error)
}
