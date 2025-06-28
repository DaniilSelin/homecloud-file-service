package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type storageRepository struct {
	cfg      *config.Config
	basePath string
	userPath string
	// Сессии для возобновляемых загрузок
	sessions     map[string]*uploadSession
	sessionMutex sync.RWMutex
}

type uploadSession struct {
	FilePath  string
	CountByte int64
	SHA256    string
	TimeOut   int64
}

func NewStorageRepository(cfg *config.Config) (interfaces.StorageRepository, error) {
	// Создаем базовую директорию, если её нет
	if err := os.MkdirAll(cfg.Storage.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base storage directory: %w", err)
	}

	// Создаем директорию для пользователей
	userPath := filepath.Join(cfg.Storage.BasePath, cfg.Storage.UserDirName)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create user directory: %w", err)
	}

	// Создаем временную директорию
	if err := os.MkdirAll(cfg.Storage.TempPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &storageRepository{
		cfg:      cfg,
		basePath: cfg.Storage.BasePath,
		userPath: userPath,
		sessions: make(map[string]*uploadSession),
	}, nil
}

// getHomeDirHandle возвращает абсолютный путь к директории хранения файлов пользователей
func (r *storageRepository) getHomeDirHandle() string {
	return r.userPath
}

// validateFilePath проверяет, что путь файла корректен и не выходит за пределы разрешенной директории
func (r *storageRepository) validateFilePath(filePath string) (string, error) {
	// Объединяем корневую директорию и путь к файлу
	uploadFilePath := filepath.Join(r.userPath, filePath)

	// Нормализуем путь
	uploadFilePath = filepath.Clean(uploadFilePath)

	// Проверяем, не пытается ли пользователь выйти за пределы разрешенной директории
	if !filepath.HasPrefix(uploadFilePath, r.userPath) {
		return "", fmt.Errorf("invalid file path: path traversal attempt")
	}

	return uploadFilePath, nil
}

// createUserDirectory создает директорию для пользователя, если её нет
func (r *storageRepository) createUserDirectory(userID uuid.UUID) error {
	userDir := filepath.Join(r.userPath, userID.String())
	return os.MkdirAll(userDir, 0755)
}

