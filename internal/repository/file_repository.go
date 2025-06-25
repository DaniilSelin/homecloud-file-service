package repository

import (
	"context"
	"fmt"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/dbmanager"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/models"

	"homecloud-file-service/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type fileRepository struct {
	cfg      *config.Config
	dbClient interfaces.DBManagerClient
}

func NewFileRepository(cfg *config.Config) (interfaces.FileRepository, error) {
	// Создаем gRPC клиент для dbmanager
	dbClient, err := dbmanager.NewGRPCDBClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create dbmanager client: %w", err)
	}

	return &fileRepository{
		cfg:      cfg,
		dbClient: dbClient,
	}, nil
}

// Основные операции с файлами
func (r *fileRepository) CreateFile(ctx context.Context, file *models.File) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateFile (repo) called", zap.String("fileID", file.ID.String()))

	err := r.dbClient.CreateFile(ctx, file)
	if err != nil {
		lg.Error(ctx, "Failed to create file", zap.Error(err))
		return fmt.Errorf("failed to create file: %w", err)
	}

	lg.Info(ctx, "File created successfully", zap.String("fileID", file.ID.String()))
	return nil
}

func (r *fileRepository) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileByID (repo) called", zap.String("fileID", id.String()))

	file, err := r.dbClient.GetFileByID(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to get file by ID", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	lg.Info(ctx, "File retrieved successfully", zap.String("fileID", id.String()))
	return file, nil
}

func (r *fileRepository) GetFileByPath(ctx context.Context, ownerID uuid.UUID, path string) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileByPath (repo) called", zap.String("ownerID", ownerID.String()), zap.String("path", path))

	file, err := r.dbClient.GetFileByPath(ctx, ownerID, path)
	if err != nil {
		lg.Error(ctx, "Failed to get file by path", zap.Error(err))
		return nil, fmt.Errorf("failed to get file by path: %w", err)
	}

	lg.Info(ctx, "File by path retrieved successfully", zap.String("path", path))
	return file, nil
}

func (r *fileRepository) UpdateFile(ctx context.Context, file *models.File) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateFile (repo) called", zap.String("fileID", file.ID.String()))

	err := r.dbClient.UpdateFile(ctx, file)
	if err != nil {
		lg.Error(ctx, "Failed to update file", zap.Error(err))
		return fmt.Errorf("failed to update file: %w", err)
	}

	lg.Info(ctx, "File updated successfully", zap.String("fileID", file.ID.String()))
	return nil
}

func (r *fileRepository) DeleteFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteFile (repo) called", zap.String("fileID", id.String()))

	err := r.dbClient.DeleteFile(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to delete file", zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	lg.Info(ctx, "File deleted successfully", zap.String("fileID", id.String()))
	return nil
}

func (r *fileRepository) SoftDeleteFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "SoftDeleteFile (repo) called", zap.String("fileID", id.String()))

	err := r.dbClient.SoftDeleteFile(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to soft delete file", zap.Error(err))
		return fmt.Errorf("failed to soft delete file: %w", err)
	}

	lg.Info(ctx, "File soft deleted successfully", zap.String("fileID", id.String()))
	return nil
}

func (r *fileRepository) RestoreFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "RestoreFile (repo) called", zap.String("fileID", id.String()))

	err := r.dbClient.RestoreFile(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to restore file", zap.Error(err))
		return fmt.Errorf("failed to restore file: %w", err)
	}

	lg.Info(ctx, "File restored successfully", zap.String("fileID", id.String()))
	return nil
}

// Операции со списками файлов
func (r *fileRepository) ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListFiles (repo) called", zap.String("ownerID", req.OwnerID.String()))

	response, err := r.dbClient.ListFiles(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to list files", zap.Error(err))
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	lg.Info(ctx, "Files listed successfully", zap.Int("count", len(response.Files)))
	return response, nil
}

func (r *fileRepository) ListFilesByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListFilesByParent (repo) called", zap.String("ownerID", ownerID.String()))

	files, err := r.dbClient.ListFilesByParent(ctx, ownerID, parentID)
	if err != nil {
		lg.Error(ctx, "Failed to list files by parent", zap.Error(err))
		return nil, fmt.Errorf("failed to list files by parent: %w", err)
	}

	lg.Info(ctx, "Files by parent listed successfully", zap.Int("count", len(files)))
	return files, nil
}

func (r *fileRepository) ListStarredFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListStarredFiles (repo) called", zap.String("ownerID", ownerID.String()))

	files, err := r.dbClient.ListStarredFiles(ctx, ownerID)
	if err != nil {
		lg.Error(ctx, "Failed to list starred files", zap.Error(err))
		return nil, fmt.Errorf("failed to list starred files: %w", err)
	}

	lg.Info(ctx, "Starred files listed successfully", zap.Int("count", len(files)))
	return files, nil
}

