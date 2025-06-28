package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/models"

	"homecloud-file-service/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MIME типы по расширениям
var mimeTypes = map[string]string{
	// Текстовые файлы
	".txt":  "text/plain",
	".md":   "text/markdown",
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".json": "application/json",
	".xml":  "application/xml",
	".csv":  "text/csv",
	".log":  "text/plain",

	// Изображения
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".ico":  "image/x-icon",

	// Документы
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

	// Архивы
	".zip": "application/zip",
	".rar": "application/vnd.rar",
	".7z":  "application/x-7z-compressed",
	".tar": "application/x-tar",
	".gz":  "application/gzip",

	// Аудио
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".flac": "audio/flac",
	".aac":  "audio/aac",

	// Видео
	".mp4":  "video/mp4",
	".avi":  "video/x-msvideo",
	".mkv":  "video/x-matroska",
	".mov":  "video/quicktime",
	".wmv":  "video/x-ms-wmv",
	".flv":  "video/x-flv",
	".webm": "video/webm",

	// Код
	".py":   "text/x-python",
	".java": "text/x-java-source",
	".cpp":  "text/x-c++src",
	".c":    "text/x-csrc",
	".go":   "text/x-go",
	".php":  "text/x-php",
	".rb":   "text/x-ruby",
	".sh":   "application/x-sh",
	".bat":  "application/x-msdos-program",

	// Другие
	".sql":  "application/sql",
	".yaml": "application/x-yaml",
	".yml":  "application/x-yaml",
	".toml": "application/toml",
	".ini":  "text/plain",
	".conf": "text/plain",
	".cfg":  "text/plain",
}

// getMimeTypeByExtension определяет MIME тип по расширению файла
func getMimeTypeByExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mimeType, exists := mimeTypes[ext]; exists {
		return mimeType
	}
	return "application/octet-stream"
}

type fileService struct {
	fileRepo    interfaces.FileRepository
	storageRepo interfaces.StorageRepository
	cfg         *config.Config
	// Добавляем map для хранения сессий в памяти
	resumableSessions map[string]*models.ResumableDownloadSession
	sessionMutex      sync.RWMutex
}

func NewFileService(fileRepo interfaces.FileRepository, storageRepo interfaces.StorageRepository, cfg *config.Config) interfaces.FileService {
	return &fileService{
		fileRepo:          fileRepo,
		storageRepo:       storageRepo,
		cfg:               cfg,
		resumableSessions: make(map[string]*models.ResumableDownloadSession),
	}
}

// Основные операции с файлами
func (s *fileService) CreateFile(ctx context.Context, req *models.CreateFileRequest, ownerID uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateFile called", zap.Any("req", req), zap.String("ownerID", ownerID.String()))

	// Определяем MIME тип
	mimeType := req.MimeType
	if mimeType == "" && !req.IsFolder {
		mimeType = getMimeTypeByExtension(req.Name)
	}

	// Создаем объект файла (без ID - он будет сгенерирован БД)
	file := &models.File{
		OwnerID:    ownerID,
		ParentID:   req.ParentID,
		Name:       req.Name,
		MimeType:   mimeType,
		Size:       req.Size,
		IsFolder:   req.IsFolder,
		IsTrashed:  false,
		Starred:    false,
		ViewedByMe: false,
		Version:    1,
	}

	// Если это файл (не папка), добавляем расширение
	if !req.IsFolder && req.Name != "" {
		ext := filepath.Ext(req.Name)
		if ext != "" {
			file.FileExtension = &ext
		}
	}

	// Сохраняем файл в БД и получаем сгенерированный ID
	if err := s.fileRepo.CreateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to create file in database", zap.Error(err))
		return nil, fmt.Errorf("failed to create file in database: %w", err)
	}

	// Теперь у нас есть ID из БД, обновляем storage_path с правильным ID
	relativeStoragePath := ""
	absoluteStoragePath := ""
	if req.ParentID != nil {
		parent, err := s.fileRepo.GetFileByID(ctx, *req.ParentID)
		if err == nil && parent != nil && parent.IsFolder {
			relativeStoragePath = filepath.Join(parent.StoragePath[len(filepath.Join(s.cfg.Storage.BasePath, s.cfg.Storage.UserDirName))+1:], fmt.Sprintf("%s_%s", file.ID.String(), req.Name))
			absoluteStoragePath = filepath.Join(parent.StoragePath, fmt.Sprintf("%s_%s", file.ID.String(), req.Name))
		}
	}
	if relativeStoragePath == "" || absoluteStoragePath == "" {
		relativeStoragePath = filepath.Join(ownerID.String(), fmt.Sprintf("%s_%s", file.ID.String(), req.Name))
		absoluteStoragePath = filepath.Join(s.cfg.Storage.BasePath, s.cfg.Storage.UserDirName, relativeStoragePath)
	}
	file.StoragePath = absoluteStoragePath

	// Обновляем storage_path в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update storage path", zap.Error(err))
		// Не возвращаем ошибку, так как файл уже создан
	}

	// Если это папка, создаем директорию в файловой системе с относительным storage_path
	if req.IsFolder {
		if err := s.storageRepo.CreateDirectory(ctx, relativeStoragePath); err != nil {
			lg.Error(ctx, "Failed to create directory", zap.Error(err))
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		lg.Info(ctx, "Directory created successfully", zap.String("path", relativeStoragePath))
	}

	// Если есть контент, сохраняем его (тоже относительный путь)
	if len(req.Content) > 0 && !req.IsFolder {
		if err := s.storageRepo.SaveFile(ctx, relativeStoragePath, req.Content); err != nil {
			lg.Error(ctx, "Failed to save file content", zap.Error(err))
			return nil, fmt.Errorf("failed to save file content: %w", err)
		}

		// Вычисляем контрольные суммы
		md5Checksum, err := s.storageRepo.CalculateChecksum(ctx, relativeStoragePath, "md5")
		if err != nil {
			lg.Error(ctx, "Failed to calculate MD5 checksum", zap.Error(err))
		} else {
			file.MD5Checksum = &md5Checksum
		}

		sha256Checksum, err := s.storageRepo.CalculateChecksum(ctx, relativeStoragePath, "sha256")
		if err != nil {
			lg.Error(ctx, "Failed to calculate SHA256 checksum", zap.Error(err))
		} else {
			file.SHA256Checksum = &sha256Checksum
		}

		// Обновляем контрольные суммы в БД
		if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
			lg.Error(ctx, "Failed to update checksums", zap.Error(err))
			// Не возвращаем ошибку, так как файл уже создан
		}
	}

	// Создаем права доступа для владельца файла
	ownerPermission := &models.FilePermission{
		ID:          uuid.New(),
		FileID:      file.ID,
		GranteeID:   &ownerID,
		GranteeType: models.GranteeTypeUser,
		Role:        models.RoleOwner,
		AllowShare:  true,
	}

	if err := s.fileRepo.CreatePermission(ctx, ownerPermission); err != nil {
		lg.Error(ctx, "Failed to create owner permission", zap.Error(err))
		// Не возвращаем ошибку, так как файл уже создан
	} else {
		lg.Info(ctx, "Owner permission created successfully", zap.String("fileID", file.ID.String()))
	}

	// Создаем ревизию файла (если это не папка)
	if !req.IsFolder {
		revision := &models.FileRevision{
			ID:          uuid.New(),
			FileID:      file.ID,
			RevisionID:  1,
			Size:        file.Size,
			StoragePath: file.StoragePath,
			UserID:      &ownerID,
		}

		// Копируем MIME тип и контрольные суммы
		if file.MimeType != "" {
			revision.MimeType = &file.MimeType
		}
		if file.MD5Checksum != nil {
			revision.MD5Checksum = file.MD5Checksum
		}

		if err := s.fileRepo.CreateRevision(ctx, revision); err != nil {
			lg.Error(ctx, "Failed to create file revision", zap.Error(err))
			// Не возвращаем ошибку, так как файл уже создан
		} else {
			lg.Info(ctx, "File revision created successfully", zap.String("fileID", file.ID.String()), zap.Int64("revisionID", revision.RevisionID))
		}
	}

	lg.Info(ctx, "File created successfully", zap.String("fileID", file.ID.String()), zap.String("name", req.Name))
	return file, nil
}

