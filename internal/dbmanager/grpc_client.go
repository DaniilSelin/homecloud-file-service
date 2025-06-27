package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"
	"homecloud-file-service/internal/models"
	pb "homecloud-file-service/internal/transport/grpc/protos"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCDBClient клиент для связи с dbmanager сервисом
type GRPCDBClient struct {
	client pb.DBServiceClient
	conn   *grpc.ClientConn
}

// Убеждаемся, что GRPCDBClient реализует интерфейс DBManagerClient
var _ interfaces.DBManagerClient = (*GRPCDBClient)(nil)

// NewGRPCDBClient создает новый gRPC клиент для dbmanager
func NewGRPCDBClient(cfg *config.Config) (*GRPCDBClient, error) {
	// Логируем конфигурацию для диагностики
	fmt.Printf("NewGRPCDBClient: DbManager config - Host: '%s', Port: %d\n", cfg.DbManager.Host, cfg.DbManager.Port)

	// Настраиваем параметры соединения
	keepaliveParams := keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}

	// Создаем соединение
	addr := fmt.Sprintf("%s:%d", cfg.DbManager.Host, cfg.DbManager.Port)
	fmt.Printf("NewGRPCDBClient: Connecting to dbmanager at: %s\n", addr)

	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepaliveParams),
	)
	if err != nil {
		fmt.Printf("NewGRPCDBClient: Failed to connect to dbmanager at %s: %v\n", addr, err)
		return nil, fmt.Errorf("failed to connect to dbmanager: %w", err)
	}

	fmt.Printf("NewGRPCDBClient: Successfully connected to dbmanager at %s\n", addr)
	client := pb.NewDBServiceClient(conn)

	return &GRPCDBClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close закрывает соединение с dbmanager
func (c *GRPCDBClient) Close() error {
	return c.conn.Close()
}

// convertFileToProto конвертирует модель File в proto сообщение
func convertFileToProto(file *models.File) *pb.File {
	protoFile := &pb.File{
		Id:          file.ID.String(),
		OwnerId:     file.OwnerID.String(),
		Name:        file.Name,
		MimeType:    file.MimeType,
		StoragePath: file.StoragePath,
		Size:        file.Size,
		IsFolder:    file.IsFolder,
		IsTrashed:   file.IsTrashed,
		Starred:     file.Starred,
		ViewedByMe:  file.ViewedByMe,
		Version:     file.Version,
		CreatedAt:   timestamppb.New(file.CreatedAt),
		UpdatedAt:   timestamppb.New(file.UpdatedAt),
	}

	// Опциональные поля
	if file.ParentID != nil {
		protoFile.ParentId = file.ParentID.String()
	}
	if file.FileExtension != nil {
		protoFile.FileExtension = *file.FileExtension
	}
	if file.MD5Checksum != nil {
		protoFile.Md5Checksum = *file.MD5Checksum
	}
	if file.SHA256Checksum != nil {
		protoFile.Sha256Checksum = *file.SHA256Checksum
	}
	if file.TrashedAt != nil {
		protoFile.TrashedAt = timestamppb.New(*file.TrashedAt)
	}
	if file.LastViewedAt != nil {
		protoFile.LastViewedAt = timestamppb.New(*file.LastViewedAt)
	}
	if file.RevisionID != nil {
		protoFile.RevisionId = file.RevisionID.String()
	}
	if file.IndexableText != nil {
		protoFile.IndexableText = *file.IndexableText
	}
	if file.ThumbnailLink != nil {
		protoFile.ThumbnailLink = *file.ThumbnailLink
	}
	if file.WebViewLink != nil {
		protoFile.WebViewLink = *file.WebViewLink
	}
	if file.WebContentLink != nil {
		protoFile.WebContentLink = *file.WebContentLink
	}
	if file.IconLink != nil {
		protoFile.IconLink = *file.IconLink
	}

	return protoFile
}

