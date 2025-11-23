package ports

import "github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"

type OllamaPort interface {
	ParseOcrResponseToJson(text string, categories []string) (*domain.Transaction, error)
}