func (s *fileService) GetFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil || file == nil {
		lg.Info(ctx, "File not found in DB, trying to find in filesystem", zap.String("fileID", fileID.String()), zap.Error(err))

		// Рекурсивно ищем файл по ID во всех подпапках пользователя
		foundPath, foundName, err := s.findFileRecursively(ctx, userID, fileID)
		if err != nil {
			lg.Error(ctx, "Failed to find file recursively", zap.Error(err))
			return nil, fmt.Errorf("file not found")
		}

		if foundPath == "" {
			return nil, fmt.Errorf("file not found")
		}

		// Получаем инфу о файле из ФС
		info, err := s.storageRepo.GetFileInfo(ctx, foundPath)
		if err != nil {
			lg.Error(ctx, "Failed to get file info from FS", zap.Error(err))
			return nil, fmt.Errorf("file not found")
		}

		// Собираем структуру файла
		parts := strings.SplitN(foundName, "_", 2)
		name := parts[1]
		mimeType := getMimeTypeByExtension(name)
		file = &models.File{
			ID:          fileID,
			OwnerID:     userID,
			Name:        name,
			Size:        info.Size,
			IsFolder:    info.IsDirectory,
			MimeType:    mimeType,
			StoragePath: foundPath,
			CreatedAt:   time.Unix(info.ModifiedAt, 0),
			UpdatedAt:   time.Unix(info.ModifiedAt, 0),
		}

		// Добавляем в БД
		if err := s.fileRepo.CreateFileFromFS(ctx, file); err != nil {
			lg.Error(ctx, "Failed to add file to DB from FS", zap.Error(err))
			// Не возвращаем ошибку, а просто логируем
		}
		lg.Info(ctx, "File restored from FS and added to DB", zap.String("fileID", fileID.String()))
	}

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Обновляем время последнего просмотра
	if err := s.fileRepo.UpdateLastViewed(ctx, fileID); err != nil {
		lg.Error(ctx, "Failed to update last viewed time", zap.Error(err))
		// Не возвращаем ошибку, так как основная операция выполнена успешно
	}

	lg.Info(ctx, "File retrieved successfully", zap.String("fileID", fileID.String()))
	return file, nil
}

func (s *fileService) UpdateFile(ctx context.Context, fileID uuid.UUID, req *models.UpdateFileRequest, userID uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateFile called", zap.String("fileID", fileID.String()), zap.Any("req", req), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Обновляем поля файла
	if req.Name != nil {
		file.Name = *req.Name
		// Обновляем расширение если это файл
		if !file.IsFolder {
			ext := filepath.Ext(*req.Name)
			if ext != "" {
				file.FileExtension = &ext
			} else {
				file.FileExtension = nil
			}
		}
	}

	if req.ParentID != nil {
		file.ParentID = req.ParentID
	}

	if req.Starred != nil {
		file.Starred = *req.Starred
	}

	// Если есть новый контент, обновляем его
	if len(req.Content) > 0 && !file.IsFolder {
		// Сохраняем новый контент
		if err := s.storageRepo.SaveFile(ctx, file.StoragePath, req.Content); err != nil {
			lg.Error(ctx, "Failed to save updated file content", zap.Error(err))
			return nil, fmt.Errorf("failed to save file content: %w", err)
		}

		// Обновляем размер
		file.Size = int64(len(req.Content))

		// Пересчитываем контрольные суммы
		md5Checksum, err := s.storageRepo.CalculateChecksum(ctx, file.StoragePath, "md5")
		if err != nil {
			lg.Error(ctx, "Failed to calculate MD5 checksum", zap.Error(err))
		} else {
			file.MD5Checksum = &md5Checksum
		}

		sha256Checksum, err := s.storageRepo.CalculateChecksum(ctx, file.StoragePath, "sha256")
		if err != nil {
			lg.Error(ctx, "Failed to calculate SHA256 checksum", zap.Error(err))
		} else {
			file.SHA256Checksum = &sha256Checksum
		}

		// Увеличиваем версию
		file.Version++
	}

	// Обновляем файл в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file in database", zap.Error(err))
		return nil, fmt.Errorf("failed to update file: %w", err)
	}

	lg.Info(ctx, "File updated successfully", zap.String("fileID", fileID.String()))
	return file, nil
}

