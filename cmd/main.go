package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/1abobik1/upload_file_service/internal/config"
	"github.com/1abobik1/upload_file_service/internal/handler"
	"github.com/1abobik1/upload_file_service/internal/grpc/server"
	"github.com/1abobik1/upload_file_service/internal/service"
	"github.com/1abobik1/upload_file_service/internal/storage"
	"github.com/sirupsen/logrus"
)

func main() {
	// загрузка конфигурации
	cfg := config.MustLoad()

	minioStorage, err := storage.NewMinIOStorage(
		cfg.MinIO.Endpoint,
		cfg.MinIO.MinIoRootUser,
		cfg.MinIO.MinIoRootPassword,
		cfg.MinIO.Bucket,
		cfg.MinIO.UseSSL,
	)
	if err != nil {
		logrus.Fatal(err)
	}

	fileService := service.NewFileService(minioStorage, cfg.MinIO.Bucket)

	fileHandler := handler.NewFileHandler(fileService)

	srv := server.New(server.Config{
		Port:                 cfg.GRPC.Port,
		MaxConcurrentStreams: cfg.GRPC.MaxConcurrentStreams,
		ShutdownTimeout:      cfg.GRPC.ShutdownTimeout,
	}, fileHandler, cfg.GRPC.FileOpsConcurrencyLimit, cfg.GRPC.ListConcurrencyLimit)

	logrus.Infof("cfg.GRPC.FileOpsConcurrencyLimit: %v,  cfg.GRPC.ListConcurrencyLimit: %v", cfg.GRPC.FileOpsConcurrencyLimit, cfg.GRPC.ListConcurrencyLimit)
	go func() {
		if err := srv.Start(); err != nil {
			logrus.Fatalf("Failed to start server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	srv.GracefulStop()
}
