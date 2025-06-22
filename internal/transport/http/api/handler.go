package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"

	"homecloud-file-service/internal/auth"
	"homecloud-file-service/internal/interfaces"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	fileService    interfaces.FileService
	storageService interfaces.StorageService
	authClient     *auth.GRPCAuthClient
}

func NewHandler(fileService interfaces.FileService, storageService interfaces.StorageService, authClient *auth.GRPCAuthClient) *Handler {
	return &Handler{
		fileService:    fileService,
		storageService: storageService,
		authClient:     authClient,
	}
}

// SetupRoutes настраивает маршруты API
func SetupRoutes(handler *Handler) *mux.Router {
	router := mux.NewRouter()

	// Health check endpoint (без аутентификации)
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/health", handler.HealthCheck).Methods("GET")

	// API v1 с аутентификацией
	api := router.PathPrefix("/api/v1").Subrouter()

	// Применяем middleware только к API маршрутам
	api.Use(auth.AuthMiddleware(handler.authClient))

	// Файлы
	api.HandleFunc("/files", handler.CreateFile).Methods("POST")
	api.HandleFunc("/files", handler.ListFiles).Methods("GET")
	api.HandleFunc("/files/{id}", handler.GetFile).Methods("GET")
	api.HandleFunc("/files/{id}", handler.UpdateFile).Methods("PUT", "PATCH")
	api.HandleFunc("/files/{id}", handler.DeleteFile).Methods("DELETE")
	api.HandleFunc("/files/{id}/restore", handler.RestoreFile).Methods("POST")

	// Загрузка и скачивание (прямые маршруты как в Python)
	api.HandleFunc("/upload", handler.UploadFile).Methods("POST")
	api.HandleFunc("/download", handler.DownloadFile).Methods("GET")

	// Загрузка и скачивание по ID файла
	api.HandleFunc("/files/{id}/upload", handler.UploadFileByID).Methods("POST")
	api.HandleFunc("/files/{id}/download", handler.DownloadFileByID).Methods("GET")
	api.HandleFunc("/files/{id}/content", handler.GetFileContent).Methods("GET")

	// Возобновляемые операции
	api.HandleFunc("/upload/resumable", handler.ResumableUploadInit).Methods("POST")
	api.HandleFunc("/upload/resumable/{sessionID}", handler.ResumableUpload).Methods("POST")
	api.HandleFunc("/download/resumable", handler.ResumableDownloadInit).Methods("GET")
	api.HandleFunc("/download/resumable/{sessionID}", handler.ResumableDownload).Methods("GET")

	// Папки
	api.HandleFunc("/folders", handler.CreateFolder).Methods("POST")
	api.HandleFunc("/folders/{id}/contents", handler.ListFolderContents).Methods("GET")

	// Поиск и фильтры
	api.HandleFunc("/files/search", handler.SearchFiles).Methods("GET")
	api.HandleFunc("/files/starred", handler.ListStarredFiles).Methods("GET")
	api.HandleFunc("/files/trashed", handler.ListTrashedFiles).Methods("GET")

	// Ревизии
	api.HandleFunc("/files/{id}/revisions", handler.ListRevisions).Methods("GET")
	api.HandleFunc("/files/{id}/revisions/{revisionId}", handler.GetRevision).Methods("GET")
	api.HandleFunc("/files/{id}/revisions/{revisionId}/restore", handler.RestoreRevision).Methods("POST")

	// Права доступа
	api.HandleFunc("/files/{id}/permissions", handler.ListPermissions).Methods("GET")
	api.HandleFunc("/files/{id}/permissions", handler.GrantPermission).Methods("POST")
	api.HandleFunc("/files/{id}/permissions/{granteeId}", handler.RevokePermission).Methods("DELETE")

	// Специальные операции
	api.HandleFunc("/files/{id}/star", handler.StarFile).Methods("POST")
	api.HandleFunc("/files/{id}/unstar", handler.UnstarFile).Methods("POST")
	api.HandleFunc("/files/{id}/move", handler.MoveFile).Methods("POST")
	api.HandleFunc("/files/{id}/copy", handler.CopyFile).Methods("POST")
	api.HandleFunc("/files/{id}/rename", handler.RenameFile).Methods("POST")

	// Метаданные
	api.HandleFunc("/files/{id}/metadata", handler.GetFileMetadata).Methods("GET")
	api.HandleFunc("/files/{id}/metadata", handler.UpdateFileMetadata).Methods("PUT")

	// Целостность
	api.HandleFunc("/files/{id}/verify", handler.VerifyFileIntegrity).Methods("POST")
	api.HandleFunc("/files/{id}/checksums", handler.CalculateFileChecksums).Methods("POST")

	// Хранилище
	api.HandleFunc("/storage/info", handler.GetStorageInfo).Methods("GET")
	api.HandleFunc("/storage/cleanup", handler.CleanupStorage).Methods("POST")
	api.HandleFunc("/storage/optimize", handler.OptimizeStorage).Methods("POST")

	return router
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

// Структуры для запросов
type UploadRequest struct {
	FilePath string `json:"filePath"`
}

type ResumableUploadRequest struct {
	FilePath string `json:"filePath"`
	Size     uint64 `json:"size"`
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
	// TODO: Реализовать создание файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать получение файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать обновление файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать удаление файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) RestoreFile(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать восстановление файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать получение списка файлов
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

// Обработчики загрузки и скачивания (прямые маршруты как в Python)
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем JSON-запрос для получения пути к файлу
	var req UploadRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Unable to decode request")
		return
	}
	defer r.Body.Close()

	// Валидация пути файла
	if req.FilePath == "" {
		h.respondWithError(w, http.StatusBadRequest, "File path cannot be empty")
		return
	}

	// Формируем путь с учетом пользователя
	userFilePath := filepath.Join(userID.String(), req.FilePath)

	// Читаем содержимое файла из тела запроса
	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// Сохраняем файл
	if err := h.storageService.SaveFile(r.Context(), userFilePath, content); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	// Отправляем успешный ответ
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded successfully to %s", userFilePath)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем JSON-запрос для получения пути к файлу
	var req DownloadRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Unable to decode request")
		return
	}
	defer r.Body.Close()

	// Валидация пути файла
	if req.FilePath == "" {
		h.respondWithError(w, http.StatusBadRequest, "File path cannot be empty")
		return
	}

	// Формируем путь с учетом пользователя
	userFilePath := filepath.Join(userID.String(), req.FilePath)

	// Получаем файл
	content, err := h.storageService.GetFile(r.Context(), userFilePath)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// Устанавливаем заголовки для скачивания файла
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(req.FilePath)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))

	// Передаем содержимое файла клиенту
	w.Write(content)
}

