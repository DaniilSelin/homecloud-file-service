package errdefs

import (
	"errors"
	"fmt"
)

var (
	// Общие ошибки
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal server error")

	// Ошибки файлового сервиса
	ErrFileNotFound     = errors.New("file not found")
	ErrFileTooLarge     = errors.New("file too large")
	ErrInvalidFileType  = errors.New("invalid file type")
	ErrStorageFull      = errors.New("storage is full")
	ErrFileCorrupted    = errors.New("file is corrupted")
	ErrPermissionDenied = errors.New("permission denied")
	ErrFileExists       = errors.New("file already exists")
	ErrInvalidPath      = errors.New("invalid path")
	ErrQuotaExceeded    = errors.New("quota exceeded")
	ErrFileInUse        = errors.New("file is in use")
	ErrRevisionNotFound = errors.New("revision not found")

	// Ошибки на уровне БД (repository)
	ErrDB = errors.New("database error")

	// Ошибки слоя бизнес логики (service)
	ErrFileOperationFailed = errors.New("file operation failed")
	ErrChecksumMismatch    = errors.New("checksum mismatch")
)

// New создает новую ошибку
func New(text string) error {
	return errors.New(text)
}

// Wrap оборачивает ошибку с контекстом (аналог fmt.Errorf с %w)
func Wrap(err error, context string) error {
	return fmt.Errorf("%s: %w", context, err)
}

// Wrapf оборачивает ошибку с форматированием
func Wrapf(err error, format string, args ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is проверяет соответствие ошибки (аналог errors.Is)
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As извлекает конкретный тип ошибки (аналог errors.As)
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}