func (s *fileService) DeleteFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	if file == nil {
		lg.Error(ctx, "File not found", zap.String("fileID", fileID.String()))
		return fmt.Errorf("file not found")
	}

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Выполняем мягкое удаление (soft delete)
	if err := s.fileRepo.SoftDeleteFile(ctx, fileID); err != nil {
		lg.Error(ctx, "Failed to soft delete file", zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	lg.Info(ctx, "File deleted successfully", zap.String("fileID", fileID.String()))
	return nil
}

func (s *fileService) DeleteFileRecursive(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteFileRecursive called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	if file == nil {
		lg.Error(ctx, "File not found", zap.String("fileID", fileID.String()))
		return fmt.Errorf("file not found")
	}

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Рекурсивно удаляем файл/папку
	if err := s.deleteFileRecursiveHelper(ctx, file, userID); err != nil {
		lg.Error(ctx, "Failed to delete file recursively", zap.Error(err))
		return fmt.Errorf("failed to delete file recursively: %w", err)
	}

	lg.Info(ctx, "File deleted recursively successfully", zap.String("fileID", fileID.String()))
	return nil
}

// deleteFileRecursiveHelper рекурсивно удаляет файл или папку и все их содержимое
func (s *fileService) deleteFileRecursiveHelper(ctx context.Context, file *models.File, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)

	// Если это папка, сначала удаляем все содержимое
	if file.IsFolder {
		lg.Debug(ctx, "Deleting folder contents", zap.String("folderID", file.ID.String()), zap.String("folderName", file.Name))
		
		// Получаем все файлы в папке
		children, err := s.fileRepo.ListFilesByParent(ctx, file.OwnerID, &file.ID)
		if err != nil {
			lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
			return fmt.Errorf("failed to list folder contents: %w", err)
		}

		// Рекурсивно удаляем каждый файл/папку
		for _, child := range children {
			if err := s.deleteFileRecursiveHelper(ctx, &child, userID); err != nil {
				lg.Error(ctx, "Failed to delete child file", zap.Error(err), zap.String("childID", child.ID.String()))
				return fmt.Errorf("failed to delete child file: %w", err)
			}
		}
	}

	// Удаляем физический файл/папку из хранилища
	relativePath := file.StoragePath
	if strings.HasPrefix(relativePath, s.cfg.Storage.BasePath) {
		relativePath = strings.TrimPrefix(relativePath, s.cfg.Storage.BasePath)
		relativePath = strings.TrimPrefix(relativePath, "/")
		relativePath = strings.TrimPrefix(relativePath, s.cfg.Storage.UserDirName)
		relativePath = strings.TrimPrefix(relativePath, "/")
	}

	if relativePath != "" {
		if file.IsFolder {
			if err := s.storageRepo.DeleteDirectory(ctx, relativePath); err != nil {
				lg.Error(ctx, "Failed to delete directory from storage", zap.Error(err), zap.String("path", relativePath))
				// Не возвращаем ошибку, продолжаем удаление из БД
			} else {
				lg.Debug(ctx, "Directory deleted from storage", zap.String("path", relativePath))
			}
		} else {
			if err := s.storageRepo.DeleteFile(ctx, relativePath); err != nil {
				lg.Error(ctx, "Failed to delete file from storage", zap.Error(err), zap.String("path", relativePath))
				// Не возвращаем ошибку, продолжаем удаление из БД
			} else {
				lg.Debug(ctx, "File deleted from storage", zap.String("path", relativePath))
			}
		}
	}

	// Удаляем запись из БД
	if err := s.fileRepo.DeleteFile(ctx, file.ID); err != nil {
		lg.Error(ctx, "Failed to delete file from database", zap.Error(err))
		return fmt.Errorf("failed to delete file from database: %w", err)
	}

	lg.Debug(ctx, "File deleted from database", zap.String("fileID", file.ID.String()), zap.String("fileName", file.Name))
	return nil
}

func (s *fileService) RestoreFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "RestoreFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Восстанавливаем файл
	if err := s.fileRepo.RestoreFile(ctx, fileID); err != nil {
		lg.Error(ctx, "Failed to restore file", zap.Error(err))
		return fmt.Errorf("failed to restore file: %w", err)
	}

	lg.Info(ctx, "File restored successfully", zap.String("fileID", fileID.String()))
	return nil
}

// Операции с контентом файлов
func (s *fileService) UploadFile(ctx context.Context, fileID uuid.UUID, content io.Reader, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UploadFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Проверяем, что это не папка
	if file.IsFolder {
		lg.Error(ctx, "Cannot upload content to folder", zap.String("fileID", fileID.String()))
		return fmt.Errorf("cannot upload content to folder")
	}

	// Читаем весь контент
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		lg.Error(ctx, "Failed to read content", zap.Error(err))
		return fmt.Errorf("failed to read content: %w", err)
	}

	// Сохраняем контент в хранилище
	if err := s.storageRepo.SaveFile(ctx, file.StoragePath, contentBytes); err != nil {
		lg.Error(ctx, "Failed to save file content", zap.Error(err))
		return fmt.Errorf("failed to save file content: %w", err)
	}

	// Обновляем размер файла
	file.Size = int64(len(contentBytes))

	// Вычисляем контрольные суммы
	md5Checksum, err := s.storageRepo.CalculateChecksum(ctx, file.StoragePath, "md5")
	if err != nil {
		lg.Error(ctx, "Failed to calculate MD5 checksum", zap.Error(err))
	} else {
		file.MD5Checksum = &md5Checksum
	}

	sha256Checksum, err := s.storageRepo.CalculateChecksum(ctx, file.StoragePath, "sha256")
	if err != nil {
		lg.Error(ctx, "Failed to calculate SHA256 checksum", zap.Error(err))
	} else {
		file.SHA256Checksum = &sha256Checksum
	}

	// Увеличиваем версию
	file.Version++

	// Обновляем файл в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file in database", zap.Error(err))
		return fmt.Errorf("failed to update file: %w", err)
	}

	lg.Info(ctx, "File uploaded successfully", zap.String("fileID", fileID.String()), zap.Int64("size", file.Size))
	return nil
}

