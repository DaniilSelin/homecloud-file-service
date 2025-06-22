package auth

import (
	"context"
	"fmt"
	"strings"

	pb "homecloud-file-service/internal/transport/grpc/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthService представляет сервис аутентификации
type AuthService struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(host string, port int) (*AuthService, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	client := pb.NewAuthServiceClient(conn)

	return &AuthService{
		client: client,
		conn:   conn,
	}, nil
}

// ValidateToken проверяет токен и возвращает информацию о пользователе
func (a *AuthService) ValidateToken(ctx context.Context, token string) (*pb.AuthUser, error) {
	// Убираем префикс "Bearer " если он есть
	token = strings.TrimPrefix(token, "Bearer ")

	req := &pb.ValidateTokenRequest{
		Token: token,
	}

	resp, err := a.client.ValidateToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return resp.User, nil
}

// GetUserID извлекает ID пользователя из токена
func (a *AuthService) GetUserID(ctx context.Context, token string) (string, error) {
	userInfo, err := a.ValidateToken(ctx, token)
	if err != nil {
		return "", err
	}

	return userInfo.Id, nil
}

// Close закрывает соединение с auth сервисом
func (a *AuthService) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}
