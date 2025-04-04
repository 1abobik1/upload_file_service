package server

import (
	"context"
	"net"
	"time"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	Port                 string
	MaxConcurrentStreams int
	ShutdownTimeout      time.Duration
}

type Server struct {
	grpcServer *grpc.Server
	config     Config
}

func New(config Config,fileService pb.FileServiceServer, fileOpsLimit, listOpsLimit int) *Server {
	limiter := newConcurrencyLimiter(fileOpsLimit, listOpsLimit)
	
	s := &Server{
		config: config,
	}

	s.grpcServer = grpc.NewServer(
		grpc.MaxConcurrentStreams(uint32(config.MaxConcurrentStreams)),
		grpc.UnaryInterceptor(limiter.unaryInterceptor),
		grpc.StreamInterceptor(limiter.streamInterceptor),
	)

	pb.RegisterFileServiceServer(s.grpcServer, fileService)
	reflection.Register(s.grpcServer)

	return s
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.Port)
	if err != nil {
		return err
	}

	logrus.Infof("Starting gRPC server on port %s", s.config.Port)
	return s.grpcServer.Serve(listener)
}

func (s *Server) GracefulStop() {
	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.Stop()
		logrus.Warn("Server forced to stop")
	case <-stopped:
		logrus.Info("Server gracefully stopped")
	}
}
