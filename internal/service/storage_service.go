package service

import (
	"context"
	"fmt"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
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
	return s.storageRepo.SaveFile(ctx, path, content)
}

func (s *storageService) GetFile(ctx context.Context, path string) ([]byte, error) {
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
	// TODO: Реализовать получение доступного места
	return 0, fmt.Errorf("not implemented")
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
	// TODO: Реализовать очистку сиротских файлов
	return fmt.Errorf("not implemented")
}

func (s *storageService) OptimizeStorage(ctx context.Context) error {
	// TODO: Реализовать оптимизацию хранилища
	return fmt.Errorf("not implemented")
}
