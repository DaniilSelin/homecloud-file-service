package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/auth"
	"homecloud-file-service/internal/logger"
	"homecloud-file-service/internal/repository"
	"homecloud-file-service/internal/service"
	grpcserver "homecloud-file-service/internal/transport/grpc"
	pb "homecloud-file-service/internal/transport/grpc/protos"
	"homecloud-file-service/internal/transport/http/api"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("config/config.local.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализируем логгер
	logger, err := logger.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	logger.Info(context.Background(), "Starting HomeCloud File Service...")

	// Инициализируем gRPC клиент для аутентификации
	authClient, err := auth.NewGRPCAuthClient(cfg)
	if err != nil {
		logger.Error(context.Background(), "Failed to create auth client", zap.Error(err))
		log.Fatalf("Failed to create auth client: %v", err)
	}
	defer authClient.Close()

	// Инициализируем репозитории
	fileRepo, err := repository.NewFileRepository(cfg)
	if err != nil {
		logger.Error(context.Background(), "Failed to create file repository", zap.Error(err))
		log.Fatalf("Failed to create file repository: %v", err)
	}

	storageRepo, err := repository.NewStorageRepository(cfg)
	if err != nil {
		logger.Error(context.Background(), "Failed to create storage repository", zap.Error(err))
		log.Fatalf("Failed to create storage repository: %v", err)
	}

	// Инициализируем сервисы
	fileService := service.NewFileService(fileRepo, storageRepo, cfg)
	storageService := service.NewStorageService(storageRepo, cfg)

	// Инициализируем gRPC сервер
	fileGRPCServer := grpcserver.NewFileServiceServer(storageService, cfg)

	// Инициализируем HTTP хэндлеры
	handler := api.NewHandler(fileService, storageService, authClient)

	// Настраиваем маршруты
	router := api.SetupRoutes(handler)

	// Добавляем health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Создаем HTTP сервер
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Создаем gRPC сервер
	grpcListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Grpc.Host, cfg.Grpc.Port))
	if err != nil {
		logger.Error(context.Background(), "Failed to create gRPC listener", zap.Error(err))
		log.Fatalf("Failed to create gRPC listener: %v", err)
	}
	defer grpcListener.Close()

	grpcGRPCServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcGRPCServer, fileGRPCServer)

	// Запускаем HTTP сервер в горутине
	go func() {
		logger.Info(context.Background(), "Starting HTTP server", zap.String("address", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(context.Background(), "Failed to start HTTP server", zap.Error(err))
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Запускаем gRPC сервер в горутине
	go func() {
		logger.Info(context.Background(), "Starting gRPC server", zap.String("address", grpcListener.Addr().String()))
		if err := grpcGRPCServer.Serve(grpcListener); err != nil {
			logger.Error(context.Background(), "Failed to start gRPC server", zap.Error(err))
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Ждем сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(context.Background(), "Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Останавливаем gRPC сервер
	grpcGRPCServer.GracefulStop()

	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error(ctx, "HTTP server forced to shutdown", zap.Error(err))
	}

	logger.Info(context.Background(), "Servers exited")
}
