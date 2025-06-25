# HomeCloud File Service - Быстрый старт

## Быстрый запуск

### 1. Предварительные требования

- Go 1.23+
- Docker & Docker Compose
- PostgreSQL 15+ (через Docker)

### 2. Клонирование и настройка

```bash
# Клонировать репозиторий
git clone <repository-url>
cd homecloud-file-service

# Установить зависимости
make deps

# Настроить конфигурацию
cp config/config.example.yaml config/config.local.yaml
# Отредактировать config/config.local.yaml при необходимости
```

### 3. Запуск с Docker

```bash
# Запустить все сервисы
docker-compose up -d

# Применить миграции
docker-compose exec file-service migrate -path migrations -database "postgres://postgres:password@postgres:5432/homecloud_files?sslmode=disable" up
```

### 4. Локальный запуск

```bash
# Запустить только базу данных
docker-compose up postgres redis -d

# Применить миграции
make migrate-up

# Запустить сервис
make dev
```

## Тестирование

### Запуск тестового скрипта

```bash
# Отредактировать токен в скрипте
vim test_api.sh

# Запустить тесты
./test_api.sh
```

### Ручное тестирование

```bash
# Проверка health endpoint
curl http://localhost:8082/api/v1/health

# Загрузка файла с автоматическим созданием папок (замените TOKEN на реальный токен)
curl -X POST "http://localhost:8082/api/v1/upload" \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@file.txt" \
  -F "filePath=Documents/Reports/test.txt"
```

## Структура проекта

```
homecloud-file-service/
├── cmd/server/          # Точка входа
├── config/              # Конфигурация
├── internal/            # Внутренняя логика
│   ├── auth/           # Аутентификация
│   ├── dbmanager/      # gRPC клиент для БД
│   ├── models/         # Модели данных
│   ├── repository/     # Доступ к данным
│   ├── service/        # Бизнес-логика
│   └── transport/      # HTTP/gRPC API
├── migrations/         # Миграции БД
├── storage/           # Файловое хранилище
└── docker-compose.yml # Docker конфигурация
```

## Основные команды

```bash
make help          # Справка по командам
make build         # Сборка приложения
make run           # Запуск приложения
make dev           # Запуск в режиме разработки
make test          # Запуск тестов
make migrate-up    # Применить миграции
make migrate-down  # Откатить миграции
make docker-build  # Сборка Docker образа
```

## API Endpoints

### Аутентификация
Все эндпоинты требуют аутентификации через Bearer токен:
```
Authorization: Bearer <your-jwt-token>
```

### Основные операции с файлами

#### Создание файла
```bash
curl -X POST "http://localhost:8082/api/v1/files" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "document.pdf",
    "parent_id": "uuid-optional",
    "is_folder": false,
    "mime_type": "application/pdf",
    "size": 1024
  }'
```

#### Получение файла
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}" \
  -H "Authorization: Bearer TOKEN"
```

#### Обновление файла
```bash
curl -X PUT "http://localhost:8082/api/v1/files/{id}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "updated_document.pdf",
    "parent_id": "uuid-optional",
    "starred": true
  }'
```

#### Удаление файла
```bash
curl -X DELETE "http://localhost:8082/api/v1/files/{id}" \
  -H "Authorization: Bearer TOKEN"
```

#### Восстановление файла
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/restore" \
  -H "Authorization: Bearer TOKEN"
```

#### Список файлов
```bash
curl -X GET "http://localhost:8082/api/v1/files?parent_id=uuid&is_trashed=false&starred=false&limit=20&offset=0&order_by=name&order_dir=asc" \
  -H "Authorization: Bearer TOKEN"
```

### Загрузка и скачивание файлов

#### Прямая загрузка (по пути) - Улучшенная версия
```bash
curl -X POST "http://localhost:8082/api/v1/upload" \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@report.pdf" \
  -F "filePath=Documents/Reports/2024/january.pdf"
```

#### Прямое скачивание (по пути)
```bash
curl -X GET "http://localhost:8082/api/v1/download?path=Documents/Reports/2024/january.pdf" \
  -H "Authorization: Bearer TOKEN" \
  -o downloaded_report.pdf
```