// convertProtoToFile конвертирует proto сообщение в модель File
func convertProtoToFile(protoFile *pb.File) (*models.File, error) {
	fileID, err := uuid.Parse(protoFile.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid file ID: %w", err)
	}

	ownerID, err := uuid.Parse(protoFile.OwnerId)
	if err != nil {
		return nil, fmt.Errorf("invalid owner ID: %w", err)
	}

	file := &models.File{
		ID:          fileID,
		OwnerID:     ownerID,
		Name:        protoFile.Name,
		MimeType:    protoFile.MimeType,
		StoragePath: protoFile.StoragePath,
		Size:        protoFile.Size,
		IsFolder:    protoFile.IsFolder,
		IsTrashed:   protoFile.IsTrashed,
		Starred:     protoFile.Starred,
		ViewedByMe:  protoFile.ViewedByMe,
		Version:     protoFile.Version,
		CreatedAt:   protoFile.CreatedAt.AsTime(),
		UpdatedAt:   protoFile.UpdatedAt.AsTime(),
	}

	// Опциональные поля
	if protoFile.ParentId != "" {
		parentID, err := uuid.Parse(protoFile.ParentId)
		if err != nil {
			return nil, fmt.Errorf("invalid parent ID: %w", err)
		}
		file.ParentID = &parentID
	}
	if protoFile.FileExtension != "" {
		file.FileExtension = &protoFile.FileExtension
	}
	if protoFile.Md5Checksum != "" {
		file.MD5Checksum = &protoFile.Md5Checksum
	}
	if protoFile.Sha256Checksum != "" {
		file.SHA256Checksum = &protoFile.Sha256Checksum
	}
	if protoFile.TrashedAt != nil {
		trashedAt := protoFile.TrashedAt.AsTime()
		file.TrashedAt = &trashedAt
	}
	if protoFile.LastViewedAt != nil {
		lastViewedAt := protoFile.LastViewedAt.AsTime()
		file.LastViewedAt = &lastViewedAt
	}
	if protoFile.RevisionId != "" {
		revisionID, err := uuid.Parse(protoFile.RevisionId)
		if err != nil {
			return nil, fmt.Errorf("invalid revision ID: %w", err)
		}
		file.RevisionID = &revisionID
	}
	if protoFile.IndexableText != "" {
		file.IndexableText = &protoFile.IndexableText
	}
	if protoFile.ThumbnailLink != "" {
		file.ThumbnailLink = &protoFile.ThumbnailLink
	}
	if protoFile.WebViewLink != "" {
		file.WebViewLink = &protoFile.WebViewLink
	}
	if protoFile.WebContentLink != "" {
		file.WebContentLink = &protoFile.WebContentLink
	}
	if protoFile.IconLink != "" {
		file.IconLink = &protoFile.IconLink
	}

	return file, nil
}

// File operations
func (c *GRPCDBClient) CreateFile(ctx context.Context, file *models.File) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Creating file in dbmanager", zap.String("fileID", file.ID.String()))

	protoFile := convertFileToProto(file)
	_, err := c.client.CreateFile(ctx, protoFile)
	if err != nil {
		lg.Error(ctx, "Failed to create file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to create file: %w", err)
	}

	lg.Info(ctx, "File created successfully in dbmanager", zap.String("fileID", file.ID.String()))
	return nil
}

func (c *GRPCDBClient) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting file from dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	protoFile, err := c.client.GetFileByID(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get file from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	file, err := convertProtoToFile(protoFile)
	if err != nil {
		lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
		return nil, fmt.Errorf("failed to convert file: %w", err)
	}

	lg.Info(ctx, "File retrieved successfully from dbmanager", zap.String("fileID", id.String()))
	return file, nil
}

func (c *GRPCDBClient) UpdateFile(ctx context.Context, file *models.File) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Updating file in dbmanager", zap.String("fileID", file.ID.String()))

	protoFile := convertFileToProto(file)
	_, err := c.client.UpdateFile(ctx, protoFile)
	if err != nil {
		lg.Error(ctx, "Failed to update file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update file: %w", err)
	}

	lg.Info(ctx, "File updated successfully in dbmanager", zap.String("fileID", file.ID.String()))
	return nil
}

func (c *GRPCDBClient) DeleteFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Deleting file from dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	_, err := c.client.DeleteFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to delete file from dbmanager", zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	lg.Info(ctx, "File deleted successfully from dbmanager", zap.String("fileID", id.String()))
	return nil
}

