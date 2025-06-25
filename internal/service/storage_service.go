package service

import (
	"context"
	"fmt"
	"path/filepath"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"

	"go.uber.org/zap"
)

type storageService struct {
	storageRepo interfaces.StorageRepository
	cfg         *config.Config
}

func NewStorageService(storageRepo interfaces.StorageRepository, cfg *config.Config) interfaces.StorageService {
	return &storageService{
		storageRepo: storageRepo,
		cfg:         cfg,
	}
}

// Основные операции
func (s *storageService) SaveFile(ctx context.Context, path string, content []byte) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "SaveFile called", zap.String("path", path))
	return s.storageRepo.SaveFile(ctx, path, content)
}

func (s *storageService) GetFile(ctx context.Context, path string) ([]byte, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFile called", zap.String("path", path))
	return s.storageRepo.GetFile(ctx, path)
}

func (s *storageService) DeleteFile(ctx context.Context, path string) error {
	return s.storageRepo.DeleteFile(ctx, path)
}

func (s *storageService) MoveFile(ctx context.Context, oldPath, newPath string) error {
	return s.storageRepo.MoveFile(ctx, oldPath, newPath)
}

func (s *storageService) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	return s.storageRepo.CopyFile(ctx, srcPath, dstPath)
}

// Операции с директориями
func (s *storageService) CreateDirectory(ctx context.Context, path string) error {
	return s.storageRepo.CreateDirectory(ctx, path)
}

func (s *storageService) DeleteDirectory(ctx context.Context, path string) error {
	return s.storageRepo.DeleteDirectory(ctx, path)
}

func (s *storageService) ListDirectory(ctx context.Context, path string) ([]string, error) {
	return s.storageRepo.ListDirectory(ctx, path)
}

// Информация о файлах
func (s *storageService) GetFileInfo(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	return s.storageRepo.GetFileInfo(ctx, path)
}

func (s *storageService) GetDirectorySize(ctx context.Context, path string) (int64, error) {
	return s.storageRepo.GetDirectorySize(ctx, path)
}

func (s *storageService) GetAvailableSpace(ctx context.Context) (int64, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetAvailableSpace called")

	// Получаем размер корневой директории хранилища
	totalSize, err := s.storageRepo.GetDirectorySize(ctx, s.cfg.Storage.BasePath)
	if err != nil {
		lg.Error(ctx, "Failed to get directory size", zap.Error(err))
		return 0, fmt.Errorf("failed to get directory size: %w", err)
	}

	// Вычисляем доступное место (упрощенная логика)
	// В реальной реализации нужно учитывать общий размер диска
	availableSpace := int64(1024*1024*1024*100) - totalSize // 100GB - использованное место
	if availableSpace < 0 {
		availableSpace = 0
	}

	lg.Info(ctx, "Available space calculated", zap.Int64("availableSpace", availableSpace))
	return availableSpace, nil
}

// Проверка целостности
func (s *storageService) CalculateChecksum(ctx context.Context, path string, algorithm string) (string, error) {
	return s.storageRepo.CalculateChecksum(ctx, path, algorithm)
}

func (s *storageService) VerifyChecksum(ctx context.Context, path string, expectedChecksum string, algorithm string) (bool, error) {
	return s.storageRepo.VerifyChecksum(ctx, path, expectedChecksum, algorithm)
}

// Очистка и обслуживание
func (s *storageService) CleanupOrphanedFiles(ctx context.Context) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CleanupOrphanedFiles called")

	// Получаем список всех файлов в хранилище
	files, err := s.storageRepo.ListDirectory(ctx, s.cfg.Storage.BasePath)
	if err != nil {
		lg.Error(ctx, "Failed to list directory", zap.Error(err))
		return fmt.Errorf("failed to list directory: %w", err)
	}

	// Простая логика очистки - удаляем временные файлы
	cleanedCount := 0
	for _, file := range files {
		if len(file) > 4 && file[len(file)-4:] == ".tmp" {
			err := s.storageRepo.DeleteFile(ctx, filepath.Join(s.cfg.Storage.BasePath, file))
			if err != nil {
				lg.Error(ctx, "Failed to delete orphaned file", zap.String("file", file), zap.Error(err))
			} else {
				cleanedCount++
			}
		}
	}

	lg.Info(ctx, "Cleanup completed", zap.Int("cleanedCount", cleanedCount))
	return nil
}

func (s *storageService) OptimizeStorage(ctx context.Context) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "OptimizeStorage called")

	// Простая логика оптимизации - проверяем целостность файлов
	files, err := s.storageRepo.ListDirectory(ctx, s.cfg.Storage.BasePath)
	if err != nil {
		lg.Error(ctx, "Failed to list directory", zap.Error(err))
		return fmt.Errorf("failed to list directory: %w", err)
	}

	optimizedCount := 0
	for _, file := range files {
		// Проверяем MD5 контрольную сумму
		_, err := s.storageRepo.CalculateChecksum(ctx, filepath.Join(s.cfg.Storage.BasePath, file), "md5")
		if err != nil {
			lg.Error(ctx, "Failed to calculate checksum", zap.String("file", file), zap.Error(err))
		} else {
			optimizedCount++
		}
	}

	lg.Info(ctx, "Storage optimization completed", zap.Int("optimizedCount", optimizedCount))
	return nil
}
