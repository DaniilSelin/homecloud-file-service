package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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

func ensureUsersStorageDir() {
	path := "storage/users"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			log.Fatalf("Failed to create users storage directory: %v", err)
		}
		log.Printf("Directory %s created successfully", path)
	} else {
		log.Printf("Directory %s already exists", path)
	}
}

func main() {
	ensureUsersStorageDir()
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	httpServer, logBase, grpcServer, err := run(ctx, os.Stdout, os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	<-ctx.Done()
	logBase.Info(ctx, "Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logBase.Error(ctx, "HTTP server shutdown failed", zap.Error(err))
	}

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	logBase.Info(ctx, "Servers exited gracefully")
}

func run(ctx context.Context, w io.Writer, args []string) (*http.Server, *logger.Logger, *grpc.Server, error) {
	cfg, err := config.LoadConfig("config/config.local.yaml")
	if err != nil {
		return nil, nil, nil, err
	}
	logBase, err := logger.New(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx = logger.CtxWWithLogger(ctx, logBase)

	logBase.Info(ctx, "Config loaded successfully")

	// Инициализируем gRPC клиент для аутентификации
	authClient, err := auth.NewGRPCAuthClient(cfg)
	if err != nil {
		logBase.Error(ctx, "Failed to create auth client", zap.Error(err))
		return nil, nil, nil, err
	}
	defer authClient.Close()
	logBase.Info(ctx, "Auth client initialized successfully")

	// Инициализируем репозитории
	fileRepo, err := repository.NewFileRepository(cfg)
	if err != nil {
		logBase.Error(ctx, "Failed to create file repository", zap.Error(err))
		return nil, nil, nil, err
	}
	logBase.Info(ctx, "File repository initialized successfully")

	storageRepo, err := repository.NewStorageRepository(cfg)
	if err != nil {
		logBase.Error(ctx, "Failed to create storage repository", zap.Error(err))
		return nil, nil, nil, err
	}
	logBase.Info(ctx, "Storage repository initialized successfully")

	// Инициализируем сервисы
	fileService := service.NewFileService(fileRepo, storageRepo, cfg)
	storageService := service.NewStorageService(storageRepo, cfg)
	logBase.Info(ctx, "FileService and StorageService initialized successfully")

	// Инициализируем gRPC сервер
	fileGRPCServer := grpcserver.NewFileServiceServer(storageService, cfg)

	// Инициализируем HTTP хэндлеры
	handler := api.NewHandler(fileService, storageService, authClient)

	// Настраиваем маршруты
	router := api.SetupRoutes(handler, logBase)

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

	logBase.Info(ctx, "Starting HTTP server", zap.String("address", httpServer.Addr))
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logBase.Error(ctx, "Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Создаем gRPC сервер
	grpcListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Grpc.Host, cfg.Grpc.Port))
	if err != nil {
		logBase.Error(ctx, "Failed to create gRPC listener", zap.Error(err))
		return nil, nil, nil, err
	}

	grpcGRPCServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.LoggerInterceptor(logBase)),
	)
	pb.RegisterFileServiceServer(grpcGRPCServer, fileGRPCServer)

	logBase.Info(ctx, "Starting gRPC server", zap.String("address", grpcListener.Addr().String()))
	go func() {
		if err := grpcGRPCServer.Serve(grpcListener); err != nil {
			logBase.Error(ctx, "Failed to start gRPC server", zap.Error(err))
		}
	}()

	return httpServer, logBase, grpcGRPCServer, nil
}
