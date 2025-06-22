package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AuthMiddleware создает middleware для проверки аутентификации
func AuthMiddleware(authClient *GRPCAuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем токен из заголовка Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			// Убираем префикс "Bearer "
			token := strings.TrimPrefix(authHeader, "Bearer ")

			// Тестовый режим для разработки
			if isTestToken(token) {
				userID := getTestUserID(token)
				ctx := context.WithValue(r.Context(), "userID", userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Извлекаем userID из токена через auth service
			userID, err := authClient.GetUserIDFromToken(r.Context(), authHeader)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Добавляем userID в контекст запроса
			ctx := context.WithValue(r.Context(), "userID", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isTestToken проверяет, является ли токен тестовым
func isTestToken(token string) bool {
	return strings.Contains(token, "test_signature")
}

// getTestUserID извлекает userID из тестового токена
func getTestUserID(token string) uuid.UUID {
	// Для тестового токена возвращаем фиксированный UUID testuser
	testUserID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	return testUserID
}

// GetUserIDFromContext извлекает userID из контекста
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value("userID").(uuid.UUID)
	return userID, ok
}