func (c *GRPCDBClient) SoftDeleteFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Soft deleting file in dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	_, err := c.client.SoftDeleteFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to soft delete file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to soft delete file: %w", err)
	}

	lg.Info(ctx, "File soft deleted successfully in dbmanager", zap.String("fileID", id.String()))
	return nil
}

func (c *GRPCDBClient) RestoreFile(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Restoring file in dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	_, err := c.client.RestoreFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to restore file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to restore file: %w", err)
	}

	lg.Info(ctx, "File restored successfully in dbmanager", zap.String("fileID", id.String()))
	return nil
}

func (c *GRPCDBClient) ListFiles(ctx context.Context, req *models.FileListRequest) (*models.FileListResponse, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Listing files from dbmanager", zap.String("ownerID", req.OwnerID.String()))

	protoReq := &pb.ListFilesRequest{
		OwnerId:   req.OwnerID.String(),
		IsTrashed: req.IsTrashed != nil && *req.IsTrashed,
		Starred:   req.Starred != nil && *req.Starred,
		Limit:     int32(req.Limit),
		Offset:    int32(req.Offset),
		OrderBy:   req.OrderBy,
		OrderDir:  req.OrderDir,
	}

	if req.ParentID != nil {
		protoReq.ParentId = req.ParentID.String()
	}

	protoResp, err := c.client.ListFiles(ctx, protoReq)
	if err != nil {
		lg.Error(ctx, "Failed to list files from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	response := &models.FileListResponse{
		Files:  files,
		Total:  protoResp.Total,
		Limit:  int(protoResp.Limit),
		Offset: int(protoResp.Offset),
	}

	lg.Info(ctx, "Files listed successfully from dbmanager", zap.Int("count", len(files)))
	return response, nil
}

func (c *GRPCDBClient) ListFilesByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Listing files by parent from dbmanager", zap.String("ownerID", ownerID.String()))

	req := &pb.ListFilesByParentRequest{
		OwnerId: ownerID.String(),
	}

	if parentID != nil {
		req.ParentId = parentID.String()
	}

	protoResp, err := c.client.ListFilesByParent(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to list files by parent from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to list files by parent: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	lg.Info(ctx, "Files by parent listed successfully from dbmanager", zap.Int("count", len(files)))
	return files, nil
}

func (c *GRPCDBClient) UpdateLastViewed(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Updating last viewed in dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	_, err := c.client.UpdateLastViewed(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to update last viewed in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update last viewed: %w", err)
	}

	lg.Info(ctx, "Last viewed updated successfully in dbmanager", zap.String("fileID", id.String()))
	return nil
}

func (c *GRPCDBClient) CheckPermission(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, requiredRole string) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Checking permission in dbmanager", zap.String("fileID", fileID.String()), zap.String("userID", userID.String()))

	req := &pb.CheckPermissionRequest{
		FileId:       fileID.String(),
		UserId:       userID.String(),
		RequiredRole: requiredRole,
	}

	resp, err := c.client.CheckPermission(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to check permission in dbmanager", zap.Error(err))
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	lg.Info(ctx, "Permission checked successfully in dbmanager", zap.Bool("hasPermission", resp.HasPermission))
	return resp.HasPermission, nil
}

// Дополнительные методы для работы с файлами
func (c *GRPCDBClient) GetFileByPath(ctx context.Context, ownerID uuid.UUID, path string) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting file by path from dbmanager", zap.String("ownerID", ownerID.String()), zap.String("path", path))

	req := &pb.GetFileByPathRequest{
		OwnerId: ownerID.String(),
		Path:    path,
	}

	protoFile, err := c.client.GetFileByPath(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get file by path from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file by path: %w", err)
	}

	file, err := convertProtoToFile(protoFile)
	if err != nil {
		lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
		return nil, fmt.Errorf("failed to convert file: %w", err)
	}

	lg.Info(ctx, "File by path retrieved successfully from dbmanager", zap.String("path", path))
	return file, nil
}

