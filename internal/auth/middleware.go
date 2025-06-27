package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"homecloud-file-service/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LoggerMiddleware создает middleware для добавления logger в контекст
func LoggerMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Добавляем logger в контекст запроса
			ctx := logger.CtxWWithLogger(r.Context(), log)
			fmt.Printf("LoggerMiddleware: Added logger to context for path: %s\n", r.URL.Path)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthMiddleware создает middleware для проверки аутентификации
func AuthMiddleware(authClient *GRPCAuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("AuthMiddleware: Processing request for path: %s\n", r.URL.Path)

			lg := logger.GetLoggerFromCtxSafe(r.Context())

			// Получаем токен из заголовка Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				fmt.Printf("AuthMiddleware: No Authorization header found\n")
				if lg != nil {
					lg.Error(r.Context(), "Authorization header is required")
				} else {
					fmt.Printf("Error: Authorization header is required\n")
				}
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			// Убираем префикс "Bearer "
			token := strings.TrimPrefix(authHeader, "Bearer ")
			fmt.Printf("AuthMiddleware: Token received: %s\n", token)

			// Тестовый режим для разработки
			if isTestToken(token) {
				userID := getTestUserID(token)
				fmt.Printf("AuthMiddleware: Using test token for userID: %s\n", userID.String())
				if lg != nil {
					lg.Info(r.Context(), "Using test token", zap.String("userID", userID.String()))
				} else {
					fmt.Printf("Info: Using test token for userID: %s\n", userID.String())
				}
				ctx := context.WithValue(r.Context(), "userID", userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Извлекаем userID из токена через auth service
			userID, err := authClient.GetUserIDFromToken(r.Context(), authHeader)
			if err != nil {
				fmt.Printf("AuthMiddleware: Failed to validate token: %v\n", err)
				if lg != nil {
					lg.Error(r.Context(), "Failed to validate token", zap.Error(err))
				} else {
					fmt.Printf("Error: Failed to validate token: %v\n", err)
				}
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			fmt.Printf("AuthMiddleware: Token validated successfully for userID: %s\n", userID.String())
			if lg != nil {
				lg.Info(r.Context(), "Token validated successfully", zap.String("userID", userID.String()))
			} else {
				fmt.Printf("Info: Token validated successfully for userID: %s\n", userID.String())
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