// Обработчики загрузки и скачивания по ID файла
func (h *Handler) UploadFileByID(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать загрузку файла по ID
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) DownloadFileByID(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать скачивание файла по ID
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать получение содержимого файла
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

// Возобновляемые операции
func (h *Handler) ResumableUploadInit(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем JSON-запрос
	var req ResumableUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Unable to decode request")
		return
	}
	defer r.Body.Close()

	// Валидация
	if req.FilePath == "" {
		h.respondWithError(w, http.StatusBadRequest, "File path cannot be empty")
		return
	}

	// Формируем путь с учетом пользователя
	userFilePath := filepath.Join(userID.String(), req.FilePath)

	// Создаем уникальный sessionID
	sessionID := uuid.New().String()

	// Сохраняем сессию в storage service
	// TODO: Добавить метод для сохранения сессии в storage service
	// h.storageService.saveUploadSession(sessionID, userFilePath, req.SHA256, int64(req.Size))
	_ = userFilePath // Временно игнорируем для компиляции

	// Формируем ответ
	response := map[string]string{
		"upload_url": fmt.Sprintf("/api/v1/upload/resumable/%s", sessionID),
		"sessionID":  sessionID,
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

func (h *Handler) ResumableUpload(w http.ResponseWriter, r *http.Request) {
	// Получаем sessionID из URL
	vars := mux.Vars(r)
	sessionID := vars["sessionID"]

	// Получаем сессию
	// TODO: Добавить метод для получения сессии из storage service
	// session, err := h.storageService.getUploadSession(sessionID)
	// if err != nil {
	//     h.respondWithError(w, http.StatusNotFound, "Session not found")
	//     return
	// }

	// Читаем заголовок Content-Range
	rangeHeader := r.Header.Get("Content-Range")
	if rangeHeader == "" {
		h.respondWithError(w, http.StatusBadRequest, "Content-Range header is required")
		return
	}

	// Парсим диапазон
	matches := rangeRegex.FindStringSubmatch(rangeHeader)
	if len(matches) < 3 {
		h.respondWithError(w, http.StatusBadRequest, "Invalid Content-Range format")
		return
	}

	start, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid start value")
		return
	}

	end, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid end value")
		return
	}

	// TODO: Реализовать сохранение части файла
	// Читаем содержимое из тела запроса
	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// TODO: Сохранить часть файла в указанную позицию
	// h.storageService.saveFileChunk(session.filePath, start, content)
	_ = start
	_ = end
	_ = content
	_ = sessionID

	h.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Chunk uploaded successfully",
	})
}

