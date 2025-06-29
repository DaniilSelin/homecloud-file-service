package api

import (
	"bytes"
	"mime"
	"context"
	_ "crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"archive/zip"

	"homecloud-file-service/internal/auth"
	"homecloud-file-service/internal/interfaces"
	"homecloud-file-service/internal/logger"
	"homecloud-file-service/internal/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	_ "go.uber.org/zap"
	"github.com/gorilla/handlers"
	"crypto/sha256"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	fileService    interfaces.FileService
	storageService interfaces.StorageService
	authClient     *auth.GRPCAuthClient
	validator      *validator.Validate
}

func NewHandler(fileService interfaces.FileService, storageService interfaces.StorageService, authClient *auth.GRPCAuthClient) *Handler {
	return &Handler{
		fileService:    fileService,
		storageService: storageService,
		authClient:     authClient,
		validator:      validator.New(),
	}
}

// SetupRoutes настраивает маршруты API
func SetupRoutes(handler *Handler, log *logger.Logger) http.Handler {
	// Инициализация маршрутизатора
	router := mux.NewRouter()

	// Health check endpoint (без аутентификации)
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/health", handler.HealthCheck).Methods("GET")

	// API v1 с аутентификацией
	api := router.PathPrefix("/api/v1").Subrouter()

	// Применяем middleware только к API маршрутам
	// Сначала добавляем logger в контекст, затем проверяем аутентификацию
	api.Use(auth.LoggerMiddleware(log))
	api.Use(auth.AuthMiddleware(handler.authClient))

	// Регистрируем обработчики для возобновляемой загрузки
	api.HandleFunc("/files/upload/resumable/{sessionID}", handler.ResumableUpload).Methods("POST", "PATCH")
	api.HandleFunc("/files/upload/resumable", handler.ResumableUploadInit).Methods("POST")

	// Регистрируем обработчики для папок
	api.HandleFunc("/folders", handler.CreateFolder).Methods("POST")
	api.HandleFunc("/folders/{id}/contents", handler.ListFolderContents).Methods("GET")
	api.HandleFunc("/folders/upload", handler.UploadFolder).Methods("POST")
	api.HandleFunc("/folders/download", handler.DownloadFolder).Methods("GET")

	// Регистрируем обработчики для файлов
	api.HandleFunc("/files/{id}/download", handler.DownloadFileByID).Methods("GET")
	api.HandleFunc("/files/{id}", handler.GetFile).Methods("GET")
	api.HandleFunc("/files/{id}", handler.DeleteFile).Methods("DELETE")
	api.HandleFunc("/files/upload", handler.UploadFile).Methods("POST")  // Для совместимости с тестами
	api.HandleFunc("/files", handler.ListFiles).Methods("GET")
	api.HandleFunc("/files", handler.CreateFile).Methods("POST")
	api.HandleFunc("/files", handler.UploadFile).Methods("PUT")  // Для совместимости с PUT запросами

	// --- CORS middleware ---
	corsMiddleware := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "Content-Range"}),
	)
	return corsMiddleware(router)
}

// Вспомогательные функции
func (h *Handler) getUserIDFromRequest(r *http.Request) (uuid.UUID, error) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, fmt.Errorf("user not found in context")
	}
	return userID, nil
}

func (h *Handler) parseUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	vars := mux.Vars(r)
	idStr := vars[param]

	// Отладочная информация
	fmt.Printf("parseUUIDParam: param='%s', idStr='%s', len=%d\n", param, idStr, len(idStr))
	fmt.Printf("parseUUIDParam: all vars=%+v\n", vars)

	if idStr == "" {
		return uuid.Nil, fmt.Errorf("parameter '%s' is empty", param)
	}

	return uuid.Parse(idStr)
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) respondWithError(w http.ResponseWriter, statusCode int, message string) {
	h.respondWithJSON(w, statusCode, map[string]string{"error": message})
}

// ensureFolderPath создает папки по указанному пути и возвращает ID последней папки
func (h *Handler) ensureFolderPath(ctx context.Context, userID uuid.UUID, folderPath string) (*uuid.UUID, error) {
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Разбиваем путь на части
	parts := strings.Split(folderPath, "/")
	var currentParentID *uuid.UUID = nil

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		// Создаем папку
		folder, err := h.fileService.CreateFolder(ctx, part, currentParentID, userID)
		if err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to create folder", zap.Error(err), zap.String("name", part))
			}
			return nil, err
		}

		currentParentID = &folder.ID
	}

	return currentParentID, nil
}

// findFolderByPath находит папку по указанному пути
func (h *Handler) findFolderByPath(ctx context.Context, userID uuid.UUID, folderPath string) (*uuid.UUID, error) {
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Если путь пустой или корневой, возвращаем nil (корневая папка)
	if folderPath == "" || folderPath == "." || folderPath == "/" {
		return nil, nil
	}

	// Разбиваем путь на части
	parts := strings.Split(folderPath, "/")
	var currentParentID *uuid.UUID = nil

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		// Получаем содержимое текущей папки
		files, err := h.fileService.ListFolderContents(ctx, currentParentID, userID)
		if err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
			}
			return nil, err
		}

		// Ищем папку с нужным именем
		var foundFolder *models.File = nil
		for _, file := range files {
			if file.Name == part && file.IsFolder {
				foundFolder = &file
				break
			}
		}

		if foundFolder == nil {
			return nil, fmt.Errorf("folder '%s' not found in path '%s'", part, folderPath)
		}

		currentParentID = &foundFolder.ID
	}

	return currentParentID, nil
}


// findFileByPath находит файл по указанному пути
func (h *Handler) findFileByPath(ctx context.Context, userID uuid.UUID, filePath string) (*models.File, error) {
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Разбиваем путь на части
	dirPath := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// Находим родительскую папку
	var parentID *uuid.UUID = nil
	if dirPath != "." && dirPath != "/" {
		var err error
		parentID, err = h.findFolderByPath(ctx, userID, dirPath)
		if err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to find parent folder", zap.Error(err), zap.String("dirPath", dirPath))
			}
			return nil, err
		}
	}

	// Получаем содержимое папки
	files, err := h.fileService.ListFolderContents(ctx, parentID, userID)
	if err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
		}
		return nil, err
	}

	// Ищем файл с нужным именем
	for _, file := range files {
		if file.Name == fileName && !file.IsFolder {
			return &file, nil
		}
	}

	return nil, fmt.Errorf("file '%s' not found in path '%s'", fileName, filePath)
}


// Структуры для запросов
type UploadRequest struct {
	FilePath string `json:"filePath"`
}