func (s *fileService) DownloadFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (io.ReadCloser, string, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DownloadFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, "", fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, "", fmt.Errorf("access denied")
	}

	// Проверяем, что это не папка
	if file.IsFolder {
		lg.Error(ctx, "Cannot download folder content", zap.String("fileID", fileID.String()))
		return nil, "", fmt.Errorf("cannot download folder content")
	}

	// Получаем контент из хранилища
	content, err := s.storageRepo.GetFile(ctx, file.StoragePath)
	if err != nil {
		lg.Error(ctx, "Failed to get file content from storage", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get file content: %w", err)
	}

	// Обновляем время последнего просмотра
	if err := s.fileRepo.UpdateLastViewed(ctx, fileID); err != nil {
		lg.Error(ctx, "Failed to update last viewed time", zap.Error(err))
		// Не возвращаем ошибку, так как основная операция выполнена успешно
	}

	lg.Info(ctx, "File downloaded successfully", zap.String("fileID", fileID.String()))
	return io.NopCloser(bytes.NewReader(content)), file.MimeType, nil
}

func (s *fileService) GetFileContent(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]byte, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileContent called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Проверяем, что это не папка
	if file.IsFolder {
		lg.Error(ctx, "Cannot get content of folder", zap.String("fileID", fileID.String()))
		return nil, fmt.Errorf("cannot get content of folder")
	}

	// Получаем контент из хранилища
	content, err := s.storageRepo.GetFile(ctx, file.StoragePath)
	if err != nil {
		lg.Error(ctx, "Failed to get file content from storage", zap.Error(err))
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	// Обновляем время последнего просмотра
	if err := s.fileRepo.UpdateLastViewed(ctx, fileID); err != nil {
		lg.Error(ctx, "Failed to update last viewed time", zap.Error(err))
		// Не возвращаем ошибку, так как основная операция выполнена успешно
	}

	lg.Info(ctx, "File content retrieved successfully", zap.String("fileID", fileID.String()), zap.Int("size", len(content)))
	return content, nil
}

// Операции со списками файлов
func (s *fileService) ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListFiles called", zap.Any("req", req))

	var files []models.File

	// Если указан путь, находим папку по пути и получаем её содержимое
	if req.Path != "" {
		// Получаем файл (папку) по пути
		folder, err := s.GetFileDetails(ctx, req.OwnerID, req.Path)
		if err != nil {
			if err.Error() == "file not found" {
				// Если папка не найдена, возвращаем пустой список
				return &models.FileListResponse{
					Files: []models.File{},
					Total: 0,
				}, nil
			}
			lg.Error(ctx, "Failed to get folder by path", zap.Error(err))
			return nil, fmt.Errorf("failed to get folder by path: %w", err)
		}

		// Проверяем, что это папка
		if !folder.IsFolder {
			lg.Error(ctx, "Specified path is not a folder", zap.String("path", req.Path))
			return nil, fmt.Errorf("specified path is not a folder")
		}

		// Получаем содержимое папки
		files, err = s.ListFolderContents(ctx, &folder.ID, req.OwnerID)
		if err != nil {
			lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
			return nil, fmt.Errorf("failed to list folder contents: %w", err)
		}

		// Применяем фильтры из запроса
		filteredFiles := make([]models.File, 0)
		for _, file := range files {
			if req.IsTrashed != nil && file.IsTrashed != *req.IsTrashed {
				continue
			}
			if req.Starred != nil && file.Starred != *req.Starred {
				continue
			}
			filteredFiles = append(filteredFiles, file)
		}
		files = filteredFiles
	} else {
		// Если путь не указан, используем стандартный метод с parent_id
		response, err := s.fileRepo.ListFiles(ctx, req)
		if err != nil {
			lg.Error(ctx, "Failed to list files from database", zap.Error(err))
			return nil, fmt.Errorf("failed to list files: %w", err)
		}
		files = response.Files
	}

	// Применяем пагинацию
	total := int64(len(files))
	if req.Offset > 0 && req.Offset < len(files) {
		files = files[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(files) {
		files = files[:req.Limit]
	}

	response := &models.FileListResponse{
		Files: files,
		Total: total,
	}

	lg.Info(ctx, "Files listed successfully", zap.Int("count", len(files)), zap.Int64("total", total))
	return response, nil
}

func (s *fileService) ListStarredFiles(ctx context.Context, userID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListStarredFiles called", zap.String("userID", userID.String()))

	// Получаем избранные файлы пользователя
	files, err := s.fileRepo.ListStarredFiles(ctx, userID)
	if err != nil {
		lg.Error(ctx, "Failed to list starred files", zap.Error(err))
		return nil, fmt.Errorf("failed to list starred files: %w", err)
	}

	lg.Info(ctx, "Starred files retrieved successfully", zap.Int("count", len(files)))
	return files, nil
}

func (s *fileService) ListTrashedFiles(ctx context.Context, userID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListTrashedFiles called", zap.String("userID", userID.String()))

	// Получаем удаленные файлы пользователя
	files, err := s.fileRepo.ListTrashedFiles(ctx, userID)
	if err != nil {
		lg.Error(ctx, "Failed to list trashed files", zap.Error(err))
		return nil, fmt.Errorf("failed to list trashed files: %w", err)
	}

	lg.Info(ctx, "Trashed files retrieved successfully", zap.Int("count", len(files)))
	return files, nil
}

func (s *fileService) SearchFiles(ctx context.Context, userID uuid.UUID, query string) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "SearchFiles called", zap.String("userID", userID.String()), zap.String("query", query))

	// Выполняем поиск файлов
	files, err := s.fileRepo.SearchFiles(ctx, userID, query)
	if err != nil {
		lg.Error(ctx, "Failed to search files", zap.Error(err))
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	lg.Info(ctx, "Files search completed successfully", zap.Int("count", len(files)))
	return files, nil
}