func (r *fileRepository) ListTrashedFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "ListTrashedFiles (repo) called", zap.String("ownerID", ownerID.String()))

	files, err := r.dbClient.ListTrashedFiles(ctx, ownerID)
	if err != nil {
		lg.Error(ctx, "Failed to list trashed files", zap.Error(err))
		return nil, fmt.Errorf("failed to list trashed files: %w", err)
	}

	lg.Info(ctx, "Trashed files listed successfully", zap.Int("count", len(files)))
	return files, nil
}

// Операции с ревизиями
func (r *fileRepository) CreateRevision(ctx context.Context, revision *models.FileRevision) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreateRevision (repo) called", zap.String("revisionID", revision.ID.String()))

	err := r.dbClient.CreateRevision(ctx, revision)
	if err != nil {
		lg.Error(ctx, "Failed to create revision", zap.Error(err))
		return fmt.Errorf("failed to create revision: %w", err)
	}

	lg.Info(ctx, "Revision created successfully", zap.String("revisionID", revision.ID.String()))
	return nil
}

func (r *fileRepository) GetRevisions(ctx context.Context, fileID uuid.UUID) ([]models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetRevisions (repo) called", zap.String("fileID", fileID.String()))

	revisions, err := r.dbClient.GetRevisions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get revisions", zap.Error(err))
		return nil, fmt.Errorf("failed to get revisions: %w", err)
	}

	lg.Info(ctx, "Revisions retrieved successfully", zap.Int("count", len(revisions)))
	return revisions, nil
}

func (r *fileRepository) GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64) (*models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetRevision (repo) called", zap.String("fileID", fileID.String()), zap.Int64("revisionID", revisionID))

	revision, err := r.dbClient.GetRevision(ctx, fileID, revisionID)
	if err != nil {
		lg.Error(ctx, "Failed to get revision", zap.Error(err))
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	lg.Info(ctx, "Revision retrieved successfully", zap.String("revisionID", revision.ID.String()))
	return revision, nil
}

func (r *fileRepository) DeleteRevision(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeleteRevision (repo) called", zap.String("revisionID", id.String()))

	err := r.dbClient.DeleteRevision(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to delete revision", zap.Error(err))
		return fmt.Errorf("failed to delete revision: %w", err)
	}

	lg.Info(ctx, "Revision deleted successfully", zap.String("revisionID", id.String()))
	return nil
}

// Операции с правами доступа
func (r *fileRepository) CreatePermission(ctx context.Context, permission *models.FilePermission) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CreatePermission (repo) called", zap.String("permissionID", permission.ID.String()))

	err := r.dbClient.CreatePermission(ctx, permission)
	if err != nil {
		lg.Error(ctx, "Failed to create permission", zap.Error(err))
		return fmt.Errorf("failed to create permission: %w", err)
	}

	lg.Info(ctx, "Permission created successfully", zap.String("permissionID", permission.ID.String()))
	return nil
}

func (r *fileRepository) GetPermissions(ctx context.Context, fileID uuid.UUID) ([]models.FilePermission, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetPermissions (repo) called", zap.String("fileID", fileID.String()))

	permissions, err := r.dbClient.GetPermissions(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get permissions", zap.Error(err))
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	lg.Info(ctx, "Permissions retrieved successfully", zap.Int("count", len(permissions)))
	return permissions, nil
}

func (r *fileRepository) UpdatePermission(ctx context.Context, permission *models.FilePermission) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdatePermission (repo) called", zap.String("permissionID", permission.ID.String()))

	err := r.dbClient.UpdatePermission(ctx, permission)
	if err != nil {
		lg.Error(ctx, "Failed to update permission", zap.Error(err))
		return fmt.Errorf("failed to update permission: %w", err)
	}

	lg.Info(ctx, "Permission updated successfully", zap.String("permissionID", permission.ID.String()))
	return nil
}

func (r *fileRepository) DeletePermission(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "DeletePermission (repo) called", zap.String("permissionID", id.String()))

	err := r.dbClient.DeletePermission(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to delete permission", zap.Error(err))
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	lg.Info(ctx, "Permission deleted successfully", zap.String("permissionID", id.String()))
	return nil
}

func (r *fileRepository) CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CheckPermission (repo) called", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	hasPermission, err := r.dbClient.CheckPermission(ctx, fileID, userID, requiredRole)
	if err != nil {
		lg.Error(ctx, "Failed to check permission", zap.Error(err))
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	lg.Info(ctx, "Permission checked successfully", zap.Bool("hasPermission", hasPermission))
	return hasPermission, nil
}

// Специальные операции
func (r *fileRepository) GetFileSize(ctx context.Context, id uuid.UUID) (int64, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileSize (repo) called", zap.String("fileID", id.String()))

	size, err := r.dbClient.GetFileSize(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to get file size", zap.Error(err))
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}

	lg.Info(ctx, "File size retrieved successfully", zap.Int64("size", size))
	return size, nil
}

func (r *fileRepository) UpdateFileSize(ctx context.Context, id uuid.UUID, size int64) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateFileSize (repo) called", zap.String("fileID", id.String()), zap.Int64("size", size))

	err := r.dbClient.UpdateFileSize(ctx, id, size)
	if err != nil {
		lg.Error(ctx, "Failed to update file size", zap.Error(err))
		return fmt.Errorf("failed to update file size: %w", err)
	}

	lg.Info(ctx, "File size updated successfully", zap.String("fileID", id.String()))
	return nil
}