func (c *GRPCDBClient) ListStarredFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Listing starred files from dbmanager", zap.String("ownerID", ownerID.String()))

	req := &pb.ListStarredFilesRequest{
		OwnerId: ownerID.String(),
	}

	protoResp, err := c.client.ListStarredFiles(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to list starred files from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to list starred files: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	lg.Info(ctx, "Starred files listed successfully from dbmanager", zap.Int("count", len(files)))
	return files, nil
}

func (c *GRPCDBClient) ListTrashedFiles(ctx context.Context, ownerID uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Listing trashed files from dbmanager", zap.String("ownerID", ownerID.String()))

	req := &pb.ListTrashedFilesRequest{
		OwnerId: ownerID.String(),
	}

	protoResp, err := c.client.ListTrashedFiles(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to list trashed files from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to list trashed files: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	lg.Info(ctx, "Trashed files listed successfully from dbmanager", zap.Int("count", len(files)))
	return files, nil
}

func (c *GRPCDBClient) SearchFiles(ctx context.Context, ownerID uuid.UUID, query string) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Searching files in dbmanager", zap.String("ownerID", ownerID.String()), zap.String("query", query))

	req := &pb.SearchFilesRequest{
		OwnerId: ownerID.String(),
		Query:   query,
	}

	protoResp, err := c.client.SearchFiles(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to search files in dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	lg.Info(ctx, "Files searched successfully in dbmanager", zap.Int("count", len(files)))
	return files, nil
}

func (c *GRPCDBClient) GetFileSize(ctx context.Context, id uuid.UUID) (int64, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting file size from dbmanager", zap.String("fileID", id.String()))

	req := &pb.FileID{Id: id.String()}
	resp, err := c.client.GetFileSize(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get file size from dbmanager", zap.Error(err))
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}

	lg.Info(ctx, "File size retrieved successfully from dbmanager", zap.Int64("size", resp.Size))
	return resp.Size, nil
}

func (c *GRPCDBClient) UpdateFileSize(ctx context.Context, id uuid.UUID, size int64) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Updating file size in dbmanager", zap.String("fileID", id.String()), zap.Int64("size", size))

	req := &pb.UpdateFileSizeRequest{
		Id:   id.String(),
		Size: size,
	}

	_, err := c.client.UpdateFileSize(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to update file size in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update file size: %w", err)
	}

	lg.Info(ctx, "File size updated successfully in dbmanager", zap.String("fileID", id.String()))
	return nil
}

func (c *GRPCDBClient) GetFileTree(ctx context.Context, ownerID uuid.UUID, rootID *uuid.UUID) ([]models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting file tree from dbmanager", zap.String("ownerID", ownerID.String()))

	req := &pb.GetFileTreeRequest{
		OwnerId: ownerID.String(),
	}

	if rootID != nil {
		req.RootId = rootID.String()
	}

	protoResp, err := c.client.GetFileTree(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get file tree from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file tree: %w", err)
	}

	// Конвертируем ответ
	files := make([]models.File, len(protoResp.Files))
	for i, protoFile := range protoResp.Files {
		file, err := convertProtoToFile(protoFile)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
			return nil, fmt.Errorf("failed to convert file: %w", err)
		}
		files[i] = *file
	}

	lg.Info(ctx, "File tree retrieved successfully from dbmanager", zap.Int("count", len(files)))
	return files, nil
}

// Методы для работы с ревизиями
func convertRevisionToProto(revision *models.FileRevision) *pb.FileRevision {
	protoRevision := &pb.FileRevision{
		Id:          revision.ID.String(),
		FileId:      revision.FileID.String(),
		RevisionId:  revision.RevisionID,
		Size:        revision.Size,
		CreatedAt:   timestamppb.New(revision.CreatedAt),
		StoragePath: revision.StoragePath,
	}

	// Опциональные поля
	if revision.MD5Checksum != nil {
		protoRevision.Md5Checksum = *revision.MD5Checksum
	}
	if revision.MimeType != nil {
		protoRevision.MimeType = *revision.MimeType
	}
	if revision.UserID != nil {
		protoRevision.UserId = revision.UserID.String()
	}

	return protoRevision
}

