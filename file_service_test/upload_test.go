package file_service_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadAPI(t *testing.T) {
	content := []byte("test file content for api upload")
	fileName := fmt.Sprintf("test_upload_api_%d.txt", time.Now().UnixNano())

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	// Send request
	req := map[string]interface{}{
		"name":     fileName,
		"content":  content,
		"mimeType": "text/plain",
		"size":     len(content),
	}
	resp, err := makeRequest("POST", "/files", req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify in listing
	respList, err := makeRequest("GET", "/files", nil)
	require.NoError(t, err)
	defer respList.Body.Close()

	var list FilesListResponse
	err = parseResponse(respList, &list)
	require.NoError(t, err)

	found := false
	for _, f := range list.Files {
		if f.Name == fileName {
			found = true
			break
		}
	}
	assert.True(t, found, "File %s should be in listing", fileName)
}

func TestResumableUploadAPI(t *testing.T) {
	content := []byte("resumable upload content for api test")
	fileName := fmt.Sprintf("test_resumable_api_%d.txt", time.Now().UnixNano())
	sha := fmt.Sprintf("%x", sha256.Sum256(content))

	// 1. Initialize session
	initReq := map[string]interface{}{
		"filePath": fileName,
		"size":     len(content),
		"sha256":   sha,
	}

	resp, err := makeRequest("POST", "/files/upload/resumable", initReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var respData map[string]string
	err = parseResponse(resp, &respData)
	require.NoError(t, err)
	sessionID := respData["session_id"]
	require.NotEmpty(t, sessionID)

	// 2. Upload chunks
	chunkSize := 7
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize - 1
		if end >= len(content) {
			end = len(content) - 1
		}
		
		// Устанавливаем Content-Range
		// headers := map[string]string{
		// 	"Content-Range": fmt.Sprintf("bytes %d-%d/%d", i, end, len(content)),
		// }
		
		// resp, err := makeRequestWithHeaders( // Нужна новая функция с поддержкой заголовков
		// 	"PATCH", 
		// 	fmt.Sprintf("/files/upload/resumable/%s", sessionID),
		// 	chunk,
		// 	headers,
		// )
	}

	// Verify in listing
	respList, err := makeRequest("GET", "/files", nil)
	require.NoError(t, err)
	defer respList.Body.Close()

	var list FilesListResponse
	err = parseResponse(respList, &list)
	require.NoError(t, err)

	found := false
	for _, f := range list.Files {
		if f.Name == fileName {
			found = true
			break
		}
	}
	assert.True(t, found, "File %s should be in listing", fileName)
}