package interfaces

import (
	"context"

	"homecloud-file-service/internal/models"

	"github.com/google/uuid"
)

// DBManagerClient интерфейс для работы с dbmanager сервисом
type DBManagerClient interface {
	// File operations
	CreateFile(ctx context.Context, file *models.File) error
	GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error)
	GetFileByPath(ctx context.Context, ownerID uuid.UUID, path string) (*models.File, error)
	UpdateFile(ctx context.Context, file *models.File) error
	DeleteFile(ctx context.Context, id uuid.UUID) error
	SoftDeleteFile(ctx context.Context, id uuid.UUID) error
	RestoreFile(ctx context.Context, id uuid.UUID) error
	ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error)
	ListFilesByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]models.File, error)
	ListStarredFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error)
	ListTrashedFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error)
	SearchFiles(ctx context.Context, ownerID uuid.UUID, query string) ([]models.File, error)
	GetFileSize(ctx context.Context, id uuid.UUID) (int64, error)
	UpdateFileSize(ctx context.Context, id uuid.UUID, size int64) error
	UpdateLastViewed(ctx context.Context, id uuid.UUID) error
	GetFileTree(ctx context.Context, ownerID uuid.UUID, rootID *uuid.UUID) ([]models.File, error)

	// Revision operations
	CreateRevision(ctx context.Context, revision *models.FileRevision) error
	GetRevisions(ctx context.Context, fileID uuid.UUID) ([]models.FileRevision, error)
	GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64) (*models.FileRevision, error)
	DeleteRevision(ctx context.Context, id uuid.UUID) error

	// Permission operations
	CreatePermission(ctx context.Context, permission *models.FilePermission) error
	GetPermissions(ctx context.Context, fileID uuid.UUID) ([]models.FilePermission, error)
	UpdatePermission(ctx context.Context, permission *models.FilePermission) error
	DeletePermission(ctx context.Context, id uuid.UUID) error
	CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error)

	// File metadata operations
	UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}) error
	GetFileMetadata(ctx context.Context, fileID uuid.UUID) (map[string]interface{}, error)

	// File operations (star, move, copy, rename)
	StarFile(ctx context.Context, fileID uuid.UUID) error
	UnstarFile(ctx context.Context, fileID uuid.UUID) error
	MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID) error
	CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string) (*models.File, error)
	RenameFile(ctx context.Context, fileID uuid.UUID, newName string) error

	// File integrity operations
	VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID) (bool, error)
	CalculateFileChecksums(ctx context.Context, fileID uuid.UUID) (map[string]string, error)

	// Close closes the connection
	Close() error
}