func convertProtoToRevision(protoRevision *pb.FileRevision) (*models.FileRevision, error) {
	revisionID, err := uuid.Parse(protoRevision.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid revision ID: %w", err)
	}

	fileID, err := uuid.Parse(protoRevision.FileId)
	if err != nil {
		return nil, fmt.Errorf("invalid file ID: %w", err)
	}

	revision := &models.FileRevision{
		ID:          revisionID,
		FileID:      fileID,
		RevisionID:  protoRevision.RevisionId,
		Size:        protoRevision.Size,
		CreatedAt:   protoRevision.CreatedAt.AsTime(),
		StoragePath: protoRevision.StoragePath,
	}

	// Опциональные поля
	if protoRevision.Md5Checksum != "" {
		revision.MD5Checksum = &protoRevision.Md5Checksum
	}
	if protoRevision.MimeType != "" {
		revision.MimeType = &protoRevision.MimeType
	}
	if protoRevision.UserId != "" {
		userID, err := uuid.Parse(protoRevision.UserId)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		revision.UserID = &userID
	}

	return revision, nil
}

func (c *GRPCDBClient) CreateRevision(ctx context.Context, revision *models.FileRevision) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Creating revision in dbmanager", zap.String("revisionID", revision.ID.String()))

	protoRevision := convertRevisionToProto(revision)
	_, err := c.client.CreateRevision(ctx, protoRevision)
	if err != nil {
		lg.Error(ctx, "Failed to create revision in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to create revision: %w", err)
	}

	lg.Info(ctx, "Revision created successfully in dbmanager", zap.String("revisionID", revision.ID.String()))
	return nil
}

func (c *GRPCDBClient) GetRevisions(ctx context.Context, fileID uuid.UUID) ([]models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting revisions from dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	protoResp, err := c.client.GetRevisions(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get revisions from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get revisions: %w", err)
	}

	// Конвертируем ответ
	revisions := make([]models.FileRevision, len(protoResp.Revisions))
	for i, protoRevision := range protoResp.Revisions {
		revision, err := convertProtoToRevision(protoRevision)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto revision", zap.Error(err))
			return nil, fmt.Errorf("failed to convert revision: %w", err)
		}
		revisions[i] = *revision
	}

	lg.Info(ctx, "Revisions retrieved successfully from dbmanager", zap.Int("count", len(revisions)))
	return revisions, nil
}

func (c *GRPCDBClient) GetRevision(ctx context.Context, fileID uuid.UUID, revisionID int64) (*models.FileRevision, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting revision from dbmanager", zap.String("fileID", fileID.String()), zap.Int64("revisionID", revisionID))

	req := &pb.GetRevisionRequest{
		FileId:     fileID.String(),
		RevisionId: revisionID,
	}

	protoRevision, err := c.client.GetRevision(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get revision from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	revision, err := convertProtoToRevision(protoRevision)
	if err != nil {
		lg.Error(ctx, "Failed to convert proto revision", zap.Error(err))
		return nil, fmt.Errorf("failed to convert revision: %w", err)
	}

	lg.Info(ctx, "Revision retrieved successfully from dbmanager", zap.String("revisionID", revision.ID.String()))
	return revision, nil
}

func (c *GRPCDBClient) DeleteRevision(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Deleting revision from dbmanager", zap.String("revisionID", id.String()))

	req := &pb.RevisionID{Id: id.String()}
	_, err := c.client.DeleteRevision(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to delete revision from dbmanager", zap.Error(err))
		return fmt.Errorf("failed to delete revision: %w", err)
	}

	lg.Info(ctx, "Revision deleted successfully from dbmanager", zap.String("revisionID", id.String()))
	return nil
}

// Методы для работы с правами доступа
func convertPermissionToProto(permission *models.FilePermission) *pb.FilePermission {
	protoPermission := &pb.FilePermission{
		Id:          permission.ID.String(),
		FileId:      permission.FileID.String(),
		GranteeType: permission.GranteeType,
		Role:        permission.Role,
		AllowShare:  permission.AllowShare,
		CreatedAt:   timestamppb.New(permission.CreatedAt),
	}

	// Опциональные поля
	if permission.GranteeID != nil {
		protoPermission.GranteeId = permission.GranteeID.String()
	}

	return protoPermission
}

func convertProtoToPermission(protoPermission *pb.FilePermission) (*models.FilePermission, error) {
	permissionID, err := uuid.Parse(protoPermission.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid permission ID: %w", err)
	}

	fileID, err := uuid.Parse(protoPermission.FileId)
	if err != nil {
		return nil, fmt.Errorf("invalid file ID: %w", err)
	}

	permission := &models.FilePermission{
		ID:          permissionID,
		FileID:      fileID,
		GranteeType: protoPermission.GranteeType,
		Role:        protoPermission.Role,
		AllowShare:  protoPermission.AllowShare,
		CreatedAt:   protoPermission.CreatedAt.AsTime(),
	}

	// Опциональные поля
	if protoPermission.GranteeId != "" {
		granteeID, err := uuid.Parse(protoPermission.GranteeId)
		if err != nil {
			return nil, fmt.Errorf("invalid grantee ID: %w", err)
		}
		permission.GranteeID = &granteeID
	}

	return permission, nil
}

func (c *GRPCDBClient) CreatePermission(ctx context.Context, permission *models.FilePermission) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Creating permission in dbmanager", zap.String("permissionID", permission.ID.String()))

	protoPermission := convertPermissionToProto(permission)
	_, err := c.client.CreatePermission(ctx, protoPermission)
	if err != nil {
		lg.Error(ctx, "Failed to create permission in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to create permission: %w", err)
	}

	lg.Info(ctx, "Permission created successfully in dbmanager", zap.String("permissionID", permission.ID.String()))
	return nil
}

