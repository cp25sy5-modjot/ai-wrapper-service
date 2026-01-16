package grpc

import (
	aiwpb "github.com/cp25sy5-modjot/proto/gen/ai/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RegisterAIWrapperServer registers the AI service and reflection.
func RegisterAIWrapperServer(s *grpc.Server, impl aiwpb.AiWrapperServiceServer) {
	aiwpb.RegisterAiWrapperServiceServer(s, impl)
	reflection.Register(s)
}
