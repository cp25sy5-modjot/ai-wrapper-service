package ports

import "github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"

type ParserPort interface {
	// Parse a free-form text into a transaction; categories are candidate hints.
	Parse(text string, categories []string) domain.Transaction
}