#### Загрузка по ID файла
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/upload" \
  -H "Content-Type: application/octet-stream" \
  -H "Authorization: Bearer TOKEN" \
  --data-binary @file.pdf
```

#### Скачивание по ID файла
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/download" \
  -H "Authorization: Bearer TOKEN" \
  -o downloaded_file.pdf
```

#### Получение содержимого файла
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/content" \
  -H "Authorization: Bearer TOKEN"
```

### Операции с папками

#### Создание папки
```bash
curl -X POST "http://localhost:8082/api/v1/folders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "Documents",
    "parent_id": "uuid-optional"
  }'
```

#### Содержимое папки (по ID)
```bash
curl -X GET "http://localhost:8082/api/v1/folders/{id}/contents" \
  -H "Authorization: Bearer TOKEN"
```

#### Содержимое папки (по пути)
```bash
curl -X GET "http://localhost:8082/api/v1/folders/contents?path=Documents/Reports" \
  -H "Authorization: Bearer TOKEN"
```

#### Просмотр папки с детализацией
```bash
curl -X GET "http://localhost:8082/api/v1/folders/browse?path=Documents/Reports" \
  -H "Authorization: Bearer TOKEN"
```

#### Навигация по пути с breadcrumbs
```bash
curl -X GET "http://localhost:8082/api/v1/folders/navigate?path=Documents/Reports/2024" \
  -H "Authorization: Bearer TOKEN"
```

### Поиск и фильтры

#### Поиск файлов
```bash
curl -X GET "http://localhost:8082/api/v1/files/search?q=document" \
  -H "Authorization: Bearer TOKEN"
```

#### Избранные файлы
```bash
curl -X GET "http://localhost:8082/api/v1/files/starred" \
  -H "Authorization: Bearer TOKEN"
```

#### Удаленные файлы
```bash
curl -X GET "http://localhost:8082/api/v1/files/trashed" \
  -H "Authorization: Bearer TOKEN"
```

### Ревизии файлов

#### Список ревизий
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/revisions" \
  -H "Authorization: Bearer TOKEN"
```

#### Получение ревизии
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/revisions/{revisionId}" \
  -H "Authorization: Bearer TOKEN"
```

#### Восстановление ревизии
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/revisions/{revisionId}/restore" \
  -H "Authorization: Bearer TOKEN"
```

### Права доступа

#### Список прав доступа
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/permissions" \
  -H "Authorization: Bearer TOKEN"
```

#### Предоставление прав
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/permissions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "grantee_id": "user-uuid",
    "grantee_type": "USER",
    "role": "READER",
    "allow_share": true
  }'
```

#### Отзыв прав
```bash
curl -X DELETE "http://localhost:8082/api/v1/files/{id}/permissions/{granteeId}" \
  -H "Authorization: Bearer TOKEN"
```

### Специальные операции с файлами

#### Добавить в избранное
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/star" \
  -H "Authorization: Bearer TOKEN"
```

#### Убрать из избранного
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/unstar" \
  -H "Authorization: Bearer TOKEN"
```

#### Переместить файл
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/move" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "new_parent_id": "uuid-optional"
  }'
```

#### Копировать файл
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/copy" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "new_parent_id": "uuid-optional",
    "new_name": "copy_of_file.pdf"
  }'
```

#### Переименовать файл
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/rename" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "new_name": "renamed_file.pdf"
  }'
```

### Метаданные файлов

#### Получить метаданные
```bash
curl -X GET "http://localhost:8082/api/v1/files/{id}/metadata" \
  -H "Authorization: Bearer TOKEN"
```

#### Обновить метаданные
```bash
curl -X PUT "http://localhost:8082/api/v1/files/{id}/metadata" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "author": "John Doe",
    "description": "Important document",
    "tags": ["work", "important"]
  }'
```

### Проверка целостности

#### Проверить целостность файла
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/verify" \
  -H "Authorization: Bearer TOKEN"
```

#### Вычислить контрольные суммы
```bash
curl -X POST "http://localhost:8082/api/v1/files/{id}/checksums" \
  -H "Authorization: Bearer TOKEN"
```

### Управление хранилищем

#### Информация о хранилище
```bash
curl -X GET "http://localhost:8082/api/v1/storage/info" \
  -H "Authorization: Bearer TOKEN"
```