type FolderRequest struct {
	Name     string     `json:"name" validate:"required"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

type ListFolderRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Path     string     `json:"path,omitempty"`
}

type ResumableUploadRequest struct {
	FilePath string `json:"filePath" validate:"required"`
	Size     uint64 `json:"size" validate:"required,gt=0"`
	SHA256   string `json:"sha256"`
}

type DownloadRequest struct {
	FilePath string `json:"filePath"`
}

type ResumableDownloadRequest struct {
	FilePath string `json:"filePath"`
}

// Регулярные выражения для парсинга заголовков
var rangeRegex = regexp.MustCompile(`bytes (\d+)-(\d+)/(\d+|\*)`)
var rangeDownloadRegex = regexp.MustCompile(`bytes=(\d+)-(\d*)`)

// Обработчики файлов
func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "CreateFile handler called", zap.String("method", r.Method), zap.String("path", r.URL.Path))
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to get userID from request", zap.Error(err))
		}
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if lg != nil {
		lg.Info(r.Context(), "User authenticated", zap.String("userID", userID.String()))
	}

	// Парсим JSON из тела запроса
	var req models.CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to decode JSON request", zap.Error(err))
		}
		h.respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if lg != nil {
		lg.Info(r.Context(), "Request parsed successfully",
			zap.String("fileName", req.Name),
			zap.Bool("isFolder", req.IsFolder),
			zap.String("mimeType", req.MimeType),
			zap.Int64("size", req.Size),
			zap.Int("contentLength", len(req.Content)))
	}

	// Валидация
	if req.Name == "" {
		if lg != nil {
			lg.Error(r.Context(), "File name is required")
		}
		h.respondWithError(w, http.StatusBadRequest, "File name is required")
		return
	}

	// Создаем файл
	createdFile, err := h.fileService.CreateFile(r.Context(), &req, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to create file",
				zap.Error(err),
				zap.String("fileName", req.Name),
				zap.String("userID", userID.String()))
		}

		// Проверяем, является ли это ошибкой дублирования имени файла
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			// Генерируем альтернативное имя
			alternativeName := generateAlternativeName(req.Name)

			errorResponse := map[string]interface{}{
				"error": "File with this name already exists",
				"details": map[string]interface{}{
					"fileName":   req.Name,
					"suggestion": alternativeName,
					"message":    fmt.Sprintf("A file named '%s' already exists in this location. Try using '%s' instead.", req.Name, alternativeName),
				},
			}

			if lg != nil {
				lg.Info(r.Context(), "Duplicate file name detected",
					zap.String("fileName", req.Name),
					zap.String("suggestedName", alternativeName))
			}

			h.respondWithJSON(w, http.StatusConflict, errorResponse)
			return
		}

		// Возвращаем конкретную ошибку вместо общего сообщения
		h.respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create file: %v", err))
		return
	}

	if lg != nil {
		lg.Info(r.Context(), "File created successfully",
			zap.String("fileID", createdFile.ID.String()),
			zap.String("fileName", createdFile.Name),
			zap.String("userID", userID.String()))
	}

	h.respondWithJSON(w, http.StatusCreated, createdFile)
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "GetFile handler called")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем файл
	file, err := h.fileService.GetFile(r.Context(), fileID, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to get file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	h.respondWithJSON(w, http.StatusOK, file)
}

func (h *Handler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "UpdateFile handler called")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Парсим JSON из тела запроса
	var req models.UpdateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Обновляем файл
	updatedFile, err := h.fileService.UpdateFile(r.Context(), fileID, &req, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to update file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, updatedFile)
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "DeleteFile handler called")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Выполняем рекурсивное жесткое удаление
	err = h.fileService.DeleteFileRecursive(r.Context(), fileID, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to delete file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to delete file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File deleted successfully"})
}

func (h *Handler) RestoreFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "RestoreFile handler called")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Восстанавливаем файл
	err = h.fileService.RestoreFile(r.Context(), fileID, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to restore file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to restore file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File restored successfully"})
}

func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("ListFiles: Handler called for path: %s\n", r.URL.Path)

	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "ListFiles handler called")
		fmt.Printf("ListFiles: Logger available in context\n")
	} else {
		fmt.Printf("Warning: Logger not available in context for ListFiles\n")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		fmt.Printf("ListFiles: Failed to get userID from request: %v\n", err)
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	fmt.Printf("ListFiles: UserID from context: %s\n", userID.String())

	// Парсим query параметры
	query := r.URL.Query()

	// Создаем запрос
	req := &models.FileListRequest{
		OwnerID: userID,
	}

	// ParentID (опционально)
	if parentIDStr := query.Get("parent_id"); parentIDStr != "" {
		parentID, err := uuid.Parse(parentIDStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid parent_id")
			return
		}
		req.ParentID = &parentID
	}

	// IsTrashed (опционально)
	if isTrashedStr := query.Get("is_trashed"); isTrashedStr != "" {
		isTrashed := isTrashedStr == "true"
		req.IsTrashed = &isTrashed
	}

	// Starred (опционально)
	if starredStr := query.Get("starred"); starredStr != "" {
		starred := starredStr == "true"
		req.Starred = &starred
	}

	// Limit (опционально)
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid limit")
			return
		}
		req.Limit = limit
	}

	// Offset (опционально)
	if offsetStr := query.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid offset")
			return
		}
		req.Offset = offset
	}

	// OrderBy (опционально)
	if orderBy := query.Get("order_by"); orderBy != "" {
		req.OrderBy = orderBy
	}

	// OrderDir (опционально)
	if orderDir := query.Get("order_dir"); orderDir != "" {
		req.OrderDir = orderDir
	}

	// Получаем список файлов
	fmt.Printf("ListFiles: Calling fileService.ListFiles with request: %+v\n", req)
	response, err := h.fileService.ListFiles(r.Context(), req)
	if err != nil {
		fmt.Printf("ListFiles: Error from fileService.ListFiles: %v\n", err)
		if lg != nil {
			lg.Error(r.Context(), "Failed to list files", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list files")
		return
	}

	fmt.Printf("ListFiles: Successfully got response from fileService: %+v\n", response)
	h.respondWithJSON(w, http.StatusOK, response)
}

// Обработчики загрузки и скачивания (прямые маршруты как в Python)
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "UploadFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим multipart/form-data
	err = r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		lg.Error(r.Context(), "Failed to parse multipart form", zap.Error(err))
		h.respondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		lg.Error(r.Context(), "Failed to get file from form", zap.Error(err))
		h.respondWithError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	// Получаем путь к файлу из формы
	filePath := r.FormValue("filePath")
	if filePath == "" {
		// Если путь не указан, используем имя файла
		filePath = header.Filename
	}

	// Валидация пути файла
	if filePath == "" {
		h.respondWithError(w, http.StatusBadRequest, "File path cannot be empty")
		return
	}

	// Нормализуем путь
	filePath = filepath.Clean(filePath)
	if filepath.IsAbs(filePath) {
		h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
		return
	}

	// Проверяем, что путь не содержит запрещенные символы
	if strings.Contains(filePath, "..") {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file path")
		return
	}

	// Читаем содержимое файла
	content, err := io.ReadAll(file)
	if err != nil {
		lg.Error(r.Context(), "Failed to read file content", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to read file content")
		return
	}

	// Определяем MIME тип
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = getMimeTypeByExtension(filePath)
	}

	// Создаем запрос на создание файла
	createReq := &models.CreateFileRequest{
		Name:     filepath.Base(filePath),
		MimeType: mimeType,
		Size:     int64(len(content)),
		Content:  content,
		IsFolder: false,
	}

	// Если путь содержит папки, создаем их
	dirPath := filepath.Dir(filePath)
	if dirPath != "." && dirPath != "/" {
		// Создаем папки по пути
		parentID, err := h.ensureFolderPath(r.Context(), userID, dirPath)
		if err != nil {
			lg.Error(r.Context(), "Failed to create folder path", zap.Error(err), zap.String("path", dirPath))
			h.respondWithError(w, http.StatusInternalServerError, "Failed to create folder structure")
			return
		}
		createReq.ParentID = parentID
	}

	// Создаем файл в системе через fileService
	createdFile, err := h.fileService.CreateFile(r.Context(), createReq, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to create file", zap.Error(err))
		
		// Проверяем, является ли это ошибкой дублирования имени файла
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			// Генерируем альтернативное имя
			alternativeName := generateAlternativeName(createReq.Name)

			errorResponse := map[string]interface{}{
				"error": "File with this name already exists",
				"details": map[string]interface{}{
					"fileName":   createReq.Name,
					"suggestion": alternativeName,
					"message":    fmt.Sprintf("A file named '%s' already exists in this location. Try using '%s' instead.", createReq.Name, alternativeName),
				},
			}

			if lg != nil {
				lg.Info(r.Context(), "Duplicate file name detected",
					zap.String("fileName", createReq.Name),
					zap.String("suggestedName", alternativeName))
			}

			h.respondWithJSON(w, http.StatusConflict, errorResponse)
			return
		}

		h.respondWithError(w, http.StatusInternalServerError, "Failed to create file")
		return
	}

	// Отправляем успешный ответ
	h.respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "File uploaded successfully",
		"file":    createdFile,
		"path":    filePath,
	})
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
    lg := logger.GetLoggerFromCtxSafe(r.Context())
    lg.Info(r.Context(), "DownloadFile handler called")

    // Получаем userID из контекста
    userID, err := h.getUserIDFromRequest(r)
    if err != nil {
        lg.Error(r.Context(), "Unauthorized access attempt", zap.Error(err))
        h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
        return
    }

    // Получаем и валидируем путь к файлу
    filePath := r.URL.Query().Get("path")
    if filePath == "" {
        h.respondWithError(w, http.StatusBadRequest, "File path is required")
        return
    }

    // Нормализация и проверка пути
    filePath = filepath.Clean(filePath)
    if filepath.IsAbs(filePath) || strings.Contains(filePath, "..") {
        lg.Info(r.Context(), "Invalid path attempt", zap.String("path", filePath))
        h.respondWithError(w, http.StatusBadRequest, "Invalid file path")
        return
    }

    // Находим файл по пути
    file, err := h.findFileByPath(r.Context(), userID, filePath)
    if err != nil {
        lg.Error(r.Context(), "File not found", zap.Error(err), zap.String("path", filePath))
        h.respondWithError(w, http.StatusNotFound, "File not found")
        return
    }

    // Скачиваем файл по ID
    reader, filename, err := h.fileService.DownloadFile(r.Context(), file.ID, userID)
    if err != nil {
        lg.Error(r.Context(), "File download failed", zap.Error(err), zap.String("fileID", file.ID.String()))
        h.respondWithError(w, http.StatusInternalServerError, "Failed to download file")
        return
    }
    defer func() {
        if err := reader.Close(); err != nil {
            lg.Error(r.Context(), "Failed to close file reader", zap.Error(err))
        }
    }()

    // Определяем MIME-тип
    mimeType := "application/octet-stream"
    if ext := filepath.Ext(filename); ext != "" {
        mimeType = mime.TypeByExtension(ext)
    }

    // Устанавливаем заголовки
    w.Header().Set("Content-Type", mimeType)
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
    w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))

    // Копируем содержимое файла в ответ
    if _, err = io.Copy(w, reader); err != nil {
        lg.Error(r.Context(), "Failed to send file content", zap.Error(err))
        // Не отправляем повторный ответ об ошибке, так как часть данных уже могла быть отправлена
        return
    }

    lg.Info(r.Context(), "File downloaded successfully", zap.String("filename", filename))
}

// Обработчики загрузки и скачивания по ID файла
func (h *Handler) UploadFileByID(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "UploadFileByID handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Читаем содержимое файла из тела запроса
	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// Создаем reader из содержимого
	reader := bytes.NewReader(content)

	// Загружаем файл
	err = h.fileService.UploadFile(r.Context(), fileID, reader, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to upload file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File uploaded successfully"})
}

func (h *Handler) DownloadFileByID(w http.ResponseWriter, r *http.Request) {
    lg := logger.GetLoggerFromCtxSafe(r.Context())
    
    // Упрощенное логирование
    log := func(msg string) {
        if lg != nil {
            lg.Info(r.Context(), "DownloadHandler: "+msg)
        }
    }
    logError := func(err error, msg string) {
        if lg != nil {
            lg.Error(r.Context(), "DownloadHandler: "+msg, zap.Error(err))
        }
    }

    log("Handler called")

    // Получаем ID пользователя
    userID, err := h.getUserIDFromRequest(r)
    if err != nil {
        logError(err, "Failed to get user ID")
        h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
        return
    }
    log(fmt.Sprintf("User ID: %s", userID))

    // Получаем ID файла
    fileID, err := h.parseUUIDParam(r, "id")
    if err != nil {
        logError(err, "Invalid file ID")
        h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
        return
    }
    log(fmt.Sprintf("File ID: %s", fileID))

    // Получаем метаданные файла
    file, err := h.fileService.GetFile(r.Context(), fileID, userID)
    if err != nil || file == nil {
        logError(err, "File not found")
        h.respondWithError(w, http.StatusNotFound, "File not found")
        return
    }
    
    // Проверяем, что это не папка
    if file.IsFolder {
        log("Attempt to download folder")
        h.respondWithError(w, http.StatusBadRequest, "Cannot download a folder")
        return
    }
    log(fmt.Sprintf("File found: %s, Size: %d bytes", file.Name, file.Size))

    // Получаем содержимое файла
    reader, mimeType, err := h.fileService.DownloadFile(r.Context(), fileID, userID)
    if err != nil {
        logError(err, "Failed to get file content")
        h.respondWithError(w, http.StatusInternalServerError, "Failed to download file")
        return
    }

    // Устанавливаем заголовки
    w.Header().Set("Content-Type", mimeType)
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
    w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))

    // Отправляем файл
    startTime := time.Now()
    bytesCopied, err := io.Copy(w, reader)
    
    // Закрываем reader ПОСЛЕ копирования
    if closeErr := reader.Close(); closeErr != nil {
        logError(closeErr, "Failed to close reader")
    }
    
    duration := time.Since(startTime)
    
    // Логируем результат
    if err != nil {
        logError(err, fmt.Sprintf("Failed to send file. Sent: %d/%d bytes", bytesCopied, file.Size))
    } else {
        log(fmt.Sprintf("File sent successfully. Bytes: %d, Duration: %s", bytesCopied, duration))
    }
}

type httpRange struct {
	start, end int64
}

func parseRanges(rangeHeader string, fileSize int64) ([]httpRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range header format")
	}

	var ranges []httpRange
	for _, r := range strings.Split(rangeHeader[6:], ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		parts := strings.Split(r, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}

		var start, end int64
		var err error

		if parts[0] == "" {
			// suffix length (e.g., -100)
			end = fileSize - 1
			start = fileSize - 1
			if parts[1] != "" {
				if start, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
					return nil, fmt.Errorf("invalid range start")
				}
				start = fileSize - start
				if start < 0 {
					start = 0
				}
			}
		} else {
			// normal range (e.g., 100-200)
			if start, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
				return nil, fmt.Errorf("invalid range start")
			}
			if parts[1] == "" {
				end = fileSize - 1
			} else {
				if end, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
					return nil, fmt.Errorf("invalid range end")
				}
			}
		}

		if start > end {
			return nil, fmt.Errorf("invalid range: start > end")
		}

		ranges = append(ranges, httpRange{start: start, end: end})
	}

	return ranges, nil
}

func (h *Handler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "GetFileContent handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем содержимое файла
	content, err := h.fileService.GetFileContent(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get file content", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get file content")
		return
	}

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))

	// Отправляем содержимое
	w.Write(content)
}

// Возобновляемые операции
func (h *Handler) ResumableUploadInit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Получаем ID пользователя
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to get user ID", zap.Error(err))
		}
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем запрос
	var req ResumableUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to decode request", zap.Error(err))
		}
		h.respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Валидируем запрос
	if err := h.validator.Struct(req); err != nil {
		if lg != nil {
			lg.Error(ctx, "Invalid request", zap.Error(err))
		}
		h.respondWithError(w, http.StatusBadRequest, "FilePath and Size are required")
		return
	}

	// Нормализуем путь
	req.FilePath = filepath.Clean(req.FilePath)
	if filepath.IsAbs(req.FilePath) {
		h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
		return
	}

	// Проверяем, что путь не содержит запрещенные символы
	if strings.Contains(req.FilePath, "..") {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file path")
		return
	}

	// Если путь содержит папки, проверяем их существование или создаем
	dirPath := filepath.Dir(req.FilePath)
	var parentID *uuid.UUID
	if dirPath != "." && dirPath != "/" {
		var err error
		parentID, err = h.ensureFolderPath(ctx, userID, dirPath)
		if err != nil {
			lg.Error(ctx, "Failed to ensure folder path", zap.Error(err))
			h.respondWithError(w, http.StatusInternalServerError, "Failed to prepare upload location")
			return
		}
	}

	// Создаем сессию
	sessionID := uuid.New().String()
	session := uploadSession{
		FilePath:  req.FilePath,
		Size:      req.Size,
		SHA256:    req.SHA256,
		UserID:    userID,
		ParentID:  parentID,
	}

	// Сохраняем сессию
	saveSession(sessionID, session)

	// Отправляем ответ
	h.respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"session_id": sessionID,
		"upload_url": fmt.Sprintf("/upload/resumable/%s", sessionID),
	})
}

func (h *Handler) ResumableDownloadInit(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "ResumableDownloadInit handler called")
	}

	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим fileID из query параметра
	fileIDStr := r.URL.Query().Get("file_id")
	if fileIDStr == "" {
		h.respondWithError(w, http.StatusBadRequest, "file_id parameter is required")
		return
	}

	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file_id")
		return
	}

	// Инициализируем сессию
	session, err := h.fileService.InitResumableDownload(r.Context(), fileID, userID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to init resumable download", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to init resumable download")
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"session_id":   session.ID,
		"file_name":    session.FileName,
		"file_size":    session.Size,
		"checksum":     session.Checksum,
		"mime_type":    session.MimeType,
		"expires_at":   session.ExpiresAt,
		"download_url": fmt.Sprintf("/api/v1/download/resumable/%s", session.ID),
	}

	h.respondWithJSON(w, http.StatusCreated, response)
}

func (h *Handler) ResumableDownload(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "ResumableDownload handler called")
	}

	// Получаем sessionID из URL
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]
	if sessionID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	// Получаем сессию
	session, err := h.fileService.GetResumableDownloadSession(r.Context(), sessionID)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to get session", zap.Error(err))
		}
		h.respondWithError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Парсим Range заголовок
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		h.respondWithError(w, http.StatusBadRequest, "Range header is required")
		return
	}

	start, end, err := h.parseRangeHeader(rangeHeader, session.Size)
	if err != nil {
		h.respondWithError(w, http.StatusRequestedRangeNotSatisfiable, "Invalid Range header")
		return
	}

	// Получаем chunk файла
	reader, err := h.fileService.DownloadFileChunk(r.Context(), sessionID, start, end)
	if err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to get file chunk", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get file chunk")
		return
	}
	defer reader.Close()

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", session.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", session.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, session.Size))

	// Определяем статус ответа
	isComplete := end+1 >= uint64(session.Size)
	if !isComplete {
		w.WriteHeader(http.StatusPartialContent)
	}

	// Отправляем chunk
	if _, err := io.Copy(w, reader); err != nil {
		if lg != nil {
			lg.Error(r.Context(), "Failed to send chunk", zap.Error(err))
		}
	}

	// Если скачивание завершено, удаляем сессию
	if isComplete {
		h.fileService.DeleteResumableDownloadSession(r.Context(), sessionID)
	}
}

// parseRangeHeader парсит Range заголовок и возвращает start и end байты
func (h *Handler) parseRangeHeader(rangeHeader string, fileSize int64) (uint64, uint64, error) {
	// Ожидаемый формат: "bytes=start-end"
	re := regexp.MustCompile(`bytes=(\d+)-(\d*)`)
	matches := re.FindStringSubmatch(rangeHeader)
	
	if len(matches) < 2 {
		return 0, 0, fmt.Errorf("invalid range header format")
	}

	start, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start value: %w", err)
	}

	var end uint64
	if matches[2] != "" {
		end, err = strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end value: %w", err)
		}
	} else {
		// Если end не указан, берем до конца файла
		end = uint64(fileSize) - 1
	}

	// Проверяем валидность диапазона
	if start >= uint64(fileSize) || end >= uint64(fileSize) || start > end {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	return start, end, nil
}

// Остальные обработчики (заглушки)
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "CreateFolder handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем запрос
	var req FolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация имени папки
	if req.Name == "" {
		h.respondWithError(w, http.StatusBadRequest, "Folder name is required")
		return
	}

	// Проверяем, что имя не содержит запрещенные символы
	if strings.Contains(req.Name, "/") || strings.Contains(req.Name, "\\") {
		h.respondWithError(w, http.StatusBadRequest, "Folder name cannot contain path separators")
		return
	}

	// Создаем папку
	folder, err := h.fileService.CreateFolder(r.Context(), req.Name, req.ParentID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to create folder", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to create folder")
		return
	}

	h.respondWithJSON(w, http.StatusCreated, folder)
}

func (h *Handler) ListFolderContents(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "ListFolderContents handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем параметры из query
	var folderID *uuid.UUID
	var path string

	// Проверяем, есть ли ID папки в query параметрах
	if folderIDStr := r.URL.Query().Get("folder_id"); folderIDStr != "" {
		parsedID, err := uuid.Parse(folderIDStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid folder ID")
			return
		}
		folderID = &parsedID
	} else if pathParam := r.URL.Query().Get("path"); pathParam != "" {
		// Если указан путь, находим папку по пути
		path = filepath.Clean(pathParam)
		if filepath.IsAbs(path) {
			h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
			return
		}

		// Находим папку по пути
		foundFolderID, err := h.findFolderByPath(r.Context(), userID, path)
		if err != nil {
			lg.Error(r.Context(), "Failed to find folder by path", zap.Error(err), zap.String("path", path))
			h.respondWithError(w, http.StatusNotFound, "Folder not found")
			return
		}
		folderID = foundFolderID
	}

	// Получаем содержимое папки
	files, err := h.fileService.ListFolderContents(r.Context(), folderID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to list folder contents", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list folder contents")
		return
	}

	// Формируем ответ с дополнительной информацией
	response := map[string]interface{}{
		"files": files,
		"path":  path,
	}

	if folderID != nil {
		response["folder_id"] = folderID.String()
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

func (h *Handler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "SearchFiles handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем query параметр
	query := r.URL.Query().Get("q")
	if query == "" {
		h.respondWithError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	// Выполняем поиск
	files, err := h.fileService.SearchFiles(r.Context(), userID, query)
	if err != nil {
		lg.Error(r.Context(), "Failed to search files", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to search files")
		return
	}

	h.respondWithJSON(w, http.StatusOK, files)
}

func (h *Handler) ListStarredFiles(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "ListStarredFiles handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем избранные файлы
	files, err := h.fileService.ListStarredFiles(r.Context(), userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to list starred files", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list starred files")
		return
	}

	h.respondWithJSON(w, http.StatusOK, files)
}

func (h *Handler) ListTrashedFiles(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "ListTrashedFiles handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем удаленные файлы
	files, err := h.fileService.ListTrashedFiles(r.Context(), userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to list trashed files", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list trashed files")
		return
	}

	h.respondWithJSON(w, http.StatusOK, files)
}

func (h *Handler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "ListRevisions handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем ревизии
	revisions, err := h.fileService.ListRevisions(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get revisions", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get revisions")
		return
	}

	h.respondWithJSON(w, http.StatusOK, revisions)
}

func (h *Handler) GetRevision(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "GetRevision handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем ID ревизии из URL
	vars := mux.Vars(r)
	revisionIDStr := vars["revisionId"]
	revisionID, err := strconv.ParseInt(revisionIDStr, 10, 64)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid revision ID")
		return
	}

	// Получаем ревизию
	revision, err := h.fileService.GetRevision(r.Context(), fileID, revisionID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get revision", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get revision")
		return
	}

	h.respondWithJSON(w, http.StatusOK, revision)
}

func (h *Handler) RestoreRevision(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "RestoreRevision handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем ID ревизии из URL
	vars := mux.Vars(r)
	revisionIDStr := vars["revisionId"]
	revisionID, err := strconv.ParseInt(revisionIDStr, 10, 64)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid revision ID")
		return
	}

	// Восстанавливаем ревизию
	err = h.fileService.RestoreRevision(r.Context(), fileID, revisionID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to restore revision", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to restore revision")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Revision restored successfully"})
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "ListPermissions handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем права доступа
	permissions, err := h.fileService.ListPermissions(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get permissions", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get permissions")
		return
	}

	h.respondWithJSON(w, http.StatusOK, permissions)
}

func (h *Handler) GrantPermission(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "GrantPermission handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Парсим JSON запрос
	var req struct {
		GranteeID   string `json:"grantee_id"`
		GranteeType string `json:"grantee_type"`
		Role        string `json:"role"`
		AllowShare  bool   `json:"allow_share"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.GranteeID == "" || req.GranteeType == "" || req.Role == "" {
		h.respondWithError(w, http.StatusBadRequest, "Grantee ID, type and role are required")
		return
	}

	// Парсим grantee ID
	granteeID, err := uuid.Parse(req.GranteeID)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid grantee ID")
		return
	}

	// Создаем объект разрешения
	permission := &models.FilePermission{
		FileID:      fileID,
		GranteeID:   &granteeID,
		GranteeType: req.GranteeType,
		Role:        req.Role,
		AllowShare:  req.AllowShare,
	}

	// Предоставляем права доступа
	err = h.fileService.GrantPermission(r.Context(), fileID, permission, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to grant permission", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to grant permission")
		return
	}

	h.respondWithJSON(w, http.StatusCreated, map[string]string{"message": "Permission granted successfully"})
}