func (r *fileRepository) UpdateLastViewed(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateLastViewed (repo) called", zap.String("fileID", id.String()))

	err := r.dbClient.UpdateLastViewed(ctx, id)
	if err != nil {
		lg.Error(ctx, "Failed to update last viewed", zap.Error(err))
		return fmt.Errorf("failed to update last viewed: %w", err)
	}

	lg.Info(ctx, "Last viewed updated successfully", zap.String("fileID", id.String()))
	return nil
}

func (r *fileRepository) SearchFiles(ctx context.Context, ownerID uuid.UUID, query string) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "SearchFiles (repo) called", zap.String("ownerID", ownerID.String()), zap.String("query", query))

	files, err := r.dbClient.SearchFiles(ctx, ownerID, query)
	if err != nil {
		lg.Error(ctx, "Failed to search files", zap.Error(err))
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	lg.Info(ctx, "Files searched successfully", zap.Int("count", len(files)))
	return files, nil
}

func (r *fileRepository) GetFileTree(ctx context.Context, ownerID uuid.UUID, rootID *uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileTree called", zap.String("ownerID", ownerID.String()), zap.Any("rootID", rootID))

	files, err := r.dbClient.GetFileTree(ctx, ownerID, rootID)
	if err != nil {
		lg.Error(ctx, "Failed to get file tree from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file tree: %w", err)
	}

	lg.Info(ctx, "File tree retrieved successfully from dbmanager", zap.Int("count", len(files)))
	return files, nil
}

// Дополнительные операции с файлами
func (r *fileRepository) MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "MoveFile called", zap.String("fileID", fileID.String()), zap.Any("newParentID", newParentID))

	err := r.dbClient.MoveFile(ctx, fileID, newParentID)
	if err != nil {
		lg.Error(ctx, "Failed to move file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to move file: %w", err)
	}

	lg.Info(ctx, "File moved successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (r *fileRepository) CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CopyFile called", zap.String("fileID", fileID.String()), zap.Any("newParentID", newParentID), zap.String("newName", newName))

	copiedFile, err := r.dbClient.CopyFile(ctx, fileID, newParentID, newName)
	if err != nil {
		lg.Error(ctx, "Failed to copy file in dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	lg.Info(ctx, "File copied successfully in dbmanager", zap.String("originalFileID", fileID.String()), zap.String("newFileID", copiedFile.ID.String()))
	return copiedFile, nil
}

func (r *fileRepository) StarFile(ctx context.Context, fileID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "StarFile called", zap.String("fileID", fileID.String()))

	err := r.dbClient.StarFile(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to star file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to star file: %w", err)
	}

	lg.Info(ctx, "File starred successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (r *fileRepository) UnstarFile(ctx context.Context, fileID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UnstarFile called", zap.String("fileID", fileID.String()))

	err := r.dbClient.UnstarFile(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to unstar file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to unstar file: %w", err)
	}

	lg.Info(ctx, "File unstarred successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

// Операции с метаданными
func (r *fileRepository) UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "UpdateFileMetadata called", zap.String("fileID", fileID.String()))

	err := r.dbClient.UpdateFileMetadata(ctx, fileID, metadata)
	if err != nil {
		lg.Error(ctx, "Failed to update file metadata in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	lg.Info(ctx, "File metadata updated successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (r *fileRepository) GetFileMetadata(ctx context.Context, fileID uuid.UUID) (map[string]interface{}, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "GetFileMetadata called", zap.String("fileID", fileID.String()))

	metadata, err := r.dbClient.GetFileMetadata(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to get file metadata from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	lg.Info(ctx, "File metadata retrieved successfully from dbmanager", zap.String("fileID", fileID.String()))
	return metadata, nil
}

// Операции проверки целостности
func (r *fileRepository) VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "VerifyFileIntegrity called", zap.String("fileID", fileID.String()))

	isIntegrityVerified, err := r.dbClient.VerifyFileIntegrity(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to verify file integrity in dbmanager", zap.Error(err))
		return false, fmt.Errorf("failed to verify file integrity: %w", err)
	}

	lg.Info(ctx, "File integrity verified successfully in dbmanager", zap.Bool("isIntegrityVerified", isIntegrityVerified))
	return isIntegrityVerified, nil
}

func (r *fileRepository) CalculateFileChecksums(ctx context.Context, fileID uuid.UUID) (map[string]string, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "CalculateFileChecksums called", zap.String("fileID", fileID.String()))

	checksums, err := r.dbClient.CalculateFileChecksums(ctx, fileID)
	if err != nil {
		lg.Error(ctx, "Failed to calculate file checksums in dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to calculate file checksums: %w", err)
	}

	lg.Info(ctx, "File checksums calculated successfully in dbmanager", zap.Int("checksumCount", len(checksums)))
	return checksums, nil
}
