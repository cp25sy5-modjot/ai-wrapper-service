package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/adapters/grpc"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/adapters/ocr"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/adapters/parser"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/pkg/grpcserver"
	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/usecase"
)

func main() {
	addr := env("GRPC_ADDR", ":50051")

	// Adapters (infrastructure)
	ocrAdapter := ocr.NewTyphoonOCR()        // TODO: swap with real OCR later
	parserAdapter := parser.NewRulesParser() // lightweight rules-based parser

	// Application service (use cases)
	aiSvc := usecase.NewAIService(ocrAdapter, parserAdapter)

	// gRPC server (interface adapter)
	s := grpcserver.New(addr)
	grpc.RegisterAIWrapperServer(s.Server, aiSvc)

	// Start
	go func() {
		log.Printf("AI Wrapper gRPC listening on %s", addr)
		if err := s.Start(); err != nil {
			log.Fatalf("gRPC serve error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	log.Println("Shutting down...")
	s.Stop()
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
