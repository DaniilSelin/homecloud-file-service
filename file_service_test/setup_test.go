package file_service_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const (
	authServiceURL = "http://localhost:8080/api/v1"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

var (
	testToken string
	testUser  = RegisterRequest{
		Username: "test_user",
		Password: "test_password",
		Email:    "test@example.com",
	}
)

func TestMain(m *testing.M) {
	// Setup test environment
	if err := setupTestUser(); err != nil {
		fmt.Printf("Failed to setup test user: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup can be added here if needed
	os.Exit(code)
}

func setupTestUser() error {
	// Try to register the test user
	registerResp, err := http.Post(
		authServiceURL+"/auth/register",
		"application/json",
		createJSONReader(testUser),
	)
	if err != nil {
		return fmt.Errorf("failed to register test user: %v", err)
	}
	defer registerResp.Body.Close()

	// If user already exists or registration successful, try to login
	loginReq := LoginRequest{
		Email:    testUser.Email,
		Password: testUser.Password,
	}

	loginResp, err := http.Post(
		authServiceURL+"/auth/login",
		"application/json",
		createJSONReader(loginReq),
	)
	if err != nil {
		return fmt.Errorf("failed to login test user: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status: %d", loginResp.StatusCode)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode login response: %v", err)
	}

	// Set the test token for use in tests
	testToken = authResp.Token
	os.Setenv("TEST_USER_ID", authResp.UserID)
	os.Setenv("TEST_TOKEN", testToken)

	return nil
}

func createJSONReader(v interface{}) io.Reader {
	data, _ := json.Marshal(v)
	return bytes.NewReader(data)
} 