// Операции с папками
func (s *fileService) CreateFolder(ctx context.Context, name string, parentID *uuid.UUID, ownerID uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateFolder called", zap.String("name", name), zap.Any("parentID", parentID), zap.String("ownerID", ownerID.String()))

	// Создаем запрос на создание файла для папки
	req := &models.CreateFileRequest{
		Name:     name,
		ParentID: parentID,
		IsFolder: true,
		MimeType: "application/x-directory",
		Size:     0,
	}

	// Используем общий метод CreateFile
	file, err := s.CreateFile(ctx, req, ownerID)
	if err != nil {
		lg.Error(ctx, "Failed to create folder", zap.Error(err))
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	lg.Info(ctx, "Folder created successfully", zap.String("folderID", file.ID.String()), zap.String("name", name))
	return file, nil
}

func (s *fileService) ListFolderContents(ctx context.Context, folderID *uuid.UUID, userID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListFolderContents called", zap.Any("folderID", folderID), zap.String("userID", userID.String()))

	// Если folderID не указан, получаем корневые файлы пользователя
	if folderID == nil {
		// Получаем файлы из корневой папки пользователя
		files, err := s.fileRepo.ListFilesByParent(ctx, userID, nil)
		if err != nil {
			lg.Error(ctx, "Failed to list root files", zap.Error(err))
			return nil, fmt.Errorf("failed to list root files: %w", err)
		}
		return files, nil
	}

	// Проверяем права доступа к папке
	hasAccess, err := s.fileRepo.CheckPermission(ctx, *folderID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied to folder", zap.String("folderID", folderID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied to folder")
	}

	// Получаем содержимое папки
	files, err := s.fileRepo.ListFilesByParent(ctx, userID, folderID)
	if err != nil {
		lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
		return nil, fmt.Errorf("failed to list folder contents: %w", err)
	}

	lg.Info(ctx, "Folder contents listed successfully", zap.String("folderID", folderID.String()), zap.Int("count", len(files)))
	return files, nil
}

func (s *fileService) GetFileTree(ctx context.Context, rootID *uuid.UUID, userID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileTree called", zap.Any("rootID", rootID), zap.String("userID", userID.String()))

	// Получаем дерево файлов
	files, err := s.fileRepo.GetFileTree(ctx, userID, rootID)
	if err != nil {
		lg.Error(ctx, "Failed to get file tree", zap.Error(err))
		return nil, fmt.Errorf("failed to get file tree: %w", err)
	}

	lg.Info(ctx, "File tree retrieved successfully", zap.Int("count", len(files)))
	return files, nil
}

// Операции с ревизиями
func (s *fileService) CreateRevision(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (*models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateRevision called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Получаем файл
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Получаем текущие ревизии для определения номера новой ревизии
	revisions, err := s.fileRepo.GetRevisions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get revisions", zap.Error(err))
		return nil, fmt.Errorf("failed to get revisions: %w", err)
	}

	// Определяем номер новой ревизии
	revisionID := int64(1)
	if len(revisions) > 0 {
		revisionID = revisions[0].RevisionID + 1
	}

	// Создаем новую ревизию
	revision := &models.FileRevision{
		ID:          uuid.New(),
		FileID:      fileID,
		RevisionID:  revisionID,
		Size:        file.Size,
		StoragePath: file.StoragePath,
		UserID:      &userID,
	}

	// Копируем MIME тип если есть
	if file.MimeType != "" {
		revision.MimeType = &file.MimeType
	}

	// Копируем MD5 контрольную сумму если есть
	if file.MD5Checksum != nil {
		revision.MD5Checksum = file.MD5Checksum
	}

	// Сохраняем ревизию
	if err := s.fileRepo.CreateRevision(ctx, revision); err != nil {
		lg.Error(ctx, "Failed to create revision", zap.Error(err))
		return nil, fmt.Errorf("failed to create revision: %w", err)
	}

	lg.Info(ctx, "Revision created successfully", zap.String("revisionID", revision.ID.String()))
	return revision, nil
}

func (s *fileService) ListRevisions(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListRevisions called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Получаем ревизии
	revisions, err := s.fileRepo.GetRevisions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get revisions", zap.Error(err))
		return nil, fmt.Errorf("failed to get revisions: %w", err)
	}

	lg.Info(ctx, "Revisions retrieved successfully", zap.Int("count", len(revisions)))
	return revisions, nil
}

func (s *fileService) GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64, userID uuid.UUID) (*models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetRevision called", zap.String("fileID", fileID.String()), zap.Int64("revisionID", revisionID), zap.String("userID", userID.String()))

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Получаем ревизию
	revision, err := s.fileRepo.GetRevision(ctx, fileID, revisionID)
	if err != nil {
		lg.Error(ctx, "Failed to get revision", zap.Error(err))
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	lg.Info(ctx, "Revision retrieved successfully", zap.String("revisionID", revision.ID.String()))
	return revision, nil
}

func (s *fileService) RestoreRevision(ctx context.Context, fileID uuid.UUID, revisionID int64, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "RestoreRevision called", zap.String("fileID", fileID.String()), zap.Int64("revisionID", revisionID), zap.String("userID", userID.String()))

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Получаем ревизию
	revision, err := s.fileRepo.GetRevision(ctx, fileID, revisionID)
	if err != nil {
		lg.Error(ctx, "Failed to get revision", zap.Error(err))
		return fmt.Errorf("failed to get revision: %w", err)
	}

	// Получаем файл
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Восстанавливаем содержимое файла из ревизии
	if revision.StoragePath != file.StoragePath {
		// Копируем файл из ревизии
		content, err := s.storageRepo.GetFile(ctx, revision.StoragePath)
		if err != nil {
			lg.Error(ctx, "Failed to get revision content", zap.Error(err))
			return fmt.Errorf("failed to get revision content: %w", err)
		}

		// Сохраняем в текущий путь файла
		if err := s.storageRepo.SaveFile(ctx, file.StoragePath, content); err != nil {
			lg.Error(ctx, "Failed to save restored content", zap.Error(err))
			return fmt.Errorf("failed to save restored content: %w", err)
		}
	}

	// Обновляем метаданные файла
	file.Size = revision.Size
	if revision.MD5Checksum != nil {
		file.MD5Checksum = revision.MD5Checksum
	}
	if revision.MimeType != nil {
		file.MimeType = *revision.MimeType
	}

	// Сохраняем обновленный файл
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file", zap.Error(err))
		return fmt.Errorf("failed to update file: %w", err)
	}

	lg.Info(ctx, "Revision restored successfully", zap.String("fileID", fileID.String()), zap.Int64("revisionID", revisionID))
	return nil
}

