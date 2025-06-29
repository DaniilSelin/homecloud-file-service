File Service API Documentation (Updated)
Реализованные и протестированные эндпоинты
Основные операции с файлами
POST /api/v1/files/upload/resumable

    Описание: Инициализация сессии для возобновляемой загрузки

    Тело запроса:
    json

{
  "filePath": "example.txt",
  "size": 12345,
  "sha256": "hex_string_checksum"
}

Ответ:
json

    {
      "session_id": "uuid-session-id"
    }

    Статусы:

        201: Сессия создана

        400: Неверный запрос

        401: Не авторизован

    Тест: TestResumableUploadAPI

PATCH /api/v1/files/upload/resumable/{sessionID}

    Описание: Загрузка части файла (чанка)

    Параметры пути:

        sessionID - UUID сессии, полученный при инициализации

    Заголовки:

        Content-Range: bytes <start>-<end>/<total>

        Content-Type: application/octet-stream

    Тело: Бинарные данные чанка

    Ответ:

        При успешной загрузке чанка:
        json

{
  "message": "Chunk uploaded successfully",
  "range": "bytes <start>-<end>/<total>"
}

При завершении загрузки:
json

        {
          "message": "Upload completed successfully",
          "file": {
            "id": "file-uuid",
            "name": "example.txt",
            "size": 12345,
            "created_at": "2023-01-01T00:00:00Z"
          }
        }

    Статусы:

        202: Чанк успешно загружен (загрузка не завершена)

        200: Файл полностью загружен

        400: Ошибка в Content-Range или несоответствие размера

        401: Не авторизован

        404: Сессия не найдена

        409: Конфликт имен файлов

    Тест: TestResumableUploadAPI

GET /api/v1/files/{id}/download

    Описание: Скачивание файла

    Параметры пути:

        id - UUID файла

    Заголовки ответа:

        Content-Type: MIME-тип файла

        Content-Length: размер файла в байтах

        Content-Disposition: attachment; filename="имя_файла"

    Тело ответа: Бинарное содержимое файла

    Статусы:

        200: Успешно

        401: Не авторизован

        404: Файл не найден

    Тест: TestFileOperations/Download_File

Операции с папками
POST /api/v1/folders/upload

    Описание: Загрузка папки в виде ZIP-архива

    Форма данных:

        folder (file): ZIP-архив с содержимым папки

        folderPath (string): Путь для сохранения папки

    Ответ:
    json

    {
      "message": "Folder uploaded successfully",
      "folder": {
        "id": "folder-uuid",
        "name": "folder_name",
        "path": "/target/path"
      }
    }

    Статусы:

        201: Папка успешно создана

        400: Ошибка в запросе

        401: Не авторизован

    Тест: TestFolderOperations/Upload_Folder

GET /api/v1/folders/download

    Описание: Скачивание папки в виде ZIP-архива

    Параметры запроса:

        path (string): Путь к папке для скачивания

    Заголовки ответа:

        Content-Type: application/zip

        Content-Disposition: attachment; filename="folder_name.zip"

    Тело ответа: ZIP-архив с содержимым папки

    Статусы:

        200: Успешно

        400: Неверный путь

        401: Не авторизован

        404: Папка не найдена

    Тест: TestFolderOperations/Download_Folder

Стандартные операции
POST /api/v1/files

    Описание: Создание файла/папки

    Тело запроса:
    json

{
  "name": "file.txt",
  "content": "base64_encoded_content",
  "mime_type": "text/plain",
  "size": 123,
  "is_folder": false,
  "parent_id": "optional-folder-uuid"
}

Ответ:
json

    {
      "id": "file-uuid",
      "name": "file.txt",
      "size": 123,
      "is_folder": false,
      "created_at": "2023-01-01T00:00:00Z"
    }

    Статусы:

        201: Успешно создано

        400: Неверный запрос

        401: Не авторизован

        409: Конфликт имен

    Тесты:

        TestFileOperations/Create_Folder

        TestFileOperations/Create_File_in_Folder

GET /api/v1/files

    Описание: Получение списка файлов

    Параметры:

        parent_id (query): UUID родительской папки

    Ответ:
    json

    {
      "files": [
        {
          "id": "file-uuid",
          "name": "file.txt",
          "size": 123,
          "is_folder": false,
          "created_at": "2023-01-01T00:00:00Z"
        }
      ],
      "total": 1
    }

    Статусы:

        200: Успешно

        401: Не авторизован

    Тест: TestFileOperations/List_Files_in_Folder

DELETE /api/v1/files/{id}

    Описание: Удаление файла/папки

    Статусы:

        200: Успешно удалено

        401: Не авторизован

        404: Не найдено

    Тесты:

        TestFileOperations/Delete_File

        TestFileOperations/Delete_Folder

Особенности реализации
Возобновляемая загрузка

    Инициализация: Клиент отправляет метаданные файла

    Загрузка чанков: Файл делится на части фиксированного размера

    Валидация:

        Проверка контрольной суммы SHA256

        Проверка соответствия размеров

        Проверка целостности чанков

    Завершение: После последнего чанка файл сохраняется в системе

Обработка конфликтов

При конфликте имен возвращается структура:
json

{
  "error": "File with this name already exists",
  "details": {
    "fileName": "example.txt",
    "suggestion": "example (1).txt",
    "message": "A file named 'example.txt' already exists. Try 'example (1).txt' instead."
  }
}

Безопасность

    Все запросы требуют JWT-токен в заголовке Authorization: Bearer <token>

    Проверка прав доступа к сессиям и файлам

    Валидация входных данных

Статус тестирования

✅ Полностью протестировано:

    Создание файлов и папок (POST /files)

    Листинг файлов (GET /files)

    Удаление файлов и папок (DELETE /files/{id})

    Загрузка файлов (POST /files/upload/resumable)

    Загрузка чанков (PATCH /files/upload/resumable/{sessionID})

    Скачивание файлов (GET /files/{id}/download)

    Загрузка папок (POST /folders/upload)

    Скачивание папок (GET /folders/download)

⚠️ Требует дополнительного тестирования:

    Обработка очень больших файлов (>1GB)

    Параллельная загрузка чанков

    Восстановление прерванных сессий

    Пограничные случаи с Content-Range

❌ Не реализовано:

    Частичное скачивание (Range-запросы)

    Перемещение/переименование файлов

    Управление правами доступа

    Версионность файлов

    Поиск по содержимому