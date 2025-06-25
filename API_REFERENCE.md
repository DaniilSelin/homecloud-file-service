# HomeCloud File Service - API Reference

## Обзор

HomeCloud File Service предоставляет REST API для управления файлами и папками с поддержкой версионирования, прав доступа, навигации по папкам и проверки целостности.

**Base URL**: `http://localhost:8082/api/v1`

## Аутентификация

Все эндпоинты требуют аутентификации через Bearer токен в заголовке:

```
Authorization: Bearer <your-jwt-token>
```

## Коды ответов

- `200 OK` - Успешная операция
- `201 Created` - Ресурс создан
- `400 Bad Request` - Неверный запрос
- `401 Unauthorized` - Не авторизован
- `403 Forbidden` - Доступ запрещен
- `404 Not Found` - Ресурс не найден
- `409 Conflict` - Конфликт (например, файл уже существует)
- `500 Internal Server Error` - Внутренняя ошибка сервера

## Модели данных

### File
```json
{
  "id": "uuid",
  "owner_id": "uuid",
  "parent_id": "uuid-optional",
  "name": "filename.pdf",
  "file_extension": ".pdf",
  "mime_type": "application/pdf",
  "storage_path": "storage/users/uuid/filename.pdf",
  "size": 1024,
  "md5_checksum": "hash",
  "sha256_checksum": "hash",
  "is_folder": false,
  "is_trashed": false,
  "trashed_at": "2023-01-01T00:00:00Z",
  "starred": false,
  "created_at": "2023-01-01T00:00:00Z",
  "updated_at": "2023-01-01T00:00:00Z",
  "last_viewed_at": "2023-01-01T00:00:00Z",
  "viewed_by_me": false,
  "version": 1,
  "revision_id": "uuid-optional",
  "indexable_text": "text content",
  "thumbnail_link": "url",
  "web_view_link": "url",
  "web_content_link": "url",
  "icon_link": "url"
}
```

### FileRevision
```json
{
  "id": "uuid",
  "file_id": "uuid",
  "revision_id": 1,
  "size": 1024,
  "storage_path": "storage/users/uuid/filename_v1.pdf",
  "mime_type": "application/pdf",
  "md5_checksum": "hash",
  "user_id": "uuid",
  "created_at": "2023-01-01T00:00:00Z"
}
```

### FilePermission
```json
{
  "id": "uuid",
  "file_id": "uuid",
  "grantee_id": "uuid",
  "grantee_type": "USER|GROUP|DOMAIN|ANYONE",
  "role": "OWNER|ORGANIZER|FILE_OWNER|WRITER|COMMENTER|READER",
  "allow_share": true,
  "created_at": "2023-01-01T00:00:00Z"
}
```

## Эндпоинты

### Основные операции с файлами

#### Создание файла
```http
POST /files
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "document.pdf",
  "parent_id": "uuid-optional",
  "is_folder": false,
  "mime_type": "application/pdf",
  "size": 1024,
  "content": "base64-encoded-content-optional"
}
```

**Ответ:**
```json
{
  "id": "uuid",
  "name": "document.pdf",
  "mime_type": "application/pdf",
  "size": 1024,
  "is_folder": false,
  "created_at": "2023-01-01T00:00:00Z"
}
```

#### Получение файла
```http
GET /files/{id}
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "id": "uuid",
  "name": "document.pdf",
  "mime_type": "application/pdf",
  "size": 1024,
  "is_folder": false,
  "starred": false,
  "created_at": "2023-01-01T00:00:00Z"
}
```

#### Обновление файла
```http
PUT /files/{id}
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "updated_document.pdf",
  "parent_id": "uuid-optional",
  "starred": true,
  "content": "base64-encoded-content-optional"
}
```

**Ответ:**
```json
{
  "id": "uuid",
  "name": "updated_document.pdf",
  "mime_type": "application/pdf",
  "updated_at": "2023-01-01T00:00:00Z"
}
```

#### Удаление файла
```http
DELETE /files/{id}
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "File deleted successfully"
}
```

