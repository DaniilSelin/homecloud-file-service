#!/bin/bash

# Тестовый скрипт для проверки загрузки и скачивания файлов
# Требует запущенный сервис на localhost:8080

BASE_URL="http://localhost:8080/api/v1"
AUTH_TOKEN="your-auth-token-here"  # Замените на реальный токен

echo "=== Тестирование HomeCloud File Service ==="

# Создаем тестовый файл
echo "Создаем тестовый файл..."
echo "Hello, HomeCloud!" > test_file.txt

# Тест 1: Загрузка файла
echo -e "\n1. Тестируем загрузку файла..."
curl -X POST "$BASE_URL/upload" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{"filePath": "test_folder/test_file.txt"}' \
  --data-binary @test_file.txt

echo -e "\n\n2. Тестируем скачивание файла..."
curl -X GET "$BASE_URL/download" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{"filePath": "test_folder/test_file.txt"}' \
  -o downloaded_file.txt

echo -e "\n\n3. Проверяем содержимое скачанного файла..."
cat downloaded_file.txt

echo -e "\n\n4. Тестируем возобновляемую загрузку..."
# Инициализация сессии
SESSION_RESPONSE=$(curl -s -X POST "$BASE_URL/upload/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{"filePath": "test_folder/resumable_file.txt", "size": 17, "sha256": "test"}')

echo "Ответ инициализации сессии: $SESSION_RESPONSE"

# Извлекаем sessionID (упрощенно)
SESSION_ID=$(echo $SESSION_RESPONSE | grep -o '"sessionID":"[^"]*"' | cut -d'"' -f4)

if [ ! -z "$SESSION_ID" ]; then
    echo "Session ID: $SESSION_ID"
    
    # Загрузка первого чанка
    echo "Загружаем первый чанк..."
    curl -X POST "$BASE_URL/upload/resumable/$SESSION_ID" \
      -H "Content-Type: application/octet-stream" \
      -H "Content-Range: bytes 0-8/17" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      --data-binary "Hello, Ho"
    
    # Загрузка второго чанка
    echo -e "\nЗагружаем второй чанк..."
    curl -X POST "$BASE_URL/upload/resumable/$SESSION_ID" \
      -H "Content-Type: application/octet-stream" \
      -H "Content-Range: bytes 9-16/17" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      --data-binary "meCloud!"
else
    echo "Не удалось получить Session ID"
fi

echo -e "\n\n5. Тестируем возобновляемое скачивание..."
# Инициализация сессии скачивания
DOWNLOAD_SESSION_RESPONSE=$(curl -s -X GET "$BASE_URL/download/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{"filePath": "test_folder/test_file.txt"}')

echo "Ответ инициализации сессии скачивания: $DOWNLOAD_SESSION_RESPONSE"

# Извлекаем sessionID для скачивания
DOWNLOAD_SESSION_ID=$(echo $DOWNLOAD_SESSION_RESPONSE | grep -o '"sessionID":"[^"]*"' | cut -d'"' -f4)

if [ ! -z "$DOWNLOAD_SESSION_ID" ]; then
    echo "Download Session ID: $DOWNLOAD_SESSION_ID"
    
    # Скачивание первого чанка
    echo "Скачиваем первый чанк..."
    curl -X GET "$BASE_URL/download/resumable/$DOWNLOAD_SESSION_ID" \
      -H "Range: bytes=0-8" \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      -o chunk1.txt
    
    echo "Содержимое первого чанка:"
    cat chunk1.txt
else
    echo "Не удалось получить Download Session ID"
fi

echo -e "\n\n6. Очистка тестовых файлов..."
rm -f test_file.txt downloaded_file.txt chunk1.txt

echo -e "\n=== Тестирование завершено ===" 