func (h *Handler) RevokePermission(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "RevokePermission handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем ID файла из URL
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем ID получателя из URL
	vars := mux.Vars(r)
	granteeIDStr := vars["granteeId"]
	granteeID, err := uuid.Parse(granteeIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid grantee ID")
		return
	}

	// Отзываем права доступа
	err = h.fileService.RevokePermission(r.Context(), fileID, granteeID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to revoke permission", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to revoke permission")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Permission revoked successfully"})
}

func (h *Handler) StarFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "StarFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Добавляем в избранное
	err = h.fileService.StarFile(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to star file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to star file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File starred successfully"})
}

func (h *Handler) UnstarFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "UnstarFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Удаляем из избранного
	err = h.fileService.UnstarFile(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to unstar file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to unstar file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File unstarred successfully"})
}

func (h *Handler) MoveFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "MoveFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Декодируем запрос
	var req struct {
		NewParentID *uuid.UUID `json:"new_parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Перемещаем файл
	err = h.fileService.MoveFile(r.Context(), fileID, req.NewParentID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to move file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to move file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File moved successfully"})
}

func (h *Handler) CopyFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "CopyFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Декодируем запрос
	var req struct {
		NewParentID *uuid.UUID `json:"new_parent_id"`
		NewName     string     `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Копируем файл
	copiedFile, err := h.fileService.CopyFile(r.Context(), fileID, req.NewParentID, req.NewName, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to copy file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to copy file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, copiedFile)
}