// Операции с правами доступа
func (s *fileService) GrantPermission(ctx context.Context, fileID uuid.UUID, permission *models.FilePermission, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GrantPermission called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (только владелец может предоставлять права)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleOwner)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Устанавливаем ID файла
	permission.FileID = fileID

	// Сохраняем разрешение
	if err := s.fileRepo.CreatePermission(ctx, permission); err != nil {
		lg.Error(ctx, "Failed to create permission", zap.Error(err))
		return fmt.Errorf("failed to create permission: %w", err)
	}

	lg.Info(ctx, "Permission granted successfully", zap.String("permissionID", permission.ID.String()))
	return nil
}

func (s *fileService) RevokePermission(ctx context.Context, fileID uuid.UUID, granteeID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "RevokePermission called", zap.String("fileID", fileID.String()), zap.String("granteeID", granteeID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (только владелец может отзывать права)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleOwner)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Получаем разрешения для файла
	permissions, err := s.fileRepo.GetPermissions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get permissions", zap.Error(err))
		return fmt.Errorf("failed to get permissions: %w", err)
	}

	// Ищем разрешение для указанного получателя
	var permissionToDelete *models.FilePermission
	for _, perm := range permissions {
		if perm.GranteeID != nil && *perm.GranteeID == granteeID {
			permissionToDelete = &perm
			break
		}
	}

	if permissionToDelete == nil {
		lg.Error(ctx, "Permission not found", zap.String("granteeID", granteeID.String()))
		return fmt.Errorf("permission not found")
	}

	// Удаляем разрешение
	if err := s.fileRepo.DeletePermission(ctx, permissionToDelete.ID); err != nil {
		lg.Error(ctx, "Failed to delete permission", zap.Error(err))
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	lg.Info(ctx, "Permission revoked successfully", zap.String("permissionID", permissionToDelete.ID.String()))
	return nil
}

func (s *fileService) ListPermissions(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]models.FilePermission, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListPermissions called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Получаем разрешения
	permissions, err := s.fileRepo.GetPermissions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get permissions", zap.Error(err))
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	lg.Info(ctx, "Permissions retrieved successfully", zap.Int("count", len(permissions)))
	return permissions, nil
}

func (s *fileService) CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CheckPermission called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()), zap.String("requiredRole", requiredRole))

	// Проверяем права доступа
	hasPermission, err := s.fileRepo.CheckPermission(ctx, fileID, userID, requiredRole)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	lg.Info(ctx, "Permission check completed", zap.Bool("hasPermission", hasPermission))
	return hasPermission, nil
}

// Специальные операции
func (s *fileService) StarFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "StarFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Добавляем в избранное
	file.Starred = true

	// Обновляем файл в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file in database", zap.Error(err))
		return fmt.Errorf("failed to star file: %w", err)
	}

	lg.Info(ctx, "File starred successfully", zap.String("fileID", fileID.String()))
	return nil
}

func (s *fileService) UnstarFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UnstarFile called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Удаляем из избранного
	file.Starred = false

	// Обновляем файл в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file in database", zap.Error(err))
		return fmt.Errorf("failed to unstar file: %w", err)
	}

	lg.Info(ctx, "File unstarred successfully", zap.String("fileID", fileID.String()))
	return nil
}

func (s *fileService) MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "MoveFile called", zap.String("fileID", fileID.String()), zap.Any("newParentID", newParentID), zap.String("userID", userID.String()))

	// Проверяем права доступа к файлу (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission for file", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied to file", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied to file")
	}

	// Если указана новая родительская папка, проверяем права доступа к ней
	if newParentID != nil {
		hasParentAccess, err := s.fileRepo.CheckPermission(ctx, *newParentID, userID, models.RoleWriter)
		if err != nil {
			lg.Error(ctx, "Failed to check permission for parent folder", zap.Error(err))
			return fmt.Errorf("failed to check permission for parent folder: %w", err)
		}

		if !hasParentAccess {
			lg.Error(ctx, "Access denied to parent folder", zap.String("parentID", newParentID.String()), zap.String("userID", userID.String()))
			return fmt.Errorf("access denied to parent folder")
		}
	}

	// Перемещаем файл
	if err := s.fileRepo.MoveFile(ctx, fileID, newParentID); err != nil {
		lg.Error(ctx, "Failed to move file", zap.Error(err))
		return fmt.Errorf("failed to move file: %w", err)
	}

	lg.Info(ctx, "File moved successfully", zap.String("fileID", fileID.String()))
	return nil
}

func (s *fileService) CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string, userID uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CopyFile called", zap.String("fileID", fileID.String()), zap.Any("newParentID", newParentID), zap.String("newName", newName), zap.String("userID", userID.String()))

	// Проверяем права доступа к исходному файлу (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission for source file", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied to source file", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied to source file")
	}

	// Если указана новая родительская папка, проверяем права доступа к ней
	if newParentID != nil {
		hasParentAccess, err := s.fileRepo.CheckPermission(ctx, *newParentID, userID, models.RoleWriter)
		if err != nil {
			lg.Error(ctx, "Failed to check permission for parent folder", zap.Error(err))
			return nil, fmt.Errorf("failed to check permission for parent folder: %w", err)
		}

		if !hasParentAccess {
			lg.Error(ctx, "Access denied to parent folder", zap.String("parentID", newParentID.String()), zap.String("userID", userID.String()))
			return nil, fmt.Errorf("access denied to parent folder")
		}
	}

	// Копируем файл
	copiedFile, err := s.fileRepo.CopyFile(ctx, fileID, newParentID, newName)
	if err != nil {
		lg.Error(ctx, "Failed to copy file", zap.Error(err))
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	lg.Info(ctx, "File copied successfully", zap.String("originalFileID", fileID.String()), zap.String("newFileID", copiedFile.ID.String()))
	return copiedFile, nil
}