#### Восстановление файла
```http
POST /files/{id}/restore
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "File restored successfully"
}
```

#### Список файлов
```http
GET /files?parent_id=uuid&is_trashed=false&starred=false&limit=20&offset=0&order_by=name&order_dir=asc
Authorization: Bearer <token>
```

**Параметры запроса:**
- `parent_id` (optional) - ID родительской папки
- `is_trashed` (optional) - Показать только удаленные файлы
- `starred` (optional) - Показать только избранные файлы
- `limit` (optional) - Количество файлов на странице (по умолчанию 20)
- `offset` (optional) - Смещение для пагинации (по умолчанию 0)
- `order_by` (optional) - Поле для сортировки (name, size, created_at, updated_at)
- `order_dir` (optional) - Направление сортировки (asc, desc)

**Ответ:**
```json
{
  "files": [
    {
      "id": "uuid",
      "name": "document.pdf",
      "size": 1024,
      "is_folder": false
    }
  ],
  "total": 100,
  "limit": 20,
  "offset": 0
}
```

### Загрузка и скачивание файлов

#### Прямая загрузка (по пути) - Улучшенная версия
```http
POST /upload
Content-Type: multipart/form-data
Authorization: Bearer <token>
```

**Параметры формы:**
- `file` (required) - Файл для загрузки
- `filePath` (optional) - Путь к файлу (если не указан, используется имя файла)

**Пример с curl:**
```bash
curl -X POST http://localhost:8082/api/v1/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@document.pdf" \
  -F "filePath=documents/report.pdf"
```

**Ответ:**
```json
{
  "message": "File uploaded successfully",
  "file": {
    "id": "uuid",
    "name": "report.pdf",
    "size": 1024,
    "is_folder": false,
    "created_at": "2023-01-01T00:00:00Z"
  },
  "path": "documents/report.pdf"
}
```

#### Прямое скачивание (по пути)
```http
GET /download?path=documents/report.pdf
Authorization: Bearer <token>
```

**Ответ:** Бинарные данные файла с заголовками:
```
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="report.pdf"
```

#### Загрузка по ID файла
```http
POST /files/{id}/upload
Content-Type: application/octet-stream
Authorization: Bearer <token>
```
Тело запроса содержит бинарные данные файла.

**Ответ:**
```json
{
  "message": "File uploaded successfully"
}
```

#### Скачивание по ID файла
```http
GET /files/{id}/download
Authorization: Bearer <token>
```

**Ответ:** Бинарные данные файла с заголовками:
```
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="document.pdf"
```

#### Получение содержимого файла
```http
GET /files/{id}/content
Authorization: Bearer <token>
```

**Ответ:** Бинарные данные файла с заголовками:
```
Content-Type: application/octet-stream
Content-Length: 1024
```

### Возобновляемые операции

#### Инициализация возобновляемой загрузки
```http
POST /upload/resumable
Content-Type: application/json
Authorization: Bearer <token>

{
  "filePath": "large_file.zip",
  "size": 1048576,
  "sha256": "hash_value"
}
```

**Ответ:**
```json
{
  "session_id": "uuid",
  "upload_url": "/api/v1/upload/resumable/uuid"
}
```

#### Загрузка части файла
```http
POST /upload/resumable/{sessionID}
Content-Type: application/octet-stream
Content-Range: bytes 0-1048575/1048576
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "bytes_uploaded": 1048576,
  "total_bytes": 1048576
}
```

#### Инициализация возобновляемого скачивания
```http
GET /download/resumable
Content-Type: application/json
Authorization: Bearer <token>

{
  "filePath": "large_file.zip"
}
```

**Ответ:**
```json
{
  "session_id": "uuid",
  "download_url": "/api/v1/download/resumable/uuid",
  "file_size": 1048576
}
```

#### Скачивание части файла
```http
GET /download/resumable/{sessionID}
Range: bytes=0-1048575
Authorization: Bearer <token>
```

**Ответ:** Часть файла с заголовками:
```
Content-Range: bytes 0-1048575/1048576
Content-Length: 1048576
```

### Операции с папками

