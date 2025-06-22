package interfaces

import (
	"context"
	"io"

	"homecloud-file-service/internal/models"

	"github.com/google/uuid"
)

// FileService интерфейс для бизнес-логики работы с файлами
type FileService interface {
	// Основные операции с файлами
	CreateFile(ctx context.Context, req *models.CreateFileRequest, ownerID uuid.UUID) (*models.File, error)
	GetFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (*models.File, error)
	UpdateFile(ctx context.Context, fileID uuid.UUID, req *models.UpdateFileRequest, userID uuid.UUID) (*models.File, error)
	DeleteFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error
	RestoreFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error

	// Операции с контентом файлов
	UploadFile(ctx context.Context, fileID uuid.UUID, content io.Reader, userID uuid.UUID) error
	DownloadFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (io.ReadCloser, string, error)
	GetFileContent(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]byte, error)

	// Операции со списками файлов
	ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error)
	ListStarredFiles(ctx context.Context, userID uuid.UUID) ([]models.File, error)
	ListTrashedFiles(ctx context.Context, userID uuid.UUID) ([]models.File, error)
	SearchFiles(ctx context.Context, userID uuid.UUID, query string) ([]models.File, error)

	// Операции с папками
	CreateFolder(ctx context.Context, name string, parentID *uuid.UUID, ownerID uuid.UUID) (*models.File, error)
	ListFolderContents(ctx context.Context, folderID *uuid.UUID, userID uuid.UUID) ([]models.File, error)
	GetFileTree(ctx context.Context, rootID *uuid.UUID, userID uuid.UUID) ([]models.File, error)

	// Операции с ревизиями
	CreateRevision(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (*models.FileRevision, error)
	ListRevisions(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]models.FileRevision, error)
	GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64, userID uuid.UUID) (*models.FileRevision, error)
	RestoreRevision(ctx context.Context, fileID uuid.UUID, revisionID int64, userID uuid.UUID) error

	// Операции с правами доступа
	GrantPermission(ctx context.Context, fileID uuid.UUID, permission *models.FilePermission, userID uuid.UUID) error
	RevokePermission(ctx context.Context, fileID uuid.UUID, granteeID uuid.UUID, userID uuid.UUID) error
	ListPermissions(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) ([]models.FilePermission, error)
	CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error)

	// Специальные операции
	StarFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error
	UnstarFile(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error
	MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, userID uuid.UUID) error
	CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string, userID uuid.UUID) (*models.File, error)
	RenameFile(ctx context.Context, fileID uuid.UUID, newName string, userID uuid.UUID) error

	// Операции с метаданными
	UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}, userID uuid.UUID) error
	GetFileMetadata(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (map[string]interface{}, error)

	// Проверка целостности
	VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (bool, error)
	CalculateFileChecksums(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) error
}

// StorageService интерфейс для работы с файловым хранилищем
type StorageService interface {
	// Основные операции
	SaveFile(ctx context.Context, path string, content []byte) error
	GetFile(ctx context.Context, path string) ([]byte, error)
	DeleteFile(ctx context.Context, path string) error
	MoveFile(ctx context.Context, oldPath, newPath string) error
	CopyFile(ctx context.Context, srcPath, dstPath string) error

	// Операции с директориями
	CreateDirectory(ctx context.Context, path string) error
	DeleteDirectory(ctx context.Context, path string) error
	ListDirectory(ctx context.Context, path string) ([]string, error)

	// Информация о файлах
	GetFileInfo(ctx context.Context, path string) (*FileInfo, error)
	GetDirectorySize(ctx context.Context, path string) (int64, error)
	GetAvailableSpace(ctx context.Context) (int64, error)

	// Проверка целостности
	CalculateChecksum(ctx context.Context, path string, algorithm string) (string, error)
	VerifyChecksum(ctx context.Context, path string, expectedChecksum string, algorithm string) (bool, error)

	// Очистка и обслуживание
	CleanupOrphanedFiles(ctx context.Context) error
	OptimizeStorage(ctx context.Context) error
}
