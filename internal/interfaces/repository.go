package interfaces

import (
	"context"

	"homecloud-file-service/internal/models"

	"github.com/google/uuid"
)

// FileRepository интерфейс для работы с файлами в БД
type FileRepository interface {
	// Основные операции с файлами
	CreateFile(ctx context.Context, file *models.File) error
	GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error)
	GetFileByPath(ctx context.Context, ownerID uuid.UUID, path string) (*models.File, error)
	UpdateFile(ctx context.Context, file *models.File) error
	DeleteFile(ctx context.Context, id uuid.UUID) error
	SoftDeleteFile(ctx context.Context, id uuid.UUID) error
	RestoreFile(ctx context.Context, id uuid.UUID) error
	CreateFileFromFS(ctx context.Context, file *models.File) error

	// Операции со списками файлов
	ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error)
	ListFilesByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]models.File, error)
	ListStarredFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error)
	ListTrashedFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error)

	// Операции с ревизиями
	CreateRevision(ctx context.Context, revision *models.FileRevision) error
	GetRevisions(ctx context.Context, fileID uuid.UUID) ([]models.FileRevision, error)
	GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64) (*models.FileRevision, error)
	DeleteRevision(ctx context.Context, id uuid.UUID) error

	// Операции с правами доступа
	CreatePermission(ctx context.Context, permission *models.FilePermission) error
	GetPermissions(ctx context.Context, fileID uuid.UUID) ([]models.FilePermission, error)
	UpdatePermission(ctx context.Context, permission *models.FilePermission) error
	DeletePermission(ctx context.Context, id uuid.UUID) error
	CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error)

	// Специальные операции
	GetFileSize(ctx context.Context, id uuid.UUID) (int64, error)
	UpdateFileSize(ctx context.Context, id uuid.UUID, size int64) error
	UpdateLastViewed(ctx context.Context, id uuid.UUID) error
	SearchFiles(ctx context.Context, ownerID uuid.UUID, query string) ([]models.File, error)
	GetFileTree(ctx context.Context, ownerID uuid.UUID, rootID *uuid.UUID) ([]models.File, error)

	// Дополнительные операции с файлами
	MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID) error
	CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string) (*models.File, error)
	StarFile(ctx context.Context, fileID uuid.UUID) error
	UnstarFile(ctx context.Context, fileID uuid.UUID) error

	// Операции с метаданными
	UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}) error
	GetFileMetadata(ctx context.Context, fileID uuid.UUID) (map[string]interface{}, error)

	// Операции проверки целостности
	VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID) (bool, error)
	CalculateFileChecksums(ctx context.Context, fileID uuid.UUID) (map[string]string, error)
}

// StorageRepository интерфейс для работы с файловым хранилищем
type StorageRepository interface {
	// Операции с файлами в хранилище
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

	// Проверка целостности
	CalculateChecksum(ctx context.Context, path string, algorithm string) (string, error)
	VerifyChecksum(ctx context.Context, path string, expectedChecksum string, algorithm string) (bool, error)
}

// FileInfo информация о файле в хранилище
type FileInfo struct {
	Path           string
	Size           int64
	IsDirectory    bool
	ModifiedAt     int64
	MD5Checksum    string
	SHA256Checksum string
}
