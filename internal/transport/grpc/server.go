package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	pb "homecloud-file-service/internal/transport/grpc/protos"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	// Валидация входных данных
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Формируем путь к директории пользователя
	userDirPath := filepath.Join(s.config.Storage.BasePath, s.config.Storage.UserDirName, req.UserId)

	// Создаем директорию пользователя
	if err := os.MkdirAll(userDirPath, 0755); err != nil {
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
			return &pb.CreateUserDirectoryResponse{
				Success:       false,
				Message:       fmt.Sprintf("Failed to create subdirectory %s: %v", dir, err),
				DirectoryPath: "",
			}, nil
		}
	}

	return &pb.CreateUserDirectoryResponse{
		Success:       true,
		Message:       "User directory created successfully",
		DirectoryPath: userDirPath,
	}, nil
}