// Операции с файлами в хранилище
func (r *storageRepository) SaveFile(ctx context.Context, path string, content []byte) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "SaveFile (repo) called",
		zap.String("path", path),
		zap.Int("contentSize", len(content)))

	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		lg.Error(ctx, "Path validation failed",
			zap.Error(err),
			zap.String("path", path))
		return fmt.Errorf("path validation failed: %w", err)
	}

	lg.Debug(ctx, "Path validated successfully",
		zap.String("originalPath", path),
		zap.String("validPath", validPath))

	// Создаем директорию, если она не существует
	dir := filepath.Dir(validPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		lg.Error(ctx, "Failed to create directory",
			zap.Error(err),
			zap.String("directory", dir))
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Создаем и открываем файл для записи
	file, err := os.Create(validPath)
	if err != nil {
		lg.Error(ctx, "Failed to create file",
			zap.Error(err),
			zap.String("path", validPath))
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Записываем содержимое
	bytesWritten, err := file.Write(content)
	if err != nil {
		lg.Error(ctx, "Failed to write file content",
			zap.Error(err),
			zap.String("path", validPath),
			zap.Int("expectedSize", len(content)))
		return fmt.Errorf("failed to write file: %w", err)
	}

	lg.Info(ctx, "File saved successfully",
		zap.String("path", validPath),
		zap.Int("bytesWritten", bytesWritten),
		zap.Int("contentSize", len(content)))

	return nil
}

func (r *storageRepository) GetFile(ctx context.Context, path string) ([]byte, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFile (repo) called", zap.String("path", path))

	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		lg.Error(ctx, "Path validation failed",
			zap.Error(err),
			zap.String("path", path))
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Проверка существования файла
	fileInfo, err := os.Stat(validPath)
	if os.IsNotExist(err) {
		lg.Info(ctx, "File not found", zap.String("path", validPath))
		return nil, fmt.Errorf("file not found: %s", path)
	} else if err != nil {
		lg.Error(ctx, "Failed to access file",
			zap.Error(err),
			zap.String("path", validPath))
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	// Проверяем, что это файл, а не директория
	if fileInfo.IsDir() {
		lg.Error(ctx, "Path is a directory, not a file",
			zap.String("path", validPath),
			zap.Int64("size", fileInfo.Size()))
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	lg.Debug(ctx, "File info retrieved",
		zap.String("path", validPath),
		zap.Int64("size", fileInfo.Size()),
		zap.Time("modTime", fileInfo.ModTime()))

	// Читаем файл
	content, err := os.ReadFile(validPath)
	if err != nil {
		lg.Error(ctx, "Failed to read file",
			zap.Error(err),
			zap.String("path", validPath))
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lg.Info(ctx, "File retrieved successfully",
		zap.String("path", validPath),
		zap.Int("contentSize", len(content)))

	return content, nil
}

func (r *storageRepository) DeleteFile(ctx context.Context, path string) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteFile (repo) called", zap.String("path", path))

	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		lg.Error(ctx, "Path validation failed",
			zap.Error(err),
			zap.String("path", path))
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Проверяем существование файла перед удалением
	if _, err := os.Stat(validPath); os.IsNotExist(err) {
		lg.Info(ctx, "File not found for deletion", zap.String("path", validPath))
		return fmt.Errorf("file not found: %s", path)
	}

	// Удаляем файл
	if err := os.Remove(validPath); err != nil {
		lg.Error(ctx, "Failed to delete file",
			zap.Error(err),
			zap.String("path", validPath))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	lg.Info(ctx, "File deleted successfully", zap.String("path", validPath))
	return nil
}

func (r *storageRepository) MoveFile(ctx context.Context, oldPath, newPath string) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "MoveFile (repo) called",
		zap.String("oldPath", oldPath),
		zap.String("newPath", newPath))

	// Валидация путей
	validOldPath, err := r.validateFilePath(oldPath)
	if err != nil {
		lg.Error(ctx, "Old path validation failed",
			zap.Error(err),
			zap.String("oldPath", oldPath))
		return fmt.Errorf("old path validation failed: %w", err)
	}

	validNewPath, err := r.validateFilePath(newPath)
	if err != nil {
		lg.Error(ctx, "New path validation failed",
			zap.Error(err),
			zap.String("newPath", newPath))
		return fmt.Errorf("new path validation failed: %w", err)
	}

	// Проверяем существование исходного файла
	if _, err := os.Stat(validOldPath); os.IsNotExist(err) {
		lg.Error(ctx, "Source file not found", zap.String("oldPath", validOldPath))
		return fmt.Errorf("source file not found: %s", oldPath)
	}

	// Создаем директорию для нового пути
	newDir := filepath.Dir(validNewPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		lg.Error(ctx, "Failed to create directory",
			zap.Error(err),
			zap.String("directory", newDir))
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Перемещаем файл
	if err := os.Rename(validOldPath, validNewPath); err != nil {
		lg.Error(ctx, "Failed to move file",
			zap.Error(err),
			zap.String("oldPath", validOldPath),
			zap.String("newPath", validNewPath))
		return fmt.Errorf("failed to move file: %w", err)
	}

	lg.Info(ctx, "File moved successfully",
		zap.String("oldPath", validOldPath),
		zap.String("newPath", validNewPath))

	return nil
}

func (r *storageRepository) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CopyFile (repo) called",
		zap.String("srcPath", srcPath),
		zap.String("dstPath", dstPath))

	// Валидация путей
	validSrcPath, err := r.validateFilePath(srcPath)
	if err != nil {
		lg.Error(ctx, "Source path validation failed",
			zap.Error(err),
			zap.String("srcPath", srcPath))
		return fmt.Errorf("source path validation failed: %w", err)
	}

	validDstPath, err := r.validateFilePath(dstPath)
	if err != nil {
		lg.Error(ctx, "Destination path validation failed",
			zap.Error(err),
			zap.String("dstPath", dstPath))
		return fmt.Errorf("destination path validation failed: %w", err)
	}

	// Проверяем существование исходного файла
	srcInfo, err := os.Stat(validSrcPath)
	if os.IsNotExist(err) {
		lg.Error(ctx, "Source file not found", zap.String("srcPath", validSrcPath))
		return fmt.Errorf("source file not found: %s", srcPath)
	} else if err != nil {
		lg.Error(ctx, "Failed to access source file",
			zap.Error(err),
			zap.String("srcPath", validSrcPath))
		return fmt.Errorf("failed to access source file: %w", err)
	}

	lg.Debug(ctx, "Source file info",
		zap.String("srcPath", validSrcPath),
		zap.Int64("size", srcInfo.Size()),
		zap.Time("modTime", srcInfo.ModTime()))

	// Создаем директорию для нового пути
	dstDir := filepath.Dir(validDstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		lg.Error(ctx, "Failed to create destination directory",
			zap.Error(err),
			zap.String("directory", dstDir))
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Открываем исходный файл
	srcFile, err := os.Open(validSrcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Создаем целевой файл
	dstFile, err := os.Create(validDstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Копируем содержимое
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Операции с директориями
func (r *storageRepository) CreateDirectory(ctx context.Context, path string) error {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Создаем директорию
	if err := os.MkdirAll(validPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (r *storageRepository) DeleteDirectory(ctx context.Context, path string) error {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Удаляем директорию
	if err := os.RemoveAll(validPath); err != nil {
		return fmt.Errorf("failed to delete directory: %w", err)
	}

	return nil
}

func (r *storageRepository) ListDirectory(ctx context.Context, path string) ([]string, error) {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Читаем содержимое директории
	entries, err := os.ReadDir(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}

	return files, nil
}

// Информация о файлах
func (r *storageRepository) GetFileInfo(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Получаем информацию о файле
	fileInfo, err := os.Stat(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Вычисляем контрольные суммы только для файлов
	var md5Checksum, sha256Checksum string
	if !fileInfo.IsDir() {
		md5Checksum, err = r.calculateChecksum(validPath, "md5")
		if err != nil {
			return nil, fmt.Errorf("failed to calculate MD5: %w", err)
		}

		sha256Checksum, err = r.calculateChecksum(validPath, "sha256")
		if err != nil {
			return nil, fmt.Errorf("failed to calculate SHA256: %w", err)
		}
	}

	return &interfaces.FileInfo{
		Path:           path,
		Size:           fileInfo.Size(),
		IsDirectory:    fileInfo.IsDir(),
		ModifiedAt:     fileInfo.ModTime().Unix(),
		MD5Checksum:    md5Checksum,
		SHA256Checksum: sha256Checksum,
	}, nil
}

func (r *storageRepository) GetDirectorySize(ctx context.Context, path string) (int64, error) {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return 0, fmt.Errorf("path validation failed: %w", err)
	}

	var totalSize int64
	err = filepath.Walk(validPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %w", err)
	}

	return totalSize, nil
}

// Проверка целостности
func (r *storageRepository) CalculateChecksum(ctx context.Context, path string, algorithm string) (string, error) {
	return r.calculateChecksum(path, algorithm)
}

func (r *storageRepository) VerifyChecksum(ctx context.Context, path string, expectedChecksum string, algorithm string) (bool, error) {
	actualChecksum, err := r.calculateChecksum(path, algorithm)
	if err != nil {
		return false, err
	}

	return actualChecksum == expectedChecksum, nil
}

// Вспомогательные методы
func (r *storageRepository) calculateChecksum(path string, algorithm string) (string, error) {
	// Валидация пути
	validPath, err := r.validateFilePath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Открываем файл
	file, err := os.Open(validPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Вычисляем контрольную сумму
	switch algorithm {
	case "sha256":
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to calculate SHA256: %w", err)
		}
		return fmt.Sprintf("%x", hasher.Sum(nil)), nil
	case "md5":
		// Для MD5 можно использовать crypto/md5
		// Но пока возвращаем SHA256 как fallback
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", err)
		}
		return fmt.Sprintf("%x", hasher.Sum(nil)), nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// Методы для работы с сессиями загрузки
func (r *storageRepository) saveUploadSession(sessionID, filePath, sha256 string, size int64) {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()

	r.sessions[sessionID] = &uploadSession{
		FilePath:  filePath,
		CountByte: size,
		SHA256:    sha256,
	}
}

func (r *storageRepository) getUploadSession(sessionID string) (*uploadSession, error) {
	r.sessionMutex.RLock()
	defer r.sessionMutex.RUnlock()

	session, found := r.sessions[sessionID]
	if !found {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

func (r *storageRepository) deleteUploadSession(sessionID string) {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()

	delete(r.sessions, sessionID)
}
