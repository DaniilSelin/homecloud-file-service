#!/bin/bash

# Тестовый скрипт для возобновляемой загрузки файла
# Пользователь: testuser
# Файл: temp/test_file.txt

BASE_URL="http://localhost:8082/api/v1"
TEST_USER_ID="550e8400-e29b-41d4-a716-446655440000"  # UUID для testuser
TEST_FILE="temp/test_file.txt"
FILE_SIZE=$(wc -c < "$TEST_FILE")
FILE_SHA256=$(sha256sum "$TEST_FILE" | cut -d' ' -f1)

echo "🧪 Тестирование возобновляемой загрузки файла"
echo "=============================================="
echo "Пользователь: testuser (UUID: $TEST_USER_ID)"
echo "Файл: $TEST_FILE"
echo "Размер: $FILE_SIZE байт"
echo "SHA256: $FILE_SHA256"
echo ""

# Создаем заглушку токена для testuser
# В реальной системе это должен быть валидный JWT токен
TEST_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNTUwZTg0MDAtZTI5Yi00MWQ0LWE3MTYtNDY2NTU0NDAwMDAwIiwidXNlcm5hbWUiOiJ0ZXN0dXNlciIsImlhdCI6MTYzNTU1NTU1NSwiZXhwIjoxOTUwOTMxNTU1fQ.test_signature"

echo "1. Инициализация возобновляемой загрузки..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/upload/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TEST_TOKEN}" \
  -d "{
    \"filePath\": \"documents/test_file.txt\",
    \"size\": $FILE_SIZE,
    \"sha256\": \"$FILE_SHA256\"
  }")

echo "Ответ: $RESPONSE"

# Извлекаем session ID
SESSION_ID=$(echo "$RESPONSE" | grep -o '"sessionID":"[^"]*"' | cut -d'"' -f4)

if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    echo "✅ Session ID получен: $SESSION_ID"
    
    echo ""
    echo "2. Загрузка файла целиком..."
    UPLOAD_RESPONSE=$(curl -s -X POST "${BASE_URL}/upload/resumable/${SESSION_ID}" \
      -H "Content-Type: application/octet-stream" \
      -H "Content-Range: bytes 0-$((FILE_SIZE-1))/$FILE_SIZE" \
      -H "Authorization: Bearer ${TEST_TOKEN}" \
      --data-binary @"$TEST_FILE")
    
    echo "Ответ загрузки: $UPLOAD_RESPONSE"
    
    echo ""
    echo "3. Проверка, что файл сохранен..."
    ls -la "storage/users/${TEST_USER_ID}/documents/" 2>/dev/null || echo "Директория не найдена"
    
    echo ""
    echo "4. Тест инициализации возобновляемого скачивания..."
    DOWNLOAD_RESPONSE=$(curl -s -X GET "${BASE_URL}/download/resumable" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${TEST_TOKEN}" \
      -d "{
        \"filePath\": \"documents/test_file.txt\"
      }")
    
    echo "Ответ скачивания: $DOWNLOAD_RESPONSE"
    
else
    echo "❌ Не удалось получить Session ID"
    echo "Полный ответ: $RESPONSE"
fi

echo ""
echo "✅ Тестирование завершено!" 