func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "RenameFile handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Декодируем запрос
	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Переименовываем файл
	err = h.fileService.RenameFile(r.Context(), fileID, req.NewName, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to rename file", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to rename file")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File renamed successfully"})
}

func (h *Handler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "GetFileMetadata handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Получаем метаданные
	metadata, err := h.fileService.GetFileMetadata(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get file metadata", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get file metadata")
		return
	}

	h.respondWithJSON(w, http.StatusOK, metadata)
}

func (h *Handler) UpdateFileMetadata(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "UpdateFileMetadata handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Декодируем метаданные
	var metadata map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid metadata format")
		return
	}

	// Обновляем метаданные
	err = h.fileService.UpdateFileMetadata(r.Context(), fileID, metadata, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to update file metadata", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update file metadata")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File metadata updated successfully"})
}

func (h *Handler) VerifyFileIntegrity(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "VerifyFileIntegrity handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Проверяем целостность
	isIntegrityVerified, err := h.fileService.VerifyFileIntegrity(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to verify file integrity", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to verify file integrity")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"is_integrity_verified": isIntegrityVerified,
	})
}

func (h *Handler) CalculateFileChecksums(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "CalculateFileChecksums handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем fileID из URL параметров
	fileID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// Вычисляем контрольные суммы
	err = h.fileService.CalculateFileChecksums(r.Context(), fileID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to calculate file checksums", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to calculate file checksums")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "File checksums calculated successfully"})
}

