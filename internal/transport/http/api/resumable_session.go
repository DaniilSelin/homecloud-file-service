package api

import (
	"fmt"
	"sync"
	"regexp"
	"github.com/google/uuid"
	"strconv"
)

// session - структура для хранения данных сессии загрузки
type uploadSession struct {
	FilePath  string
	Size      uint64
	SHA256    string
	UserID    uuid.UUID
	ParentID  *uuid.UUID
}

// Мьютекс для защиты sessionStore
var sessionMutex = sync.RWMutex{}

// sessionStore - хранилище сессий загрузки
var sessionStore = make(map[string]uploadSession)

// регулярное выражение для парсинга start и end из Content-Range
var rangeRegexRes = regexp.MustCompile(`bytes (\d+)-(\d+)/(\d+|\*)`)

// generateSessionID - генерирует уникальный идентификатор сессии
func generateSessionID() string {
	return uuid.New().String()
}

// saveSession - сохраняет информацию о сессии загрузки
func saveSession(id string, session uploadSession) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	sessionStore[id] = session
}

// getSession - получает сессию загрузки по sessionID
func getSession(sessionID string) (uploadSession, error) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	session, found := sessionStore[sessionID]
	if !found {
		return uploadSession{}, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// deleteSession - удаляет сессию загрузки
func deleteSession(sessionID string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	delete(sessionStore, sessionID)
}

// parseContentRange - извлекает начальный и конечный байты из заголовка Content-Range
func parseContentRange(rangeHeader string) (start, end uint64, err error) {
	matches := rangeRegexRes.FindStringSubmatch(rangeHeader)
	if len(matches) < 3 {
		return 0, 0, fmt.Errorf("invalid Content-Range format")
	}

	start, err = strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start value: %v", err)
	}

	end, err = strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end value: %v", err)
	}

	return start, end, nil
} 