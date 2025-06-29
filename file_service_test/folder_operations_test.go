package file_service_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var apiURL = "http://localhost:8080/api/v1"

func TestFolderOperations(t *testing.T) {
	// Создаем временную директорию для тестов
	tempDir, err := os.MkdirTemp("", "folder_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Создаем тестовую структуру папок и файлов
	testFiles := map[string]string{
		"file1.txt":           "Content of file 1",
		"subfolder/file2.txt": "Content of file 2",
		"subfolder/file3.txt": "Content of file 3",
		"images/pic1.jpg":     "Binary content 1",
		"images/pic2.png":     "Binary content 2",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("Upload_Folder", func(t *testing.T) {
		// Создаем ZIP архив
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		for path, content := range testFiles {
			writer, err := zipWriter.Create(path)
			require.NoError(t, err)
			_, err = writer.Write([]byte(content))
			require.NoError(t, err)
		}
		err := zipWriter.Close()
		require.NoError(t, err)

		// Создаем multipart запрос
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Добавляем ZIP файл
		part, err := writer.CreateFormFile("folder", "test_folder.zip")
		require.NoError(t, err)
		_, err = io.Copy(part, bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)

		// Добавляем путь папки
		err = writer.WriteField("folderPath", "test_folder")
		require.NoError(t, err)
		writer.Close()

		// Отправляем запрос
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/folders/upload", apiURL), body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testToken))

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Проверяем ответ
		var result struct {
			Message string      `json:"message"`
			Folder  interface{} `json:"folder"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result.Message, "Folder uploaded successfully")
	})

	t.Run("Download_Folder", func(t *testing.T) {
		// Отправляем запрос на скачивание папки
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/folders/download?path=test_folder", apiURL), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testToken))

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))

		// Читаем все содержимое ответа
		content, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Создаем reader для ZIP архива
		zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		require.NoError(t, err)

		// Проверяем содержимое архива
		foundFiles := make(map[string]bool)
		for _, file := range zipReader.File {
			foundFiles[file.Name] = true

			// Проверяем содержимое файла
			if content, ok := testFiles[file.Name]; ok {
				rc, err := file.Open()
				require.NoError(t, err)
				defer rc.Close()

				data, err := io.ReadAll(rc)
				require.NoError(t, err)
				assert.Equal(t, content, string(data))
			}
		}

		// Проверяем, что все файлы присутствуют
		for path := range testFiles {
			assert.True(t, foundFiles[path], "File %s should be in the ZIP archive", path)
		}
	})
} 