func (h *Handler) GetStorageInfo(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "GetStorageInfo handler called")

	// Получаем информацию о хранилище
	availableSpace, err := h.storageService.GetAvailableSpace(r.Context())
	if err != nil {
		lg.Error(r.Context(), "Failed to get storage info", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get storage info")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"available_space": availableSpace,
	})
}

func (h *Handler) CleanupStorage(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "CleanupStorage handler called")

	// Очищаем хранилище
	err := h.storageService.CleanupOrphanedFiles(r.Context())
	if err != nil {
		lg.Error(r.Context(), "Failed to cleanup storage", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to cleanup storage")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Storage cleanup completed successfully"})
}

func (h *Handler) OptimizeStorage(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "OptimizeStorage handler called")

	// Оптимизируем хранилище
	err := h.storageService.OptimizeStorage(r.Context())
	if err != nil {
		lg.Error(r.Context(), "Failed to optimize storage", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to optimize storage")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Storage optimization completed successfully"})
}

// HealthCheck проверяет состояние сервиса
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"service":   "homecloud-file-service",
		"timestamp": "2024-01-01T00:00:00Z",
	})
}

// BrowseFolder позволяет просматривать содержимое папки с дополнительной информацией
func (h *Handler) BrowseFolder(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "BrowseFolder handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем параметры из query
	folderIDStr := r.URL.Query().Get("folder_id")
	path := r.URL.Query().Get("path")

	var folderID *uuid.UUID
	var currentPath string

	if folderIDStr != "" {
		// Если указан ID папки
		parsedID, err := uuid.Parse(folderIDStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid folder ID")
			return
		}
		folderID = &parsedID
	} else if path != "" {
		// Если указан путь
		currentPath = filepath.Clean(path)
		if filepath.IsAbs(currentPath) {
			h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
			return
		}

		// Находим папку по пути
		foundFolderID, err := h.findFolderByPath(r.Context(), userID, currentPath)
		if err != nil {
			lg.Error(r.Context(), "Failed to find folder by path", zap.Error(err), zap.String("path", currentPath))
			h.respondWithError(w, http.StatusNotFound, "Folder not found")
			return
		}
		folderID = foundFolderID
	}

	// Получаем содержимое папки
	files, err := h.fileService.ListFolderContents(r.Context(), folderID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to list folder contents", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list folder contents")
		return
	}

	// Разделяем файлы и папки
	var folders []models.File
	var regularFiles []models.File

	for _, file := range files {
		if file.IsFolder {
			folders = append(folders, file)
		} else {
			regularFiles = append(regularFiles, file)
		}
	}

	// Формируем ответ с подробной информацией
	response := map[string]interface{}{
		"path":          currentPath,
		"folders":       folders,
		"files":         regularFiles,
		"total_folders": len(folders),
		"total_files":   len(regularFiles),
		"total_items":   len(files),
	}

	if folderID != nil {
		response["folder_id"] = folderID.String()
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// NavigateToPath позволяет навигировать по пути и получать информацию о каждом уровне
func (h *Handler) NavigateToPath(w http.ResponseWriter, r *http.Request) {
	lg := logger.GetLoggerFromCtxSafe(r.Context())
	lg.Info(r.Context(), "NavigateToPath handler called")

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем путь из query параметров
	path := r.URL.Query().Get("path")
	if path == "" {
		h.respondWithError(w, http.StatusBadRequest, "Path parameter is required")
		return
	}

	// Нормализуем путь
	path = filepath.Clean(path)
	if filepath.IsAbs(path) {
		h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
		return
	}

	// Разбиваем путь на части
	parts := strings.Split(path, "/")
	var breadcrumbs []map[string]interface{}
	var currentParentID *uuid.UUID = nil
	var currentPath string

	// Строим breadcrumbs
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if currentPath != "" {
			currentPath += "/"
		}
		currentPath += part

		// Получаем содержимое текущей папки
		files, err := h.fileService.ListFolderContents(r.Context(), currentParentID, userID)
		if err != nil {
			lg.Error(r.Context(), "Failed to list folder contents", zap.Error(err))
			h.respondWithError(w, http.StatusInternalServerError, "Failed to navigate path")
			return
		}

		// Ищем текущую папку
		var currentFolder *models.File = nil
		for _, file := range files {
			if file.Name == part && file.IsFolder {
				currentFolder = &file
				break
			}
		}

		if currentFolder == nil {
			// Если папка не найдена, останавливаемся
			break
		}

		// Добавляем в breadcrumbs
		breadcrumb := map[string]interface{}{
			"name":      part,
			"path":      currentPath,
			"folder_id": currentFolder.ID.String(),
			"level":     i,
		}
		breadcrumbs = append(breadcrumbs, breadcrumb)

		currentParentID = &currentFolder.ID
	}

	// Получаем содержимое конечной папки
	finalContents, err := h.fileService.ListFolderContents(r.Context(), currentParentID, userID)
	if err != nil {
		lg.Error(r.Context(), "Failed to get final folder contents", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get folder contents")
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"requested_path": path,
		"current_path":   currentPath,
		"breadcrumbs":    breadcrumbs,
		"contents":       finalContents,
		"total_items":    len(finalContents),
	}

	if currentParentID != nil {
		response["current_folder_id"] = currentParentID.String()
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// GetFileDetails получает детальную информацию о файле по пути
func (h *Handler) GetFileDetails(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("GetFileDetails: Handler called for path: %s\n", r.URL.Path)

	lg := logger.GetLoggerFromCtxSafe(r.Context())
	if lg != nil {
		lg.Info(r.Context(), "GetFileDetails handler called")
	}

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		fmt.Printf("GetFileDetails: Failed to get userID from request: %v\n", err)
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем путь к файлу из query параметра
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		fmt.Printf("GetFileDetails: No file path provided\n")
		h.respondWithError(w, http.StatusBadRequest, "File path is required")
		return
	}

	fmt.Printf("GetFileDetails: Getting details for filePath: %s\n", filePath)

	// Получаем детальную информацию о файле
	file, err := h.fileService.GetFileDetails(r.Context(), userID, filePath)
	if err != nil {
		fmt.Printf("GetFileDetails: Error from fileService: %v\n", err)
		if lg != nil {
			lg.Error(r.Context(), "Failed to get file details", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get file details")
		return
	}

	fmt.Printf("GetFileDetails: Successfully got file details\n")
	h.respondWithJSON(w, http.StatusOK, file)
}

// generateAlternativeName генерирует альтернативное имя для файла, если файл с таким именем уже существует
func generateAlternativeName(name string) string {
	ext := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, ext)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d%s", baseName, timestamp, ext)
}

func (h *Handler) ResumableUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Получаем sessionID из URL
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]
	if sessionID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	// Получаем сессию
	session, err := getSession(sessionID)
	if err != nil {
		if lg != nil {
			lg.Error(ctx, "Session not found", zap.Error(err), zap.String("sessionID", sessionID))
		}
		h.respondWithError(w, http.StatusNotFound, "Upload session not found")
		return
	}

	// Проверяем пользователя
	userID, err := h.getUserIDFromRequest(r)
	if err != nil || userID != session.UserID {
		if lg != nil {
			lg.Error(ctx, "Unauthorized upload attempt", zap.Error(err))
		}
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Проверяем заголовок Content-Range
	rangeHeader := r.Header.Get("Content-Range")
	if rangeHeader == "" {
		h.respondWithError(w, http.StatusBadRequest, "Content-Range header is required")
		return
	}

	start, end, err := parseContentRange(rangeHeader)
	total := end
	if err != nil {
		if lg != nil {
			lg.Error(ctx, "Invalid Content-Range", zap.Error(err), zap.String("range", rangeHeader))
		}
		h.respondWithError(w, http.StatusBadRequest, "Invalid Content-Range header")
		return
	}

	// Проверяем, что размер файла совпадает с размером в сессии
	if total != session.Size {
		if lg != nil {
			lg.Error(ctx, "File size mismatch",
				zap.Int("expected", int(session.Size)),
				zap.Int("actual", int(total)))
		}
		h.respondWithError(w, http.StatusBadRequest, "File size mismatch")
		return
	}

	// Создаем временный файл для загрузки
	tempDir := filepath.Join(os.TempDir(), "resumable_uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to create temp directory", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to prepare upload location")
		return
	}

	tempFilePath := filepath.Join(tempDir, sessionID)
	file, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to open temp file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process upload")
		return
	}
	defer file.Close()

	// Записываем данные в нужную позицию
	if _, err := file.Seek(int64(start), 0); err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to seek in file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process upload")
		return
	}

	if _, err := io.Copy(file, r.Body); err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to write chunk", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to process upload")
		return
	}

	// Проверяем, завершена ли загрузка
	isComplete := end+1 == session.Size

	// Синхронизируем файл
	if err := file.Sync(); err != nil {
		if lg != nil {
			lg.Error(ctx, "Failed to sync file", zap.Error(err))
		}
		h.respondWithError(w, http.StatusInternalServerError, "Failed to finalize upload")
		return
	}

	if isComplete {
		// Проверяем контрольную сумму, если она была предоставлена
		if session.SHA256 != "" {
			if err := file.Sync(); err != nil {
				if lg != nil {
					lg.Error(ctx, "Failed to sync file", zap.Error(err))
				}
				h.respondWithError(w, http.StatusInternalServerError, "Failed to finalize upload")
				return
			}

			hasher := sha256.New()
			if _, err := file.Seek(0, 0); err != nil {
				if lg != nil {
					lg.Error(ctx, "Failed to seek to start", zap.Error(err))
				}
				h.respondWithError(w, http.StatusInternalServerError, "Failed to verify upload")
				return
			}

			if _, err := io.Copy(hasher, file); err != nil {
				if lg != nil {
					lg.Error(ctx, "Failed to calculate checksum", zap.Error(err))
				}
				h.respondWithError(w, http.StatusInternalServerError, "Failed to verify upload")
				return
			}

			actualSHA256 := fmt.Sprintf("%x", hasher.Sum(nil))
			if actualSHA256 != session.SHA256 {
				if lg != nil {
					lg.Error(ctx, "Checksum mismatch",
						zap.String("expected", session.SHA256),
						zap.String("actual", actualSHA256))
				}
				os.Remove(tempFilePath)
				h.respondWithError(w, http.StatusBadRequest, "Checksum verification failed")
				return
			}
		}

		// Читаем содержимое файла
		if _, err := file.Seek(0, 0); err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to seek to start", zap.Error(err))
			}
			h.respondWithError(w, http.StatusInternalServerError, "Failed to prepare file content")
			return
		}

		content, err := io.ReadAll(file)
		if err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to read file content", zap.Error(err))
			}
			h.respondWithError(w, http.StatusInternalServerError, "Failed to prepare file content")
			return
		}

		// Определяем MIME тип
		mimeType := getMimeTypeByExtension(session.FilePath)

		// Создаем запрос на создание файла
		createReq := &models.CreateFileRequest{
			Name:     filepath.Base(session.FilePath),
			MimeType: mimeType,
			Size:     int64(session.Size),
			Content:  content,
			IsFolder: false,
			ParentID: session.ParentID,
		}

		// Создаем файл в системе через fileService
		createdFile, err := h.fileService.CreateFile(ctx, createReq, session.UserID)
		if err != nil {
			if lg != nil {
				lg.Error(ctx, "Failed to create file", zap.Error(err))
			}

			// Проверяем, является ли это ошибкой дублирования имени файла
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				// Генерируем альтернативное имя
				alternativeName := generateAlternativeName(createReq.Name)

				errorResponse := map[string]interface{}{
					"error": "File with this name already exists",
					"details": map[string]interface{}{
						"fileName":   createReq.Name,
						"suggestion": alternativeName,
						"message":    fmt.Sprintf("A file named '%s' already exists in this location. Try using '%s' instead.", createReq.Name, alternativeName),
					},
				}

				if lg != nil {
					lg.Info(ctx, "Duplicate file name detected",
						zap.String("fileName", createReq.Name),
						zap.String("suggestedName", alternativeName))
				}

				h.respondWithJSON(w, http.StatusConflict, errorResponse)
				return
			}

			h.respondWithError(w, http.StatusInternalServerError, "Failed to create file")
			return
		}

		// Удаляем временный файл и сессию
		os.Remove(tempFilePath)
		deleteSession(sessionID)

		h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Upload completed successfully",
			"file":    createdFile,
		})
	} else {
		// Загрузка не завершена, отправляем статус успешной обработки части
		h.respondWithJSON(w, http.StatusAccepted, map[string]interface{}{
			"message": "Chunk uploaded successfully",
			"range":   rangeHeader,
		})
	}
}

