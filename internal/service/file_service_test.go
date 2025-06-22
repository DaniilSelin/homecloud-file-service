package service

import (
	"testing"

	"homecloud-file-service/internal/errdefs"
	"homecloud-file-service/internal/models"

	"github.com/google/uuid"
)

// MockFileRepository для тестирования
type MockFileRepository struct {
	files map[uuid.UUID]*models.File
}

func NewMockFileRepository() *MockFileRepository {
	return &MockFileRepository{
		files: make(map[uuid.UUID]*models.File),
	}
}

func (m *MockFileRepository) Create(file *models.File) error {
	m.files[file.ID] = file
	return nil
}

func (m *MockFileRepository) GetByID(id uuid.UUID) (*models.File, error) {
	if file, exists := m.files[id]; exists {
		return file, nil
	}
	return nil, errdefs.ErrFileNotFound
}

func (m *MockFileRepository) GetByUserID(userID uuid.UUID, limit, offset int) ([]*models.File, error) {
	var files []*models.File
	for _, file := range m.files {
		if file.OwnerID == userID {
			files = append(files, file)
		}
	}
	return files, nil
}

func (m *MockFileRepository) Update(file *models.File) error {
	if _, exists := m.files[file.ID]; !exists {
		return errdefs.ErrFileNotFound
	}
	m.files[file.ID] = file
	return nil
}

func (m *MockFileRepository) Delete(id uuid.UUID) error {
	if _, exists := m.files[id]; !exists {
		return errdefs.ErrFileNotFound
	}
	delete(m.files, id)
	return nil
}

func (m *MockFileRepository) SoftDelete(id uuid.UUID) error {
	if file, exists := m.files[id]; exists {
		file.IsTrashed = true
		return nil
	}
	return errdefs.ErrFileNotFound
}

func (m *MockFileRepository) Restore(id uuid.UUID) error {
	if file, exists := m.files[id]; exists {
		file.IsTrashed = false
		return nil
	}
	return errdefs.ErrFileNotFound
}

func (m *MockFileRepository) GetRevisions(fileID uuid.UUID) ([]*models.FileRevision, error) {
	return []*models.FileRevision{}, nil
}

func (m *MockFileRepository) GetRevision(fileID, revisionID uuid.UUID) (*models.FileRevision, error) {
	return nil, errdefs.ErrRevisionNotFound
}

func (m *MockFileRepository) CreateRevision(revision *models.FileRevision) error {
	return nil
}

func (m *MockFileRepository) GetPermissions(fileID uuid.UUID) ([]*models.FilePermission, error) {
	return []*models.FilePermission{}, nil
}

func (m *MockFileRepository) GrantPermission(permission *models.FilePermission) error {
	return nil
}

func (m *MockFileRepository) RevokePermission(fileID, granteeID uuid.UUID) error {
	return nil
}

func (m *MockFileRepository) Search(userID uuid.UUID, query string, limit, offset int) ([]*models.File, error) {
	return []*models.File{}, nil
}

func (m *MockFileRepository) GetStarred(userID uuid.UUID, limit, offset int) ([]*models.File, error) {
	return []*models.File{}, nil
}

func (m *MockFileRepository) GetTrashed(userID uuid.UUID, limit, offset int) ([]*models.File, error) {
	return []*models.File{}, nil
}

func (m *MockFileRepository) Star(fileID, userID uuid.UUID) error {
	return nil
}

func (m *MockFileRepository) Unstar(fileID, userID uuid.UUID) error {
	return nil
}

func (m *MockFileRepository) Move(fileID, newParentID uuid.UUID) error {
	return nil
}

func (m *MockFileRepository) Copy(fileID, newParentID uuid.UUID, newName string) (*models.File, error) {
	return nil, nil
}

func (m *MockFileRepository) Rename(fileID uuid.UUID, newName string) error {
	return nil
}

func (m *MockFileRepository) UpdateMetadata(fileID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (m *MockFileRepository) GetMetadata(fileID uuid.UUID) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *MockFileRepository) VerifyIntegrity(fileID uuid.UUID) (bool, error) {
	return true, nil
}

func (m *MockFileRepository) CalculateChecksums(fileID uuid.UUID) (map[string]string, error) {
	return map[string]string{}, nil
}

// MockStorageRepository для тестирования
type MockStorageRepository struct{}

func NewMockStorageRepository() *MockStorageRepository {
	return &MockStorageRepository{}
}

func (m *MockStorageRepository) SaveFile(userID uuid.UUID, filePath string, content []byte) error {
	return nil
}

func (m *MockStorageRepository) GetFile(userID uuid.UUID, filePath string) ([]byte, error) {
	return []byte("test content"), nil
}

func (m *MockStorageRepository) DeleteFile(userID uuid.UUID, filePath string) error {
	return nil
}

func (m *MockStorageRepository) FileExists(userID uuid.UUID, filePath string) (bool, error) {
	return true, nil
}

func (m *MockStorageRepository) CreateResumableSession(userID uuid.UUID, filePath string, size int64, checksum string) (string, error) {
	return "test-session-id", nil
}

func (m *MockStorageRepository) UploadChunk(sessionID string, chunk []byte, start, end int64) error {
	return nil
}

func (m *MockStorageRepository) FinalizeUpload(sessionID string) error {
	return nil
}

func (m *MockStorageRepository) GetResumableSession(sessionID string) (*models.ResumableSession, error) {
	return &models.ResumableSession{
		ID:       sessionID,
		UserID:   uuid.New(),
		FilePath: "test/path",
		Size:     1024,
		Checksum: "test-checksum",
	}, nil
}

func (m *MockStorageRepository) DownloadChunk(sessionID string, start, end int64) ([]byte, error) {
	return []byte("test chunk"), nil
}

func (m *MockStorageRepository) CleanupExpiredSessions() error {
	return nil
}

// Простой тест для проверки компиляции
func TestFileService_Compilation(t *testing.T) {
	// Этот тест просто проверяет, что код компилируется
	t.Log("File service compiles successfully")
}
