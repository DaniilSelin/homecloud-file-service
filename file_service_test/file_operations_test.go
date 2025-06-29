package file_service_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8082/api/v1"
)

var (
	testFolderID string
	testFileID   string
)

type FileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsFolder  bool      `json:"is_folder"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FilesListResponse struct {
	Files []FileResponse `json:"files"`
	Total int           `json:"total"`
}

type CreateFileRequest struct {
	Name     string  `json:"name"`
	Content  string  `json:"content,omitempty"`
	MimeType string  `json:"mime_type,omitempty"`
	Size     int64   `json:"size,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
	IsFolder bool    `json:"is_folder"`
}

type CreateFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

func makeRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			reqBody = bytes.NewReader(v)
			fmt.Printf("Making %s request to %s with binary data\n", method, path)
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			fmt.Printf("Making %s request to %s\nRequest body: %s\n", method, path, string(jsonBody))
			reqBody = bytes.NewReader(jsonBody)
		}
	} else {
		fmt.Printf("Making %s request to %s without body\n", method, path)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	token := os.Getenv("TEST_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TEST_TOKEN environment variable is not set")
	}
	req.Header.Set("Authorization", "Bearer "+token)

	if body != nil {
		switch body.(type) {
		case []byte:
			req.Header.Set("Content-Type", "application/octet-stream")
			if method == "PATCH" {
				// Для чанков добавляем заголовок Content-Range
				size := len(body.([]byte))
				req.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", size-1, size))
			}
		default:
			req.Header.Set("Content-Type", "application/json")
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Debug response
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response body: %s\n", string(body))
		resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset the body for subsequent reading
	}

	return resp, nil
}

// Helper function to read and parse response body
func parseResponse(resp *http.Response, v interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.Unmarshal(body, v)
}

func parseErrorResponse(resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read error response: %v", err)
	}
	// Восстанавливаем тело ответа для последующего чтения
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	var errorResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return fmt.Sprintf("failed to parse error response: %v, body: %s", err, string(body))
	}
	return errorResp.Error
}