#### Очистка хранилища
```bash
curl -X POST "http://localhost:8082/api/v1/storage/cleanup" \
  -H "Authorization: Bearer TOKEN"
```

#### Оптимизация хранилища
```bash
curl -X POST "http://localhost:8082/api/v1/storage/optimize" \
  -H "Authorization: Bearer TOKEN"
```

## Примеры использования

### Полный цикл работы с файлом

1. **Создание файла:**
```bash
curl -X POST "http://localhost:8082/api/v1/files" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "document.pdf",
    "mime_type": "application/pdf",
    "size": 1024
  }'
```

2. **Загрузка содержимого:**
```bash
curl -X POST "http://localhost:8082/api/v1/files/{file-id}/upload" \
  -H "Content-Type: application/octet-stream" \
  -H "Authorization: Bearer TOKEN" \
  --data-binary @document.pdf
```

3. **Добавление в избранное:**
```bash
curl -X POST "http://localhost:8082/api/v1/files/{file-id}/star" \
  -H "Authorization: Bearer TOKEN"
```

4. **Скачивание файла:**
```bash
curl -X GET "http://localhost:8082/api/v1/files/{file-id}/download" \
  -H "Authorization: Bearer TOKEN" \
  -o downloaded_document.pdf
```

### Работа с папками

1. **Создание папки:**
```bash
curl -X POST "http://localhost:8082/api/v1/folders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "Documents"
  }'
```

2. **Перемещение файла в папку:**
```bash
curl -X POST "http://localhost:8082/api/v1/files/{file-id}/move" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "new_parent_id": "folder-id"
  }'
```

3. **Просмотр содержимого папки:**
```bash
curl -X GET "http://localhost:8082/api/v1/folders/browse?path=Documents/Reports" \
  -H "Authorization: Bearer TOKEN"
```

### Навигация по папкам

1. **Создание структуры папок при загрузке файла:**
```bash
curl -X POST "http://localhost:8082/api/v1/upload" \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@report.pdf" \
  -F "filePath=Documents/Reports/2024/january.pdf"
```

2. **Просмотр содержимого с детализацией:**
```bash
curl -X GET "http://localhost:8082/api/v1/folders/browse?path=Documents/Reports" \
  -H "Authorization: Bearer TOKEN"
```

3. **Навигация с breadcrumbs:**
```bash
curl -X GET "http://localhost:8082/api/v1/folders/navigate?path=Documents/Reports/2024" \
  -H "Authorization: Bearer TOKEN"
```

4. **Скачивание файла по пути:**
```bash
curl -X GET "http://localhost:8082/api/v1/download?path=Documents/Reports/2024/january.pdf" \
  -H "Authorization: Bearer TOKEN" \
  -o january.pdf
```

### Поиск и фильтрация

1. **Поиск файлов:**
```bash
curl -X GET "http://localhost:8082/api/v1/files/search?q=document" \
  -H "Authorization: Bearer TOKEN"
```

2. **Получение избранных файлов:**
```bash
curl -X GET "http://localhost:8082/api/v1/files/starred" \
  -H "Authorization: Bearer TOKEN"
```

3. **Фильтрация по папке:**
```bash
curl -X GET "http://localhost:8082/api/v1/files?parent_id={folder-id}" \
  -H "Authorization: Bearer TOKEN"
```

## Особенности реализации

### Автоматическое создание папок
При загрузке файла по пути система автоматически создает все необходимые папки в иерархии. Например, при загрузке файла в путь `Documents/Reports/2024/january.pdf` будут созданы папки:
- `Documents`
- `Documents/Reports`
- `Documents/Reports/2024`

### Навигация по путям
Система поддерживает навигацию как по ID папок, так и по путям, что позволяет легко интегрировать с веб-интерфейсами для просмотра файлов.

### Права доступа
Все операции проверяют права доступа пользователя к файлам и папкам. Пользователь может работать только с файлами, к которым у него есть доступ.

### Версионирование
При обновлении файлов автоматически создаются ревизии, что позволяет отслеживать изменения и восстанавливать предыдущие версии.

## Мониторинг

- **Health Check**: `