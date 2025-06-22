#!/bin/bash

# Тестовый скрипт для проверки API файлового сервиса
# Замените AUTH_TOKEN на реальный токен аутентификации

AUTH_TOKEN="your-auth-token-here"
BASE_URL="http://localhost:8080/api/v1"

echo "🧪 Тестирование API файлового сервиса"
echo "======================================"

# Проверка health endpoint
echo "1. Проверка health endpoint..."
curl -s -X GET "${BASE_URL}/health" | jq '.' || echo "Health endpoint недоступен"

# Создание тестового файла
echo -e "\n2. Создание тестового файла..."
cat > test_file.txt << EOF
Это тестовый файл для проверки API.
Содержимое файла для тестирования загрузки и скачивания.
EOF

# Тест прямой загрузки файла
echo -e "\n3. Тест прямой загрузки файла..."
curl -s -X POST "${BASE_URL}/upload" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -d '{"filePath": "test/upload_test.txt"}' \
  --data-binary @test_file.txt | jq '.' || echo "Ошибка загрузки файла"

# Тест прямой загрузки файла (без авторизации для проверки)
echo -e "\n4. Тест загрузки файла без авторизации..."
curl -s -X POST "${BASE_URL}/upload" \
  -H "Content-Type: application/json" \
  -d '{"filePath": "test/upload_test_no_auth.txt"}' \
  --data-binary @test_file.txt | jq '.' || echo "Ожидаемая ошибка авторизации"

# Тест инициализации возобновляемой загрузки
echo -e "\n5. Тест инициализации возобновляемой загрузки..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/upload/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -d '{
    "filePath": "test/resumable_test.txt",
    "size": 1024,
    "sha256": "test-checksum"
  }')

echo "$RESPONSE" | jq '.' || echo "Ошибка инициализации возобновляемой загрузки"

# Извлечение session ID из ответа
SESSION_ID=$(echo "$RESPONSE" | jq -r '.session_id // empty')
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    echo -e "\n6. Тест загрузки части файла (session_id: ${SESSION_ID})..."
    curl -s -X POST "${BASE_URL}/upload/resumable/${SESSION_ID}" \
      -H "Content-Type: application/octet-stream" \
      -H "Content-Range: bytes 0-1023/1024" \
      -H "Authorization: Bearer ${AUTH_TOKEN}" \
      --data-binary @test_file.txt | jq '.' || echo "Ошибка загрузки части файла"
else
    echo "Session ID не получен, пропускаем тест загрузки части"
fi

# Тест инициализации возобновляемого скачивания
echo -e "\n7. Тест инициализации возобновляемого скачивания..."
DOWNLOAD_RESPONSE=$(curl -s -X GET "${BASE_URL}/download/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -d '{"filePath": "test/upload_test.txt"}')

echo "$DOWNLOAD_RESPONSE" | jq '.' || echo "Ошибка инициализации возобновляемого скачивания"

# Извлечение session ID для скачивания
DOWNLOAD_SESSION_ID=$(echo "$DOWNLOAD_RESPONSE" | jq -r '.session_id // empty')
if [ -n "$DOWNLOAD_SESSION_ID" ] && [ "$DOWNLOAD_SESSION_ID" != "null" ]; then
    echo -e "\n8. Тест скачивания части файла (session_id: ${DOWNLOAD_SESSION_ID})..."
    curl -s -X GET "${BASE_URL}/download/resumable/${DOWNLOAD_SESSION_ID}" \
      -H "Content-Range: bytes 0-1023/1024" \
      -H "Authorization: Bearer ${AUTH_TOKEN}" \
      -o downloaded_chunk.bin
    echo "Часть файла скачана в downloaded_chunk.bin"
else
    echo "Session ID для скачивания не получен, пропускаем тест скачивания части"
fi

# Очистка
echo -e "\n9. Очистка тестовых файлов..."
rm -f test_file.txt downloaded_chunk.bin

echo -e "\n✅ Тестирование завершено!"
echo "Проверьте логи сервера для дополнительной информации." 