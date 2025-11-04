package usecase

import (
	"context"
	"strings"
	"time"

	aiwpb "github.com/cp25sy5-modjot/proto/gen/ai/v1"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/ports"
)

type AIService struct {
	aiwpb.UnimplementedAiWrapperServiceServer // satisfies gRPC interface
	ocr    ports.OCRPort
	parser ports.ParserPort
}

func NewAIService(ocr ports.OCRPort, parser ports.ParserPort) *AIService {
	return &AIService{ocr: ocr, parser: parser}
}

// ===== gRPC Methods =====

func (s *AIService) Check(ctx context.Context, req *aiwpb.HealthCheckRequest) (*aiwpb.HealthCheckResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = "ai-wrapper"
	}
	return &aiwpb.HealthCheckResponse{
		Healthy: true,
		Message: "OK: " + name,
	}, nil
}

func (s *AIService) ExtractTextFromImage(ctx context.Context, req *aiwpb.ExtractTextRequest) (*aiwpb.ExtractTextResponse, error) {
	if len(req.GetImageData()) == 0 {
		return nil, invalidArg("image_data is empty")
	}
	txt, err := s.ocr.ExtractText(ctx, req.GetImageData())
	if err != nil {
		return nil, invalidArg("image_data decode failed: "+err.Error())
	}
	return &aiwpb.ExtractTextResponse{ExtractedText: txt}, nil
}

func (s *AIService) BuildTransactionFromText(ctx context.Context, req *aiwpb.BuildTransactionFromTextRequest) (*aiwpb.TransactionResponse, error) {
	text := strings.TrimSpace(req.GetTextToAnalyze())
	if text == "" {
		return nil, invalidArg("text_to_analyze is empty")
	}
	//to be change to call ollama
	tr := s.parser.Parse(text, req.GetCategories())
	return toPB(tr), nil
}

func (s *AIService) BuildTransactionFromImage(ctx context.Context, req *aiwpb.BuildTransactionFromImageRequest) (*aiwpb.TransactionResponse, error) {
	if len(req.GetImageData()) == 0 {
		return nil, invalidArg("image_data is empty")
	}
	txt, err := s.ocr.ExtractText(ctx, req.GetImageData())
	if err != nil {
		return nil, invalidArg("image_data decode failed: "+err.Error())
	}
	txt = strings.TrimSpace(txt)
	if txt == "" {
		return toPB(domain.Transaction{
			Title:    "Unknown",
			Price:    0,
			Quantity: 1,
			Date:     time.Now().Format("2006-01-02"),
			Category: fallbackCategory(req.GetCategories()),
		}), nil
	}
	tr := s.parser.Parse(txt, req.GetCategories())
	return toPB(tr), nil
}

// ===== helpers =====

func toPB(t domain.Transaction) *aiwpb.TransactionResponse {
	return &aiwpb.TransactionResponse{
		Title:    t.Title,
		Price:    t.Price,
		Quantity: t.Quantity,
		Date:     t.Date,
		Category: t.Category,
	}
}

func fallbackCategory(categories []string) string {
	if len(categories) > 0 && strings.TrimSpace(categories[0]) != "" {
		return categories[0]
	}
	return "Uncategorized"
}

func invalidArg(msg string) error {
	// gRPC status code 3 (InvalidArgument)
	return statusError(3, msg)
}

// minimal wrapper to avoid importing status/codes everywhere
func statusError(code int, msg string) error {
	// google.golang.org/grpc/status + codes would be typical; kept minimal here:
	return grpcErr{code: code, msg: msg}
}

type grpcErr struct{ code int; msg string }
func (e grpcErr) Error() string { return e.msg }
