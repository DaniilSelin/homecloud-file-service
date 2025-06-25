package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"
	pb "homecloud-file-service/internal/transport/grpc/protos"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggerInterceptor добавляет логгер в контекст gRPC запросов
func LoggerInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Добавляем логгер в контекст
		ctxWithLogger := logger.CtxWWithLogger(ctx, log)

		// Логируем входящий запрос
		log.Info(ctxWithLogger, "gRPC request received",
			zap.String("method", info.FullMethod),
			zap.Any("request", req))

		// Выполняем обработчик с контекстом, содержащим логгер
		resp, err := handler(ctxWithLogger, req)

		// Логируем результат
		if err != nil {
			log.Error(ctxWithLogger, "gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Error(err))
		} else {
			log.Info(ctxWithLogger, "gRPC request completed",
				zap.String("method", info.FullMethod))
		}

		return resp, err
	}
}

// FileServiceServer реализация gRPC сервера для файлового сервиса
type FileServiceServer struct {
	pb.UnimplementedFileServiceServer
	storageService interfaces.StorageService
	config         *config.Config
}

// NewFileServiceServer создает новый экземпляр gRPC сервера
func NewFileServiceServer(storageService interfaces.StorageService, cfg *config.Config) *FileServiceServer {
	return &FileServiceServer{
		storageService: storageService,
		config:         cfg,
	}
}

// CreateUserDirectory создает директорию для пользователя при регистрации
func (s *FileServiceServer) CreateUserDirectory(ctx context.Context, req *pb.CreateUserDirectoryRequest) (*pb.CreateUserDirectoryResponse, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateUserDirectory called", zap.String("userID", req.UserId))
	// Валидация входных данных
	if req.UserId == "" {
		lg.Error(ctx, "user_id is required")
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Формируем путь к директории пользователя
	userDirPath := filepath.Join(s.config.Storage.BasePath, s.config.Storage.UserDirName, req.UserId)

	// Создаем директорию пользователя
	if err := os.MkdirAll(userDirPath, 0755); err != nil {
		lg.Error(ctx, "Failed to create user directory", zap.Error(err))
		return &pb.CreateUserDirectoryResponse{
			Success:       false,
			Message:       fmt.Sprintf("Failed to create user directory: %v", err),
			DirectoryPath: "",
		}, nil
	}

	// Создаем стандартные поддиректории
	standardDirs := []string{"documents", "photos", "videos", "music", "downloads"}
	for _, dir := range standardDirs {
		subDirPath := filepath.Join(userDirPath, dir)
		if err := os.MkdirAll(subDirPath, 0755); err != nil {
			lg.Error(ctx, "Failed to create subdirectory", zap.String("dir", dir), zap.Error(err))
			return &pb.CreateUserDirectoryResponse{
				Success:       false,
				Message:       fmt.Sprintf("Failed to create subdirectory %s: %v", dir, err),
				DirectoryPath: "",
			}, nil
		}
	}
	lg.Info(ctx, "User directory created successfully", zap.String("userID", req.UserId), zap.String("path", userDirPath))
	return &pb.CreateUserDirectoryResponse{
		Success:       true,
		Message:       "User directory created successfully",
		DirectoryPath: userDirPath,
	}, nil
}