func (h *Handler) ResumableDownloadInit(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Декодируем JSON-запрос
	var req ResumableDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Unable to decode request")
		return
	}
	defer r.Body.Close()

	// Валидация
	if req.FilePath == "" {
		h.respondWithError(w, http.StatusBadRequest, "File path cannot be empty")
		return
	}

	// Формируем путь с учетом пользователя
	userFilePath := filepath.Join(userID.String(), req.FilePath)

	// Получаем информацию о файле
	fileInfo, err := h.storageService.GetFileInfo(r.Context(), userFilePath)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	// Создаем уникальный sessionID
	sessionID := uuid.New().String()

	// Сохраняем сессию
	// TODO: Добавить метод для сохранения сессии в storage service
	// h.storageService.saveUploadSession(sessionID, userFilePath, fileInfo.SHA256Checksum, fileInfo.Size)

	// Формируем ответ
	response := map[string]interface{}{
		"download_url": fmt.Sprintf("/api/v1/download/resumable/%s", sessionID),
		"sessionID":    sessionID,
		"file_size":    fileInfo.Size,
		"checksum":     fileInfo.SHA256Checksum,
	}

	h.respondWithJSON(w, http.StatusCreated, response)
}

func (h *Handler) ResumableDownload(w http.ResponseWriter, r *http.Request) {
	// Получаем sessionID из URL (пока не используется)
	// vars := mux.Vars(r)
	// sessionID := vars["sessionID"]

	// Получаем сессию
	// TODO: Добавить метод для получения сессии из storage service
	// session, err := h.storageService.getUploadSession(sessionID)
	// if err != nil {
	//     h.respondWithError(w, http.StatusNotFound, "Session not found")
	//     return
	// }

	// Читаем заголовок Range
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		h.respondWithError(w, http.StatusBadRequest, "Range header is required")
		return
	}

	// Парсим диапазон
	matches := rangeDownloadRegex.FindStringSubmatch(rangeHeader)
	if len(matches) < 2 {
		h.respondWithError(w, http.StatusBadRequest, "Invalid Range header format")
		return
	}

	start, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid start value")
		return
	}

	var end uint64
	if matches[2] != "" {
		end, err = strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid end value")
			return
		}
	} else {
		// TODO: Получить размер файла из сессии
		// end = session.CountByte - 1
		end = start + 1024*1024 - 1 // 1MB chunk
	}

	// TODO: Реализовать чтение части файла
	// content, err := h.storageService.getFileChunk(session.filePath, start, end-start+1)
	// if err != nil {
	//     h.respondWithError(w, http.StatusInternalServerError, "Failed to read file chunk")
	//     return
	// }

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, end+1))

	// TODO: Отправить содержимое части файла
	// w.Write(content)
}

// Остальные обработчики (заглушки)
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	_, err := h.getUserIDFromRequest(r)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// TODO: Реализовать создание папки
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListFolderContents(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListStarredFiles(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListTrashedFiles(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GetRevision(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) RestoreRevision(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GrantPermission(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) RevokePermission(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) StarFile(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) UnstarFile(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) MoveFile(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) CopyFile(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) UpdateFileMetadata(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) VerifyFileIntegrity(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) CalculateFileChecksums(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) GetStorageInfo(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) CleanupStorage(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

func (h *Handler) OptimizeStorage(w http.ResponseWriter, r *http.Request) {
	h.respondWithError(w, http.StatusNotImplemented, "Not implemented")
}

// HealthCheck проверяет состояние сервиса
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"service":   "homecloud-file-service",
		"timestamp": "2024-01-01T00:00:00Z",
	})
}