var (
	sessions = make(map[string]uploadSession)
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

// UploadFolder обрабатывает загрузку папки в виде ZIP архива
func (h *Handler) UploadFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Парсим multipart/form-data
	err = r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		lg.Error(ctx, "Failed to parse multipart form", zap.Error(err))
		h.respondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Получаем ZIP файл из формы
	file, header, err := r.FormFile("folder")
	if err != nil {
		lg.Error(ctx, "Failed to get folder from form", zap.Error(err))
		h.respondWithError(w, http.StatusBadRequest, "No folder provided")
		return
	}
	defer file.Close()

	// Получаем путь к папке из формы
	folderPath := r.FormValue("folderPath")
	if folderPath == "" {
		// Если путь не указан, используем имя файла без расширения
		folderPath = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Валидация пути папки
	if folderPath == "" {
		h.respondWithError(w, http.StatusBadRequest, "Folder path cannot be empty")
		return
	}

	// Нормализуем путь
	folderPath = filepath.Clean(folderPath)
	if filepath.IsAbs(folderPath) {
		h.respondWithError(w, http.StatusBadRequest, "Absolute paths are not allowed")
		return
	}

	// Проверяем, что путь не содержит запрещенные символы
	if strings.Contains(folderPath, "..") {
		h.respondWithError(w, http.StatusBadRequest, "Invalid folder path")
		return
	}

	// Создаем корневую папку
	rootFolder, err := h.fileService.CreateFolder(ctx, filepath.Base(folderPath), nil, userID)
	if err != nil {
		lg.Error(ctx, "Failed to create root folder", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to create folder structure")
		return
	}

	// Читаем ZIP архив
	zipReader, err := zip.NewReader(file, header.Size)
	if err != nil {
		lg.Error(ctx, "Failed to read ZIP archive", zap.Error(err))
		h.respondWithError(w, http.StatusBadRequest, "Invalid ZIP archive")
		return
	}

	// Создаем структуру папок и файлов
	for _, zipFile := range zipReader.File {
		// Пропускаем директории, они будут созданы автоматически
		if zipFile.FileInfo().IsDir() {
			continue
		}

		// Получаем содержимое файла
		rc, err := zipFile.Open()
		if err != nil {
			lg.Error(ctx, "Failed to open file in ZIP", zap.Error(err), zap.String("file", zipFile.Name))
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			lg.Error(ctx, "Failed to read file content", zap.Error(err), zap.String("file", zipFile.Name))
			continue
		}

		// Определяем путь к файлу относительно корневой папки
		relPath := filepath.Join(folderPath, zipFile.Name)
		dirPath := filepath.Dir(relPath)

		// Создаем структуру папок
		var parentID *uuid.UUID
		if dirPath != "." && dirPath != "/" {
			parentID, err = h.ensureFolderPath(ctx, userID, dirPath)
			if err != nil {
				lg.Error(ctx, "Failed to create folder path", zap.Error(err), zap.String("path", dirPath))
				continue
			}
		} else {
			parentID = &rootFolder.ID
		}

		// Создаем файл
		createReq := &models.CreateFileRequest{
			Name:     filepath.Base(zipFile.Name),
			MimeType: getMimeTypeByExtension(zipFile.Name),
			Size:     int64(len(content)),
			Content:  content,
			IsFolder: false,
			ParentID: parentID,
		}

		_, err = h.fileService.CreateFile(ctx, createReq, userID)
		if err != nil {
			lg.Error(ctx, "Failed to create file", zap.Error(err), zap.String("file", zipFile.Name))
			continue
		}
	}

	h.respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Folder uploaded successfully",
		"folder":  rootFolder,
	})
}

