package grpcserver

import (
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	addr   string
	lis    net.Listener
	Server *grpc.Server
}

func New(addr string) *Server {
	s := grpc.NewServer()
	hs := health.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	return &Server{
		addr:   addr,
		Server: s, // add creds/interceptors here later
	}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.lis = lis
	return s.Server.Serve(lis)
}

func (s *Server) Stop() {
	s.Server.GracefulStop()
	if s.lis != nil {
		_ = s.lis.Close()
	}
}
