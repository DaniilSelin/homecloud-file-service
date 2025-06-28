package grpc

import (
	"context"
	"fmt"
	"path/filepath"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"
	"homecloud-file-service/internal/models"
	pb "homecloud-file-service/internal/transport/grpc/protos"

	"github.com/google/uuid"
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
	fileService    interfaces.FileService
	config         *config.Config
}

// NewFileServiceServer создает новый экземпляр gRPC сервера
func NewFileServiceServer(storageService interfaces.StorageService, fileService interfaces.FileService, cfg *config.Config) *FileServiceServer {
	return &FileServiceServer{
		storageService: storageService,
		fileService:    fileService,
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

	// Парсим userID
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		lg.Error(ctx, "Invalid user_id format", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id format")
	}

	// Формируем путь к директории пользователя
	userDirPath := filepath.Join(s.config.Storage.BasePath, s.config.Storage.UserDirName, req.UserId)

	// Создаем стандартные поддиректории и записи в БД
	standardDirs := []string{"documents", "photos", "videos", "music", "downloads"}
	
	for _, dirName := range standardDirs {
		// Создаем запись в базе данных через FileService
		createReq := &models.CreateFileRequest{
			Name:     dirName,
			ParentID: nil, // Корневая папка пользователя
			IsFolder: true,
			MimeType: "application/x-directory",
			Size:     0,
		}

		_, err := s.fileService.CreateFile(ctx, createReq, userID)
		if err != nil {
			lg.Error(ctx, "Failed to create folder record in database", 
				zap.String("dir", dirName), 
				zap.Error(err))
			return &pb.CreateUserDirectoryResponse{
				Success:       false,
				Message:       fmt.Sprintf("Failed to create folder record in database for %s: %v", dirName, err),
				DirectoryPath: "",
			}, nil
		}

		lg.Info(ctx, "Created folder record in database", zap.String("dir", dirName))
	}

	lg.Info(ctx, "User directory and database records created successfully", 
		zap.String("userID", req.UserId), 
		zap.String("path", userDirPath))
	
	return &pb.CreateUserDirectoryResponse{
		Success:       true,
		Message:       "User directory and database records created successfully",
		DirectoryPath: userDirPath,
	}, nil
}