#### Создание папки
```http
POST /folders
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "Documents",
  "parent_id": "uuid-optional"
}
```

**Ответ:**
```json
{
  "id": "uuid",
  "name": "Documents",
  "is_folder": true,
  "created_at": "2023-01-01T00:00:00Z"
}
```

#### Содержимое папки (по ID)
```http
GET /folders/{id}/contents
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "files": [
    {
      "id": "uuid",
      "name": "document.pdf",
      "size": 1024,
      "is_folder": false
    },
    {
      "id": "uuid2",
      "name": "subfolder",
      "is_folder": true
    }
  ],
  "folder_id": "uuid",
  "path": "Documents"
}
```

#### Содержимое папки (по пути)
```http
GET /folders/contents?path=Documents/Reports
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "files": [
    {
      "id": "uuid",
      "name": "report.pdf",
      "size": 1024,
      "is_folder": false
    }
  ],
  "folder_id": "uuid",
  "path": "Documents/Reports"
}
```

#### Просмотр папки с детализацией
```http
GET /folders/browse?folder_id=uuid
Authorization: Bearer <token>
```

**Или по пути:**
```http
GET /folders/browse?path=Documents/Reports
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "path": "Documents/Reports",
  "folders": [
    {
      "id": "uuid",
      "name": "subfolder",
      "is_folder": true,
      "created_at": "2023-01-01T00:00:00Z"
    }
  ],
  "files": [
    {
      "id": "uuid2",
      "name": "report.pdf",
      "size": 1024,
      "is_folder": false,
      "created_at": "2023-01-01T00:00:00Z"
    }
  ],
  "total_folders": 1,
  "total_files": 1,
  "total_items": 2,
  "folder_id": "uuid"
}
```

#### Навигация по пути с breadcrumbs
```http
GET /folders/navigate?path=Documents/Reports/2024
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "requested_path": "Documents/Reports/2024",
  "current_path": "Documents/Reports/2024",
  "breadcrumbs": [
    {
      "name": "Documents",
      "path": "Documents",
      "folder_id": "uuid1",
      "level": 0
    },
    {
      "name": "Reports",
      "path": "Documents/Reports",
      "folder_id": "uuid2",
      "level": 1
    },
    {
      "name": "2024",
      "path": "Documents/Reports/2024",
      "folder_id": "uuid3",
      "level": 2
    }
  ],
  "contents": [
    {
      "id": "uuid4",
      "name": "january.pdf",
      "size": 1024,
      "is_folder": false
    }
  ],
  "total_items": 1,
  "current_folder_id": "uuid3"
}
```

### Поиск и фильтры

#### Поиск файлов
```http
GET /files/search?q=document
Authorization: Bearer <token>
```

**Параметры запроса:**
- `q` (required) - Поисковый запрос

**Ответ:**
```json
[
  {
    "id": "uuid",
    "name": "document.pdf",
    "size": 1024,
    "is_folder": false
  }
]
```

#### Избранные файлы
```http
GET /files/starred
Authorization: Bearer <token>
```

**Ответ:**
```json
[
  {
    "id": "uuid",
    "name": "important_document.pdf",
    "starred": true,
    "size": 1024
  }
]
```

#### Удаленные файлы
```http
GET /files/trashed
Authorization: Bearer <token>
```

**Ответ:**
```json
[
  {
    "id": "uuid",
    "name": "deleted_file.pdf",
    "is_trashed": true,
    "trashed_at": "2023-01-01T00:00:00Z"
  }
]
```

### Ревизии файлов

#### Список ревизий
```http
GET /files/{id}/revisions
Authorization: Bearer <token>
```

**Ответ:**
```json
[
  {
    "id": "uuid",
    "revision_id": 1,
    "size": 1024,
    "created_at": "2023-01-01T00:00:00Z"
  }
]
```

#### Получение ревизии
```http
GET /files/{id}/revisions/{revisionId}
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "id": "uuid",
  "revision_id": 1,
  "size": 1024,
  "storage_path": "storage/users/uuid/filename_v1.pdf",
  "created_at": "2023-01-01T00:00:00Z"
}
```

