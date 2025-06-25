#!/bin/bash

# Тестовый скрипт для проверки функциональности работы с папками и навигации
# HomeCloud File Service

# Конфигурация
BASE_URL="http://localhost:8082/api/v1"
TOKEN="your-jwt-token-here"  # Замените на реальный токен

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функции для вывода
print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Проверка доступности сервиса
check_health() {
    print_header "Проверка доступности сервиса"
    
    response=$(curl -s -w "%{http_code}" "$BASE_URL/health")
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Сервис доступен"
        echo "Ответ: $body"
    else
        print_error "Сервис недоступен (HTTP $http_code)"
        exit 1
    fi
}

# Создание тестового файла
create_test_file() {
    print_header "Создание тестового файла"
    
    echo "Это тестовый файл для проверки функциональности." > test_file.txt
    print_success "Тестовый файл создан"
}

# Тест 1: Загрузка файла с автоматическим созданием папок
test_upload_with_folders() {
    print_header "Тест 1: Загрузка файла с автоматическим созданием папок"
    
    response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/upload" \
        -H "Authorization: Bearer $TOKEN" \
        -F "file=@test_file.txt" \
        -F "filePath=Documents/Reports/2024/test_report.txt")
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 201 ]; then
        print_success "Файл загружен с автоматическим созданием папок"
        echo "Ответ: $body"
    else
        print_error "Ошибка загрузки файла (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 2: Создание папки
test_create_folder() {
    print_header "Тест 2: Создание папки"
    
    response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/folders" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{
            "name": "TestFolder"
        }')
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 201 ]; then
        print_success "Папка создана"
        echo "Ответ: $body"
    else
        print_error "Ошибка создания папки (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 3: Просмотр содержимого папки по пути
test_browse_folder() {
    print_header "Тест 3: Просмотр содержимого папки по пути"
    
    response=$(curl -s -w "%{http_code}" -X GET "$BASE_URL/folders/browse?path=Documents/Reports" \
        -H "Authorization: Bearer $TOKEN")
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Содержимое папки получено"
        echo "Ответ: $body"
    else
        print_error "Ошибка получения содержимого папки (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 4: Навигация по пути с breadcrumbs
test_navigate_path() {
    print_header "Тест 4: Навигация по пути с breadcrumbs"
    
    response=$(curl -s -w "%{http_code}" -X GET "$BASE_URL/folders/navigate?path=Documents/Reports/2024" \
        -H "Authorization: Bearer $TOKEN")
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Навигация выполнена"
        echo "Ответ: $body"
    else
        print_error "Ошибка навигации (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 5: Скачивание файла по пути
test_download_by_path() {
    print_header "Тест 5: Скачивание файла по пути"
    
    response=$(curl -s -w "%{http_code}" -X GET "$BASE_URL/download?path=Documents/Reports/2024/test_report.txt" \
        -H "Authorization: Bearer $TOKEN" \
        -o downloaded_test_file.txt)
    
    http_code="${response: -3}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Файл скачан по пути"
        if [ -f "downloaded_test_file.txt" ]; then
            echo "Файл сохранен как downloaded_test_file.txt"
            echo "Содержимое:"
            cat downloaded_test_file.txt
        fi
    else
        print_error "Ошибка скачивания файла (HTTP $http_code)"
    fi
}

# Тест 6: Создание файла в папке
test_create_file_in_folder() {
    print_header "Тест 6: Создание файла в папке"
    
    response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/files" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{
            "name": "document_in_folder.pdf",
            "mime_type": "application/pdf",
            "size": 1024
        }')
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 201 ]; then
        print_success "Файл создан"
        echo "Ответ: $body"
    else
        print_error "Ошибка создания файла (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 7: Поиск файлов
test_search_files() {
    print_header "Тест 7: Поиск файлов"
    
    response=$(curl -s -w "%{http_code}" -X GET "$BASE_URL/files/search?q=test" \
        -H "Authorization: Bearer $TOKEN")
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Поиск выполнен"
        echo "Ответ: $body"
    else
        print_error "Ошибка поиска (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Тест 8: Получение списка файлов
test_list_files() {
    print_header "Тест 8: Получение списка файлов"
    
    response=$(curl -s -w "%{http_code}" -X GET "$BASE_URL/files?limit=10&offset=0" \
        -H "Authorization: Bearer $TOKEN")
    
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" -eq 200 ]; then
        print_success "Список файлов получен"
        echo "Ответ: $body"
    else
        print_error "Ошибка получения списка файлов (HTTP $http_code)"
        echo "Ответ: $body"
    fi
}

# Очистка тестовых файлов
cleanup() {
    print_header "Очистка тестовых файлов"
    
    rm -f test_file.txt downloaded_test_file.txt
    print_success "Тестовые файлы удалены"
}

# Основная функция
main() {
    print_header "Запуск тестов функциональности работы с папками и навигации"
    
    # Проверка токена
    if [ "$TOKEN" = "your-jwt-token-here" ]; then
        print_error "Пожалуйста, установите реальный JWT токен в переменной TOKEN"
        exit 1
    fi
    
    # Выполнение тестов
    check_health
    create_test_file
    test_upload_with_folders
    test_create_folder
    test_browse_folder
    test_navigate_path
    test_download_by_path
    test_create_file_in_folder
    test_search_files
    test_list_files
    cleanup
    
    print_header "Тестирование завершено"
    print_success "Все тесты выполнены"
}

# Запуск основной функции
main "$@" 