func (c *GRPCDBClient) GetPermissions(ctx context.Context, fileID uuid.UUID) ([]models.FilePermission, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting permissions from dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	protoResp, err := c.client.GetPermissions(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get permissions from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Конвертируем ответ
	permissions := make([]models.FilePermission, len(protoResp.Permissions))
	for i, protoPermission := range protoResp.Permissions {
		permission, err := convertProtoToPermission(protoPermission)
		if err != nil {
			lg.Error(ctx, "Failed to convert proto permission", zap.Error(err))
			return nil, fmt.Errorf("failed to convert permission: %w", err)
		}
		permissions[i] = *permission
	}

	lg.Info(ctx, "Permissions retrieved successfully from dbmanager", zap.Int("count", len(permissions)))
	return permissions, nil
}

func (c *GRPCDBClient) UpdatePermission(ctx context.Context, permission *models.FilePermission) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Updating permission in dbmanager", zap.String("permissionID", permission.ID.String()))

	protoPermission := convertPermissionToProto(permission)
	_, err := c.client.UpdatePermission(ctx, protoPermission)
	if err != nil {
		lg.Error(ctx, "Failed to update permission in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update permission: %w", err)
	}

	lg.Info(ctx, "Permission updated successfully in dbmanager", zap.String("permissionID", permission.ID.String()))
	return nil
}