func (s *fileService) RenameFile(ctx context.Context, fileID uuid.UUID, newName string, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "RenameFile called", zap.String("fileID", fileID.String()), zap.String("newName", newName), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Обновляем имя файла
	file.Name = newName

	// Обновляем расширение если это файл
	if !file.IsFolder {
		ext := filepath.Ext(newName)
		if ext != "" {
			file.FileExtension = &ext
		} else {
			file.FileExtension = nil
		}
	}

	// Обновляем файл в БД
	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		lg.Error(ctx, "Failed to update file in database", zap.Error(err))
		return fmt.Errorf("failed to rename file: %w", err)
	}

	lg.Info(ctx, "File renamed successfully", zap.String("fileID", fileID.String()), zap.String("newName", newName))
	return nil
}

// Операции с метаданными
func (s *fileService) UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateFileMetadata called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (нужны права на запись)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleWriter)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Обновляем метаданные
	if err := s.fileRepo.UpdateFileMetadata(ctx, fileID, metadata); err != nil {
		lg.Error(ctx, "Failed to update file metadata", zap.Error(err))
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	lg.Info(ctx, "File metadata updated successfully", zap.String("fileID", fileID.String()))
	return nil
}

func (s *fileService) GetFileMetadata(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (map[string]interface{}, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileMetadata called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Получаем метаданные
	metadata, err := s.fileRepo.GetFileMetadata(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file metadata", zap.Error(err))
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	lg.Info(ctx, "File metadata retrieved successfully", zap.String("fileID", fileID.String()))
	return metadata, nil
}

// Проверка целостности
func (s *fileService) VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "VerifyFileIntegrity called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return false, fmt.Errorf("access denied")
	}

	// Проверяем целостность файла
	isIntegrityVerified, err := s.fileRepo.VerifyFileIntegrity(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to verify file integrity", zap.Error(err))
		return false, fmt.Errorf("failed to verify file integrity: %w", err)
	}

	lg.Info(ctx, "File integrity verification completed", zap.Bool("isIntegrityVerified", isIntegrityVerified))
	return isIntegrityVerified, nil
}

func (s *fileService) CalculateFileChecksums(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CalculateFileChecksums called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return fmt.Errorf("access denied")
	}

	// Вычисляем контрольные суммы
	checksums, err := s.fileRepo.CalculateFileChecksums(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to calculate file checksums", zap.Error(err))
		return fmt.Errorf("failed to calculate file checksums: %w", err)
	}

	lg.Info(ctx, "File checksums calculated successfully", zap.Int("checksumCount", len(checksums)))
	return nil
}

// generateStoragePath генерирует путь для хранения файла
func (s *fileService) generateStoragePath(ownerID uuid.UUID, fileID uuid.UUID, fileName string, parentID *uuid.UUID) string {
	if parentID != nil {
		parent, err := s.fileRepo.GetFileByID(context.Background(), *parentID)
		if err == nil && parent != nil && parent.IsFolder {
			return filepath.Join(parent.StoragePath, fmt.Sprintf("%s_%s", fileID.String(), fileName))
		}
	}
	return filepath.Join(s.cfg.Storage.BasePath, s.cfg.Storage.UserDirName, ownerID.String(), fmt.Sprintf("%s_%s", fileID.String(), fileName))
}

// GetFileDetails получает детальную информацию о файле из dbmanager по пути
func (s *fileService) GetFileDetails(ctx context.Context, userID uuid.UUID, filePath string) (*models.File, error) {
	fmt.Printf("fileService.GetFileDetails: called for userID: %s, filePath: %s\n", userID.String(), filePath)
	lg := logger.GetLoggerFromCtxSafe(ctx)
	if lg != nil {
		lg.Info(ctx, "GetFileDetails called", zap.String("userID", userID.String()), zap.String("filePath", filePath))
	}

	// Получаем детальную информацию из dbmanager
	fmt.Printf("fileService.GetFileDetails: calling fileRepo.GetFileByPath...\n")
	file, err := s.fileRepo.GetFileByPath(ctx, userID, filePath)
	if err != nil {
		fmt.Printf("fileService.GetFileDetails: error from fileRepo: %v\n", err)
		if lg != nil {
			lg.Error(ctx, "Failed to get file details from dbmanager", zap.Error(err))
		}
		return nil, fmt.Errorf("failed to get file details: %w", err)
	}

	fmt.Printf("fileService.GetFileDetails: got file details: %+v\n", file)
	if lg != nil {
		lg.Info(ctx, "File details retrieved successfully", zap.String("fileID", file.ID.String()))
	}
	return file, nil
}

// getUserDirPath возвращает путь к директории пользователя
func (s *fileService) getUserDirPath(userID uuid.UUID) string {
	return userID.String()
}

