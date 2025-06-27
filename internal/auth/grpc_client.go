package auth

import (
	"context"
	"fmt"
	"strings"

	"homecloud-file-service/config"
	"homecloud-file-service/internal/logger"
	"homecloud-file-service/internal/transport/grpc/protos"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCAuthClient gRPC клиент для работы с auth сервисом
type GRPCAuthClient struct {
	client protos.AuthServiceClient
	conn   *grpc.ClientConn
}

// NewGRPCAuthClient создает новый gRPC клиент для auth сервиса
func NewGRPCAuthClient(cfg *config.Config) (*GRPCAuthClient, error) {
	// Формируем адрес auth сервиса
	authAddr := fmt.Sprintf("%s:%d", cfg.Auth.Host, cfg.Auth.Port)

	fmt.Printf("Creating gRPC auth client for address: %s\n", authAddr)

	// Устанавливаем соединение
	conn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Failed to connect to auth service at %s: %v\n", authAddr, err)
		return nil, fmt.Errorf("failed to connect to auth service at %s: %w", authAddr, err)
	}

	fmt.Printf("Successfully connected to auth service at %s\n", authAddr)

	// Создаем клиент
	client := protos.NewAuthServiceClient(conn)

	return &GRPCAuthClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close закрывает соединение с auth сервисом
func (c *GRPCAuthClient) Close() error {
	return c.conn.Close()
}

// ValidateToken проверяет токен и возвращает информацию о пользователе
func (c *GRPCAuthClient) ValidateToken(ctx context.Context, token string) (*protos.AuthUser, error) {
	lg := logger.GetLoggerFromCtx(ctx)
	if lg == nil {
		// Fallback логирование если logger недоступен
		fmt.Printf("Warning: Logger not available in context for ValidateToken\n")
	} else {
		lg.Info(ctx, "ValidateToken called", zap.String("token", token))
	}

	fmt.Printf("ValidateToken: Starting validation for token: %s\n", token[:20]+"...")

	// Проверяем, что клиент доступен
	if c.client == nil {
		fmt.Printf("ValidateToken: Auth service client is not available\n")
		return nil, fmt.Errorf("auth service client is not available")
	}

	// Убираем префикс "Bearer " если есть
	token = strings.TrimPrefix(token, "Bearer ")
	fmt.Printf("ValidateToken: Token after trimming: %s\n", token[:20]+"...")

	// Вызываем gRPC метод
	fmt.Printf("ValidateToken: Calling gRPC ValidateToken method\n")
	resp, err := c.client.ValidateToken(ctx, &protos.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		fmt.Printf("ValidateToken: gRPC call failed: %v\n", err)
		if lg != nil {
			lg.Error(ctx, "failed to validate token", zap.Error(err))
		} else {
			fmt.Printf("Error: failed to validate token: %v\n", err)
		}
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	fmt.Printf("ValidateToken: gRPC call successful, user ID: %s\n", resp.User.Id)
	return resp.User, nil
}

// GetUserProfile получает профиль пользователя по ID
func (c *GRPCAuthClient) GetUserProfile(ctx context.Context, userID uuid.UUID) (*protos.AuthUser, error) {
	// Вызываем gRPC метод
	resp, err := c.client.GetUserProfile(ctx, &protos.GetUserProfileRequest{
		UserId: userID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return resp.User, nil
}

// GetUserIDFromToken извлекает userID из токена
func (c *GRPCAuthClient) GetUserIDFromToken(ctx context.Context, token string) (uuid.UUID, error) {
	// Если auth service недоступен, используем тестовый режим
	if c.client == nil {
		return c.getTestUserIDFromToken(token)
	}

	user, err := c.ValidateToken(ctx, token)
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(user.Id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	return userID, nil
}

// getTestUserIDFromToken извлекает userID из тестового токена
func (c *GRPCAuthClient) getTestUserIDFromToken(token string) (uuid.UUID, error) {
	// Убираем префикс "Bearer "
	token = strings.TrimPrefix(token, "Bearer ")

	// Проверяем, является ли это тестовым токеном
	if strings.Contains(token, "test_signature") {
		// Для тестового токена возвращаем фиксированный UUID testuser
		testUserID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
		return testUserID, nil
	}

	return uuid.Nil, fmt.Errorf("invalid test token")
}