// DownloadFolder обрабатывает скачивание папки в виде ZIP архива
func (h *Handler) DownloadFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lg := logger.GetLoggerFromCtxSafe(ctx)

	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Получаем путь к папке
	folderPath := r.URL.Query().Get("path")
	if folderPath == "" {
		h.respondWithError(w, http.StatusBadRequest, "Folder path is required")
		return
	}

	// Нормализация и проверка пути
	folderPath = filepath.Clean(folderPath)
	if filepath.IsAbs(folderPath) || strings.Contains(folderPath, "..") {
		h.respondWithError(w, http.StatusBadRequest, "Invalid folder path")
		return
	}

	// Находим папку по пути
	folder, err := h.findFileByPath(ctx, userID, folderPath)
	if err != nil {
		lg.Error(ctx, "Folder not found", zap.Error(err))
		h.respondWithError(w, http.StatusNotFound, "Folder not found")
		return
	}

	if !folder.IsFolder {
		h.respondWithError(w, http.StatusBadRequest, "Specified path is not a folder")
		return
	}

	// Получаем список всех файлов в папке
	files, err := h.fileService.ListFolderContents(ctx, &folder.ID, userID)
	if err != nil {
		lg.Error(ctx, "Failed to list folder contents", zap.Error(err))
		h.respondWithError(w, http.StatusInternalServerError, "Failed to prepare folder contents")
		return
	}

	// Создаем ZIP архив
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", folder.Name))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Добавляем файлы в архив
	for _, file := range files {
		if file.IsFolder {
			continue
		}

		// Создаем файл в архиве
		writer, err := zipWriter.Create(file.Name)
		if err != nil {
			lg.Error(ctx, "Failed to create file in ZIP", zap.Error(err))
			continue
		}

		// Получаем содержимое файла
		content, err := h.fileService.GetFileContent(ctx, file.ID, userID)
		if err != nil {
			lg.Error(ctx, "Failed to get file content", zap.Error(err))
			continue
		}

		// Записываем содержимое в архив
		if _, err := writer.Write(content); err != nil {
			lg.Error(ctx, "Failed to write file content to ZIP", zap.Error(err))
			continue
		}
	}
}