func (c *GRPCDBClient) DeletePermission(ctx context.Context, id uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Deleting permission from dbmanager", zap.String("permissionID", id.String()))

	req := &pb.PermissionID{Id: id.String()}
	_, err := c.client.DeletePermission(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to delete permission from dbmanager", zap.Error(err))
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	lg.Info(ctx, "Permission deleted successfully from dbmanager", zap.String("permissionID", id.String()))
	return nil
}

// Методы для работы с метаданными файлов
func (c *GRPCDBClient) UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, metadata map[string]interface{}) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Updating file metadata in dbmanager", zap.String("fileID", fileID.String()))

	// Конвертируем metadata в JSON строку
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		lg.Error(ctx, "Failed to marshal metadata", zap.Error(err))
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	req := &pb.UpdateFileMetadataRequest{
		FileId:   fileID.String(),
		Metadata: string(metadataJSON),
	}

	_, err = c.client.UpdateFileMetadata(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to update file metadata in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	lg.Info(ctx, "File metadata updated successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (c *GRPCDBClient) GetFileMetadata(ctx context.Context, fileID uuid.UUID) (map[string]interface{}, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Getting file metadata from dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	resp, err := c.client.GetFileMetadata(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to get file metadata from dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// Конвертируем JSON строку обратно в map
	var metadata map[string]interface{}
	if resp.Metadata != "" {
		if err := json.Unmarshal([]byte(resp.Metadata), &metadata); err != nil {
			lg.Error(ctx, "Failed to unmarshal metadata", zap.Error(err))
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		metadata = make(map[string]interface{})
	}

	lg.Info(ctx, "File metadata retrieved successfully from dbmanager", zap.String("fileID", fileID.String()))
	return metadata, nil
}

// Методы для операций с файлами
func (c *GRPCDBClient) StarFile(ctx context.Context, fileID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Starring file in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	_, err := c.client.StarFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to star file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to star file: %w", err)
	}

	lg.Info(ctx, "File starred successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (c *GRPCDBClient) UnstarFile(ctx context.Context, fileID uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Unstarring file in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	_, err := c.client.UnstarFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to unstar file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to unstar file: %w", err)
	}

	lg.Info(ctx, "File unstarred successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (c *GRPCDBClient) MoveFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Moving file in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.MoveFileRequest{
		FileId: fileID.String(),
	}

	if newParentID != nil {
		req.NewParentId = newParentID.String()
	}

	_, err := c.client.MoveFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to move file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to move file: %w", err)
	}

	lg.Info(ctx, "File moved successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

func (c *GRPCDBClient) CopyFile(ctx context.Context, fileID uuid.UUID, newParentID *uuid.UUID, newName string) (*models.File, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Copying file in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.CopyFileRequest{
		FileId:  fileID.String(),
		NewName: newName,
	}

	if newParentID != nil {
		req.NewParentId = newParentID.String()
	}

	protoFile, err := c.client.CopyFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to copy file in dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	file, err := convertProtoToFile(protoFile)
	if err != nil {
		lg.Error(ctx, "Failed to convert proto file", zap.Error(err))
		return nil, fmt.Errorf("failed to convert file: %w", err)
	}

	lg.Info(ctx, "File copied successfully in dbmanager", zap.String("fileID", fileID.String()))
	return file, nil
}

func (c *GRPCDBClient) RenameFile(ctx context.Context, fileID uuid.UUID, newName string) error {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Renaming file in dbmanager", zap.String("fileID", fileID.String()), zap.String("newName", newName))

	req := &pb.RenameFileRequest{
		FileId:  fileID.String(),
		NewName: newName,
	}

	_, err := c.client.RenameFile(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to rename file in dbmanager", zap.Error(err))
		return fmt.Errorf("failed to rename file: %w", err)
	}

	lg.Info(ctx, "File renamed successfully in dbmanager", zap.String("fileID", fileID.String()))
	return nil
}

// Методы для проверки целостности файлов
func (c *GRPCDBClient) VerifyFileIntegrity(ctx context.Context, fileID uuid.UUID) (bool, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Verifying file integrity in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	resp, err := c.client.VerifyFileIntegrity(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to verify file integrity in dbmanager", zap.Error(err))
		return false, fmt.Errorf("failed to verify file integrity: %w", err)
	}

	lg.Info(ctx, "File integrity verified successfully in dbmanager", zap.Bool("isIntegrityVerified", resp.IsIntegrityVerified))
	return resp.IsIntegrityVerified, nil
}

func (c *GRPCDBClient) CalculateFileChecksums(ctx context.Context, fileID uuid.UUID) (map[string]string, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	lg.Info(ctx, "Calculating file checksums in dbmanager", zap.String("fileID", fileID.String()))

	req := &pb.FileID{Id: fileID.String()}
	resp, err := c.client.CalculateFileChecksums(ctx, req)
	if err != nil {
		lg.Error(ctx, "Failed to calculate file checksums in dbmanager", zap.Error(err))
		return nil, fmt.Errorf("failed to calculate file checksums: %w", err)
	}

	lg.Info(ctx, "File checksums calculated successfully in dbmanager", zap.Int("checksumCount", len(resp.Checksums)))
	return resp.Checksums, nil
}