func TestFileOperations(t *testing.T) {
	// Test variables
	timestamp := time.Now().UnixNano()
	folderName := fmt.Sprintf("test-folder-%d", timestamp)
	fileName := "test-file.txt"
	fileContent := "Hello, World!"

	// 1. Create a folder
	t.Run("Create Folder", func(t *testing.T) {
		createReq := CreateFileRequest{
			Name:     folderName,
			IsFolder: true,
			MimeType: "application/x-directory",
		}

		resp, err := makeRequest(http.MethodPost, "/files", createReq)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusCreated, resp.StatusCode, parseErrorResponse(resp))
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Logf("Create Folder Response: %s", string(body))
		resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset the body for subsequent reading

		var folder FileResponse
		err = parseResponse(resp, &folder)
		require.NoError(t, err)
		assert.Equal(t, folderName, folder.Name)
		assert.True(t, folder.IsFolder)

		t.Logf("Created folder ID: %s", folder.ID)
		testFolderID = folder.ID
		require.NotEmpty(t, testFolderID, "Failed to store folder ID")
	})

	// 2. Create a file in the folder
	t.Run("Create File in Folder", func(t *testing.T) {
		require.NotEmpty(t, testFolderID, "Folder ID should be set from previous test")

		content := base64.StdEncoding.EncodeToString([]byte(fileContent))
		createReq := CreateFileRequest{
			Name:     fileName,
			Content:  content,
			MimeType: "text/plain",
			Size:     int64(len(fileContent)),
			ParentID: &testFolderID,
			IsFolder: false,
		}

		resp, err := makeRequest(http.MethodPost, "/files", createReq)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusCreated, resp.StatusCode, parseErrorResponse(resp))
		}

		var file FileResponse
		err = parseResponse(resp, &file)
		require.NoError(t, err)
		assert.Equal(t, fileName, file.Name)
		assert.False(t, file.IsFolder)
		assert.Equal(t, testFolderID, *file.ParentID)

		testFileID = file.ID
		require.NotEmpty(t, testFileID, "Failed to store file ID")
	})

	// 3. List files in folder
	t.Run("List Files in Folder", func(t *testing.T) {
		require.NotEmpty(t, testFolderID, "Folder ID should be set from previous test")

		resp, err := makeRequest(http.MethodGet, fmt.Sprintf("/files?parent_id=%s", testFolderID), nil)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusOK, resp.StatusCode, parseErrorResponse(resp))
		}

		var listResp FilesListResponse
		err = parseResponse(resp, &listResp)
		require.NoError(t, err)
		assert.Equal(t, 1, listResp.Total)
		assert.Len(t, listResp.Files, 1)
		assert.Equal(t, fileName, listResp.Files[0].Name)
	})

	// 4. Get file details
	t.Run("Get File Details", func(t *testing.T) {
		require.NotEmpty(t, testFileID, "File ID should be set from previous test")

		resp, err := makeRequest(http.MethodGet, fmt.Sprintf("/files/%s", testFileID), nil)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusOK, resp.StatusCode, parseErrorResponse(resp))
		}

		var file FileResponse
		err = parseResponse(resp, &file)
		require.NoError(t, err)
		assert.Equal(t, fileName, file.Name)
		assert.False(t, file.IsFolder)
		assert.NotZero(t, file.Size)
	})

	// 5. Download file
	t.Run("Download File", func(t *testing.T) {
	    require.NotEmpty(t, testFileID, "File ID should be set from previous test")

	    resp, err := makeRequest(http.MethodGet, fmt.Sprintf("/files/%s/download", testFileID), nil)
	    require.NoError(t, err, "Failed to make download request")
	    defer resp.Body.Close()
	    
	    // Проверяем статус код
	    assert.Equal(t, http.StatusOK, resp.StatusCode, 
	        "Expected status %d, got %d. Error: %s", 
	        http.StatusOK, resp.StatusCode, parseErrorResponse(resp))

	    // Проверяем заголовки
	    assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"),
	        "Content-Type header mismatch. Expected text/plain, got %s", resp.Header.Get("Content-Type"))
	    
	    expectedDisposition := fmt.Sprintf("attachment; filename=%q", fileName)
	    assert.Equal(t, expectedDisposition, resp.Header.Get("Content-Disposition"),
	        "Content-Disposition header mismatch. Expected %s, got %s", 
	        expectedDisposition, resp.Header.Get("Content-Disposition"))
	    
	    expectedLength := fmt.Sprint(len(fileContent))
	    assert.Equal(t, expectedLength, resp.Header.Get("Content-Length"),
	        "Content-Length header mismatch. Expected %s, got %s",
	        expectedLength, resp.Header.Get("Content-Length"))
	    
	    body, err := io.ReadAll(resp.Body)
	    require.NoError(t, err, "Failed to read response body")
	    
	    assert.Equal(t, fileContent, string(body),
	        "File content mismatch. Expected: %q, got: %q",
	        fileContent, string(body))
	})

	// 6. Delete file
	t.Run("Delete File", func(t *testing.T) {
		require.NotEmpty(t, testFileID, "File ID should be set from previous test")

		resp, err := makeRequest(http.MethodDelete, fmt.Sprintf("/files/%s", testFileID), nil)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusOK, resp.StatusCode, parseErrorResponse(resp))
		}

		resp, err = makeRequest(http.MethodGet, fmt.Sprintf("/files/%s", testFileID), nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// 7. Delete folder
	t.Run("Delete Folder", func(t *testing.T) {
		require.NotEmpty(t, testFolderID, "Folder ID should be set from previous test")

		resp, err := makeRequest(http.MethodDelete, fmt.Sprintf("/files/%s", testFolderID), nil)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d, error: %s", http.StatusOK, resp.StatusCode, parseErrorResponse(resp))
		}

		resp, err = makeRequest(http.MethodGet, fmt.Sprintf("/files/%s", testFolderID), nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
} 