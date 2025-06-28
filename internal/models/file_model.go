package models

import (
	"time"

	"github.com/google/uuid"
)

// File представляет файл или папку в системе
type File struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	OwnerID        uuid.UUID  `json:"owner_id" db:"owner_id"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty" db:"parent_id"`
	Name           string     `json:"name" db:"name"`
	FileExtension  *string    `json:"file_extension,omitempty" db:"file_extension"`
	MimeType       string     `json:"mime_type" db:"mime_type"`
	StoragePath    string     `json:"storage_path" db:"storage_path"`
	Size           int64      `json:"size" db:"size"`
	MD5Checksum    *string    `json:"md5_checksum,omitempty" db:"md5_checksum"`
	SHA256Checksum *string    `json:"sha256_checksum,omitempty" db:"sha256_checksum"`
	IsFolder       bool       `json:"is_folder" db:"is_folder"`
	IsTrashed      bool       `json:"is_trashed" db:"is_trashed"`
	TrashedAt      *time.Time `json:"trashed_at,omitempty" db:"trashed_at"`
	Starred        bool       `json:"starred" db:"starred"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	LastViewedAt   *time.Time `json:"last_viewed_at,omitempty" db:"last_viewed_at"`
	ViewedByMe     bool       `json:"viewed_by_me" db:"viewed_by_me"`
	Version        int64      `json:"version" db:"version"`
	RevisionID     *uuid.UUID `json:"revision_id,omitempty" db:"revision_id"`
	IndexableText  *string    `json:"indexable_text,omitempty" db:"indexable_text"`
	ThumbnailLink  *string    `json:"thumbnail_link,omitempty" db:"thumbnail_link"`
	WebViewLink    *string    `json:"web_view_link,omitempty" db:"web_view_link"`
	WebContentLink *string    `json:"web_content_link,omitempty" db:"web_content_link"`
	IconLink       *string    `json:"icon_link,omitempty" db:"icon_link"`
}

// FileRevision представляет ревизию файла
type FileRevision struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	FileID      uuid.UUID  `json:"file_id" db:"file_id"`
	RevisionID  int64      `json:"revision_id" db:"revision_id"`
	MD5Checksum *string    `json:"md5_checksum,omitempty" db:"md5_checksum"`
	Size        int64      `json:"size" db:"size"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	StoragePath string     `json:"storage_path" db:"storage_path"`
	MimeType    *string    `json:"mime_type,omitempty" db:"mime_type"`
	UserID      *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
}

// FilePermission представляет права доступа к файлу
type FilePermission struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	FileID      uuid.UUID  `json:"file_id" db:"file_id"`
	GranteeID   *uuid.UUID `json:"grantee_id,omitempty" db:"grantee_id"`
	GranteeType string     `json:"grantee_type" db:"grantee_type"`
	Role        string     `json:"role" db:"role"`
	AllowShare  bool       `json:"allow_share" db:"allow_share"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// CreateFileRequest запрос на создание файла
type CreateFileRequest struct {
	Name     string     `json:"name" validate:"required"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	IsFolder bool       `json:"is_folder"`
	MimeType string     `json:"mime_type,omitempty"`
	Size     int64      `json:"size,omitempty"`
	Content  []byte     `json:"content,omitempty"`
}

// UpdateFileRequest запрос на обновление файла
type UpdateFileRequest struct {
	Name     *string    `json:"name,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Starred  *bool      `json:"starred,omitempty"`
	Content  []byte     `json:"content,omitempty"`
}

// FileListRequest запрос на получение списка файлов
type FileListRequest struct {
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	Path      string     `json:"path,omitempty"`
	IsTrashed *bool      `json:"is_trashed,omitempty"`
	Starred   *bool      `json:"starred,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
	OrderBy   string     `json:"order_by,omitempty"`
	OrderDir  string     `json:"order_dir,omitempty"`
}

// FileListResponse ответ со списком файлов
type FileListResponse struct {
	Files  []File `json:"files"`
	Total  int64  `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Permission constants
const (
	GranteeTypeUser   = "USER"
	GranteeTypeGroup  = "GROUP"
	GranteeTypeDomain = "DOMAIN"
	GranteeTypeAnyone = "ANYONE"
)

// Role constants
const (
	RoleOwner     = "OWNER"
	RoleOrganizer = "ORGANIZER"
	RoleFileOwner = "FILE_OWNER"
	RoleWriter    = "WRITER"
	RoleCommenter = "COMMENTER"
	RoleReader    = "READER"
)

// ResumableSession представляет сессию для возобновляемых загрузок/скачиваний
type ResumableSession struct {
	ID          string     `json:"id" db:"id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	FilePath    string     `json:"file_path" db:"file_path"`
	Size        int64      `json:"size" db:"size"`
	Checksum    string     `json:"checksum" db:"checksum"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty" db:"uploaded_at"`
	ExpiresAt   time.Time  `json:"expires_at" db:"expires_at"`
	IsCompleted bool       `json:"is_completed" db:"is_completed"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// ResumableDownloadSession представляет сессию для возобновляемого скачивания
type ResumableDownloadSession struct {
	ID          string     `json:"id"`
	FileID      uuid.UUID  `json:"file_id"`
	UserID      uuid.UUID  `json:"user_id"`
	FileName    string     `json:"file_name"`
	FilePath    string     `json:"file_path"`
	Size        int64      `json:"size"`
	Checksum    string     `json:"checksum"`
	MimeType    string     `json:"mime_type"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