func (s *fileService) findFileRecursively(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (string, string, error) {
	// Начинаем поиск с корневой папки пользователя
	userDir := s.getUserDirPath(userID)
	return s.findFileRecursivelyHelper(ctx, userID, fileID, userDir)
}

func (s *fileService) findFileRecursivelyHelper(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, currentPath string) (string, string, error) {
	lg := logger.GetLoggerFromCtx(ctx)

	// Получаем список файлов в текущей папке
	fileNames, err := s.storageRepo.ListDirectory(ctx, currentPath)
	if err != nil {
		lg.Error(ctx, "Failed to list directory", zap.String("path", currentPath), zap.Error(err))
		return "", "", fmt.Errorf("failed to list directory: %w", err)
	}

	// Сначала проверяем файлы в текущей папке
	for _, fileName := range fileNames {
		parts := strings.SplitN(fileName, "_", 2)
		if len(parts) == 2 && parts[0] == fileID.String() {
			fullPath := filepath.Join(currentPath, fileName)
			lg.Info(ctx, "File found", zap.String("path", fullPath), zap.String("name", fileName))
			return fullPath, fileName, nil
		}
	}

	// Затем рекурсивно ищем в подпапках
	for _, fileName := range fileNames {
		// Проверяем, является ли это папкой (у папок нет расширения или они заканчиваются на /)
		ext := filepath.Ext(fileName)
		if ext == "" || strings.HasSuffix(fileName, "/") {
			subDirPath := filepath.Join(currentPath, fileName)
			foundPath, foundName, err := s.findFileRecursivelyHelper(ctx, userID, fileID, subDirPath)
			if err == nil && foundPath != "" {
				return foundPath, foundName, nil
			}
		}
	}

	return "", "", fmt.Errorf("file not found")
}

// Возобновляемое скачивание
func (s *fileService) InitResumableDownload(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (*models.ResumableDownloadSession, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "InitResumableDownload called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	// Получаем файл из БД
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file from database", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if file == nil {
		lg.Error(ctx, "File not found", zap.String("fileID", fileID.String()))
		return nil, fmt.Errorf("file not found")
	}

	// Проверяем права доступа (нужны права на чтение)
	hasAccess, err := s.fileRepo.CheckPermission(ctx, fileID, userID, models.RoleReader)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		lg.Error(ctx, "Access denied", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))
		return nil, fmt.Errorf("access denied")
	}

	// Проверяем, что это не папка
	if file.IsFolder {
		lg.Error(ctx, "Cannot download folder", zap.String("fileID", fileID.String()))
		return nil, fmt.Errorf("cannot download folder")
	}

	// Вычисляем контрольную сумму файла
	relativePath := file.StoragePath
	if strings.HasPrefix(relativePath, s.cfg.Storage.BasePath) {
		relativePath = strings.TrimPrefix(relativePath, s.cfg.Storage.BasePath)
		relativePath = strings.TrimPrefix(relativePath, "/")
		relativePath = strings.TrimPrefix(relativePath, s.cfg.Storage.UserDirName)
		relativePath = strings.TrimPrefix(relativePath, "/")
	}

	checksum, err := s.storageRepo.CalculateChecksum(ctx, relativePath, "sha256")
	if err != nil {
		lg.Error(ctx, "Failed to calculate checksum", zap.Error(err))
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Генерируем уникальный ID сессии
	sessionID := uuid.New().String()

	// Создаем сессию
	session := &models.ResumableDownloadSession{
		ID:        sessionID,
		FileID:    fileID,
		UserID:    userID,
		FileName:  file.Name,
		FilePath:  relativePath,
		Size:      file.Size,
		Checksum:  checksum,
		MimeType:  file.MimeType,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Сессия действительна 24 часа
		CreatedAt: time.Now(),
	}

	// Сохраняем сессию в памяти (в реальном приложении лучше использовать Redis или БД)
	s.saveResumableDownloadSession(session)

	lg.Info(ctx, "Resumable download session created", zap.String("sessionID", sessionID))
	return session, nil
}

func (s *fileService) GetResumableDownloadSession(ctx context.Context, sessionID string) (*models.ResumableDownloadSession, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetResumableDownloadSession called", zap.String("sessionID", sessionID))

	session := s.getResumableDownloadSession(sessionID)
	if session == nil {
		lg.Error(ctx, "Session not found", zap.String("sessionID", sessionID))
		return nil, fmt.Errorf("session not found")
	}

	// Проверяем, не истекла ли сессия
	if time.Now().After(session.ExpiresAt) {
		lg.Error(ctx, "Session expired", zap.String("sessionID", sessionID))
		s.deleteResumableDownloadSession(sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

func (s *fileService) DownloadFileChunk(ctx context.Context, sessionID string, start, end uint64) (io.ReadCloser, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DownloadFileChunk called", zap.String("sessionID", sessionID), zap.Uint64("start", start), zap.Uint64("end", end))

	session := s.getResumableDownloadSession(sessionID)
	if session == nil {
		lg.Error(ctx, "Session not found", zap.String("sessionID", sessionID))
		return nil, fmt.Errorf("session not found")
	}

	// Проверяем, не истекла ли сессия
	if time.Now().After(session.ExpiresAt) {
		lg.Error(ctx, "Session expired", zap.String("sessionID", sessionID))
		s.deleteResumableDownloadSession(sessionID)
		return nil, fmt.Errorf("session expired")
	}

	// Проверяем валидность диапазона
	if start >= uint64(session.Size) || end >= uint64(session.Size) || start > end {
		lg.Error(ctx, "Invalid range", zap.Uint64("start", start), zap.Uint64("end", end), zap.Int64("fileSize", session.Size))
		return nil, fmt.Errorf("invalid range")
	}

	// Получаем файл из хранилища
	content, err := s.storageRepo.GetFile(ctx, session.FilePath)
	if err != nil {
		lg.Error(ctx, "Failed to get file from storage", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Создаем reader для нужного диапазона
	chunkSize := end - start + 1
	if start+chunkSize > uint64(len(content)) {
		chunkSize = uint64(len(content)) - start
	}

	chunk := content[start : start+chunkSize]
	return io.NopCloser(bytes.NewReader(chunk)), nil
}

func (s *fileService) DeleteResumableDownloadSession(ctx context.Context, sessionID string) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteResumableDownloadSession called", zap.String("sessionID", sessionID))

	s.deleteResumableDownloadSession(sessionID)
	return nil
}

// Вспомогательные методы для работы с сессиями в памяти
func (s *fileService) saveResumableDownloadSession(session *models.ResumableDownloadSession) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.resumableSessions[session.ID] = session
}

func (s *fileService) getResumableDownloadSession(sessionID string) *models.ResumableDownloadSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	return s.resumableSessions[sessionID]
}

func (s *fileService) deleteResumableDownloadSession(sessionID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	delete(s.resumableSessions, sessionID)
}
