package usecase

import (
	"context"
	"log"
	"strings"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/ports"
	aiwpb "github.com/cp25sy5-modjot/proto/gen/ai/v2"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AIService struct {
	aiwpb.UnimplementedAiWrapperServiceServer
	ocr    ports.OCRPort
	ollama ports.OllamaPort

	ocrSem chan struct{} // limit OCR concurrency
}

func NewAIService(ocr ports.OCRPort, ollama ports.OllamaPort) *AIService {
	return &AIService{
		ocr:    ocr,
		ollama: ollama,
		ocrSem: make(chan struct{}, 3),
	}
}

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
		return nil, status.Error(codes.InvalidArgument, "image_data is empty")
	}

	txt, err := s.runOCR(ctx, req.GetImageData())
	if err != nil {
		return nil, err
	}

	return &aiwpb.ExtractTextResponse{ExtractedText: txt}, nil
}

func (s *AIService) BuildTransactionFromText(ctx context.Context, req *aiwpb.BuildTransactionFromTextRequest) (*aiwpb.TransactionResponseV2, error) {

	log.Printf("BuildTransactionFromText called")

	text := strings.TrimSpace(req.GetTextToAnalyze())
	if text == "" {
		return nil, status.Error(codes.InvalidArgument, "text_to_analyze is empty")
	}

	tr, err := s.ollama.ParseOcrResponseToJson(ctx, text, req.GetCategories())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse text: %v", err)
	}

	return toPB(tr), nil
}

func (s *AIService) BuildTransactionFromImage(ctx context.Context, req *aiwpb.BuildTransactionFromImageRequest) (*aiwpb.TransactionResponseV2, error) {

	log.Printf("BuildTransactionFromImage called")

	if len(req.GetImageData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "image_data is empty")
	}

	txt, err := s.runOCR(ctx, req.GetImageData())
	if err != nil {
		return nil, err
	}

	txt = strings.TrimSpace(txt)

	tr, err := s.ollama.ParseOcrResponseToJson(ctx, txt, req.GetCategories())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse text: %v", err)
	}

	return toPB(tr), nil
}

func (s *AIService) runOCR(ctx context.Context, img []byte) (string, error) {

	// concurrency limiter
	s.ocrSem <- struct{}{}
	defer func() { <-s.ocrSem }()

	txt, err := s.ocr.ExtractText(ctx, img)
	if err != nil {
		log.Printf("OCR error: %v", err)

		// OCR เป็น external infra → ใช้ Unavailable
		return "", status.Errorf(codes.Unavailable, "ocr failed: %v", err)
	}

	return txt, nil
}

// ===== helpers =====

func toPB(t *domain.Transaction) *aiwpb.TransactionResponseV2 {
	return &aiwpb.TransactionResponseV2{
		Title: t.Title,
		Date:  t.Date,
		Items: buildTransactionItemsPB(t.Items),
	}
}

func buildTransactionItemsPB(items []domain.TransactionItem) []*aiwpb.TransactionItem {
	var pbItems []*aiwpb.TransactionItem

	for _, item := range items {
		pbItems = append(pbItems, &aiwpb.TransactionItem{
			Title:    item.Title,
			Price:    item.Price,
			Category: item.Category,
		})
	}

	return pbItems
}