#### Восстановление ревизии
```http
POST /files/{id}/revisions/{revisionId}/restore
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "Revision restored successfully"
}
```

### Права доступа

#### Список прав доступа
```http
GET /files/{id}/permissions
Authorization: Bearer <token>
```

**Ответ:**
```json
[
  {
    "id": "uuid",
    "grantee_id": "uuid",
    "role": "reader",
    "granted_by": "uuid",
    "created_at": "2023-01-01T00:00:00Z"
  }
]
```

#### Предоставление прав
```http
POST /files/{id}/permissions
Content-Type: application/json
Authorization: Bearer <token>

{
  "grantee_id": "user-uuid",
  "role": "reader"
}
```

**Ответ:**
```json
{
  "message": "Permission granted successfully"
}
```

#### Отзыв прав
```http
DELETE /files/{id}/permissions/{granteeId}
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "Permission revoked successfully"
}
```

### Специальные операции с файлами

#### Добавить в избранное
```http
POST /files/{id}/star
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "File starred successfully"
}
```

#### Убрать из избранного
```http
POST /files/{id}/unstar
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "File unstarred successfully"
}
```

#### Переместить файл
```http
POST /files/{id}/move
Content-Type: application/json
Authorization: Bearer <token>

{
  "new_parent_id": "uuid-optional"
}
```

**Ответ:**
```json
{
  "message": "File moved successfully"
}
```

#### Копировать файл
```http
POST /files/{id}/copy
Content-Type: application/json
Authorization: Bearer <token>

{
  "new_parent_id": "uuid-optional",
  "new_name": "copy_of_file.pdf"
}
```

**Ответ:**
```json
{
  "id": "uuid",
  "name": "copy_of_file.pdf",
  "created_at": "2023-01-01T00:00:00Z"
}
```

#### Переименовать файл
```http
POST /files/{id}/rename
Content-Type: application/json
Authorization: Bearer <token>

{
  "new_name": "renamed_file.pdf"
}
```

**Ответ:**
```json
{
  "message": "File renamed successfully"
}
```

### Метаданные файлов

#### Получить метаданные
```http
GET /files/{id}/metadata
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "author": "John Doe",
  "description": "Important document",
  "tags": ["work", "important"],
  "created_date": "2023-01-01"
}
```

#### Обновить метаданные
```http
PUT /files/{id}/metadata
Content-Type: application/json
Authorization: Bearer <token>

{
  "author": "John Doe",
  "description": "Important document",
  "tags": ["work", "important"]
}
```

**Ответ:**
```json
{
  "message": "File metadata updated successfully"
}
```

### Проверка целостности

#### Проверить целостность файла
```http
POST /files/{id}/verify
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "is_integrity_verified": true
}
```

#### Вычислить контрольные суммы
```http
POST /files/{id}/checksums
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "File checksums calculated successfully"
}
```

### Управление хранилищем

#### Информация о хранилище
```http
GET /storage/info
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "available_space": 107374182400
}
```

#### Очистка хранилища
```http
POST /storage/cleanup
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "Storage cleanup completed successfully"
}
```

#### Оптимизация хранилища
```http
POST /storage/optimize
Authorization: Bearer <token>
```

**Ответ:**
```json
{
  "message": "Storage optimization completed successfully"
}
```

### Проверка здоровья сервиса
```http
GET /health
```

**Ответ:**
```json
{
  "status": "ok",
  "timestamp": "2023-01-01T00:00:00Z"
}
```

## Ошибки

### Стандартные ошибки

#### 400 Bad Request
```json
{
  "error": "Invalid request body"
}
```

#### 401 Unauthorized
```json
{
  "error": "Unauthorized"
}
```

#### 403 Forbidden
```json
{
  "error": "Access denied"
}
```

#### 404 Not Found
```json
{
  "error": "File not found"
}
```

#### 409 Conflict
```json
{
  "error": "File already exists"
}
```

#### 500 Internal Server Error
```json
{
  "error": "Internal server error"
}
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