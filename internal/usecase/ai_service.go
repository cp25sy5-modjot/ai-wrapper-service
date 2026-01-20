package usecase

import (
	"context"
	"log"
	"strings"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/ports"
	aiwpb "github.com/cp25sy5-modjot/proto/gen/ai/v2"
)

type AIService struct {
	aiwpb.UnimplementedAiWrapperServiceServer // satisfies gRPC interface
	ocr                                       ports.OCRPort
	ollama                                    ports.OllamaPort
}

func NewAIService(ocr ports.OCRPort, ollama ports.OllamaPort) *AIService {
	return &AIService{ocr: ocr, ollama: ollama}
}

// ===== gRPC Methods =====

func (s *AIService) Check(ctx context.Context, req *aiwpb.HealthCheckRequest) (*aiwpb.HealthCheckResponse, error) {
	log.Printf("Health check requested: %v", req.GetName())
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
	log.Printf("ExtractTextFromImage called")
	if len(req.GetImageData()) == 0 {
		return nil, invalidArg("image_data is empty")
	}
	txt, err := s.ocr.ExtractText(ctx, req.GetImageData())
	if err != nil {
		return nil, invalidArg("image_data decode failed: " + err.Error())
	}
	return &aiwpb.ExtractTextResponse{ExtractedText: txt}, nil
}

func (s *AIService) BuildTransactionFromText(ctx context.Context, req *aiwpb.BuildTransactionFromTextRequest) (*aiwpb.TransactionResponseV2, error) {
	log.Printf("BuildTransactionFromText called")
	text := strings.TrimSpace(req.GetTextToAnalyze())
	if text == "" {
		return nil, invalidArg("text_to_analyze is empty")
	}
	//to be change to call ollama instead of parser
	tr, err := s.ollama.ParseOcrResponseToJson(ctx, text, req.GetCategories())
	if err != nil {
		return nil, invalidArg("failed to parse text: " + err.Error())
	}
	return toPB(tr), nil
}

func (s *AIService) BuildTransactionFromImage(ctx context.Context, req *aiwpb.BuildTransactionFromImageRequest) (*aiwpb.TransactionResponseV2, error) {
	log.Printf("BuildTransactionFromImage called")
	if len(req.GetImageData()) == 0 {
		return nil, invalidArg("image_data is empty")
	}
	txt, err := s.ocr.ExtractText(ctx, req.GetImageData())
	if err != nil {
		return nil, invalidArg("image_data decode failed: " + err.Error())
	}
	txt = strings.TrimSpace(txt)
	//to be change to call ollama instead of parser
	tr, err := s.ollama.ParseOcrResponseToJson(ctx, txt, req.GetCategories())
	if err != nil {
		return nil, invalidArg("failed to parse text: " + err.Error())
	}
	return toPB(tr), nil
}

// ===== helpers =====

func toPB(t *domain.Transaction) *aiwpb.TransactionResponseV2 {
	return &aiwpb.TransactionResponseV2{
		Title:    t.Title,
		Date:     t.Date,
		Items:    buildTransactionItemsPB(t.Items),
	}
}

func buildTransactionItemsPB(items []domain.TransactionItem) []*aiwpb.TransactionItem {
	var pbItems []*aiwpb.TransactionItem
	for _, item := range items {
		pbItem := &aiwpb.TransactionItem{
			Title:    item.Title,
			Price:    item.Price,
			Category: item.Category,
		}
		pbItems = append(pbItems, pbItem)
	}
	return pbItems
}

func invalidArg(msg string) error {
	// gRPC status code 3 (InvalidArgument)
	return statusError(3, msg)
}

// minimal wrapper to avoid importing status/codes everywhere
func statusError(code int, msg string) error {
	// google.golang.org/grpc/status + codes would be typical; kept minimal here:
	log.Printf("gRPC error %d: %s", code, msg)
	return grpcErr{code: code, msg: msg}
}

type grpcErr struct {
	code int
	msg  string
}

func (e grpcErr) Error() string { return e.msg }
