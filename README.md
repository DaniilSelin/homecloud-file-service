# HomeCloud File Service

Файловый сервис для системы HomeCloud, предоставляющий REST API для управления файлами и папками с поддержкой версионирования, прав доступа, навигации по папкам и проверки целостности.

## Архитектура

### Структура проекта

```
homecloud-file-service/
├── cmd/
│   └── server/
│       └── main.go                 # Точка входа приложения
├── config/
│   ├── config.go                   # Конфигурация приложения
│   ├── config.example.yaml         # Пример конфигурации
│   └── config.local.yaml          # Локальная конфигурация
├── internal/
│   ├── auth/
│   │   ├── grpc_client.go          # gRPC клиент для аутентификации
│   │   └── middleware.go           # Middleware для аутентификации
│   ├── dbmanager/
│   │   └── grpc_client.go          # gRPC клиент для работы с БД
│   ├── errdefs/
│   │   └── errdefs.go             # Определения ошибок
│   ├── interfaces/
│   │   ├── repository.go          # Интерфейсы репозиториев
│   │   ├── service.go             # Интерфейсы сервисов
│   │   └── dbmanager.go           # Интерфейс DBManager клиента
│   ├── logger/
│   │   └── logger.go              # Логирование
│   ├── models/
│   │   └── file_model.go          # Модели данных
│   ├── repository/
│   │   ├── file_repository.go     # Репозиторий файлов
│   │   └── storage_repository.go  # Репозиторий хранилища
│   ├── service/
│   │   ├── file_service.go        # Сервис файлов
│   │   └── storage_service.go     # Сервис хранилища
│   └── transport/
│       ├── grpc/
│       │   ├── protos/
│       │   │   ├── auth_grpc.pb.go # Сгенерированный gRPC код
│       │   │   ├── auth.pb.go
│       │   │   ├── auth.proto     # Proto файл для аутентификации
│       │   │   ├── db_service_grpc.pb.go
│       │   │   ├── db_service.pb.go
│       │   │   ├── db_service.proto # Proto файл для работы с БД
│       │   │   ├── file_service_grpc.pb.go
│       │   │   ├── file_service.pb.go
│       │   │   └── file_service.proto # Proto файл для файлового сервиса
│       │   └── server.go          # gRPC сервер
│       └── http/
│           └── api/
│               └── handler.go     # HTTP обработчики
├── migrations/
│   ├── 001_create_files_table.up.sql
│   ├── 001_create_files_table.down.sql
│   ├── 002_create_file_revisions_table.up.sql
│   ├── 002_create_file_revisions_table.down.sql
│   ├── 003_create_file_permissions_table.up.sql
│   └── 003_create_file_permissions_table.down.sql
├── storage/
│   └── users/                     # Директория для файлов пользователей
├── temp/                          # Временная директория для загрузок
├── Dockerfile                     # Docker образ
├── docker-compose.yml            # Docker Compose
├── Makefile                      # Команды для разработки
├── go.mod                        # Go модули
├── test_upload_download.sh       # Тестовый скрипт
└── README.md                     # Документация
```

### Слои архитектуры

1. **Transport Layer** (`internal/transport/`) - HTTP API, gRPC
2. **Service Layer** (`internal/service/`) - Бизнес-логика
3. **Repository Layer** (`internal/repository/`) - Доступ к данным
4. **DBManager Client** (`internal/dbmanager/`) - gRPC клиент для работы с БД
5. **Models** (`internal/models/`) - Структуры данных
6. **Interfaces** (`internal/interfaces/`) - Контракты между слоями

## Логика хранения файлов

Сервис использует логику, перенесенную из Python скриптов:

### Структура директорий
```
storage/
├── users/                    # Основная директория пользователей
│   ├── {user-id-1}/         # Директория пользователя 1
│   │   ├── documents/       # Папки пользователя
│   │   ├── photos/
│   │   └── file.txt
│   └── {user-id-2}/         # Директория пользователя 2
│       └── ...
└── temp/                    # Временные файлы для загрузок
```

### Особенности реализации

1. **Изоляция пользователей**: Каждый пользователь имеет свою директорию по UUID
2. **Валидация путей**: Защита от path traversal атак
3. **Создание директорий**: Автоматическое создание папок пользователей
4. **Оригинальные файлы**: Файлы хранятся в исходном виде без шифрования
5. **Прямое взаимодействие с ОС**: Репозиторий работает напрямую с файловой системой
6. **gRPC интеграция**: Все операции с БД выполняются через gRPC DBManager сервис
7. **Автоматическое создание папок**: При загрузке файла по пути система автоматически создает все необходимые папки
8. **Навигация по путям**: Поддержка навигации как по ID папок, так и по путям

## API Endpoints

Сервис предоставляет REST API для управления файлами и папками. Все эндпоинты требуют аутентификации через Bearer токен.

### Основные группы эндпоинтов:

- **Файлы**: Создание, чтение, обновление, удаление файлов
- **Папки**: Создание папок и просмотр их содержимого с поддержкой навигации
- **Загрузка/скачивание**: Улучшенная загрузка через multipart/form-data и скачивание по путям
- **Навигация**: Просмотр папок с детализацией и breadcrumbs
- **Поиск и фильтры**: Поиск файлов, избранное, корзина
- **Ревизии**: Управление версиями файлов
- **Права доступа**: Предоставление и отзыв прав доступа
- **Метаданные**: Работа с метаданными файлов
- **Целостность**: Проверка целостности и контрольные суммы
- **Хранилище**: Управление хранилищем

**Полное описание всех эндпоинтов**: [API Reference](API_REFERENCE.md)

## Быстрый старт

Для быстрого запуска и тестирования сервиса см. [QUICKSTART.md](QUICKSTART.md)

## Примеры использования

### Загрузка файла с автоматическим созданием папок
```bash
curl -X POST "http://localhost:8082/api/v1/upload" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@report.pdf" \
  -F "filePath=Documents/Reports/2024/january.pdf"
```

### Создание файла
```bash
curl -X POST "http://localhost:8082/api/v1/files" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "document.pdf",
    "mime_type": "application/pdf",
    "size": 1024
  }'
```

### Создание папки
```bash
curl -X POST "http://localhost:8082/api/v1/folders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "Documents",
    "parent_id": "uuid-optional"
  }'
```

### Просмотр содержимого папки с детализацией
```bash
curl -X GET "http://localhost:8082/api/v1/folders/browse?path=Documents/Reports" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Навигация по пути с breadcrumbs
```bash
curl -X GET "http://localhost:8082/api/v1/folders/navigate?path=Documents/Reports/2024" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Скачивание файла по пути
```bash
curl -X GET "http://localhost:8082/api/v1/download?path=Documents/Reports/2024/january.pdf" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o january.pdf
```

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

## Коды ответов

- `200 OK` - Успешная операция
- `201 Created` - Ресурс создан
- `400 Bad Request` - Неверный запрос
- `401 Unauthorized` - Не авторизован
- `403 Forbidden` - Доступ запрещен
- `404 Not Found` - Ресурс не найден
- `409 Conflict` - Конфликт (например, файл уже существует)
- `500 Internal Server Error` - Внутренняя ошибка сервера

## Установка и запуск

### Предварительные требования

- Go 1.23+
- PostgreSQL 15+
- Redis 7+ (опционально)
- Docker & Docker Compose (опционально)

### Локальная разработка

1. Клонировать репозиторий:
```bash
git clone <repository-url>
cd homecloud-file-service
```

2. Установить зависимости:
```bash
make deps
```

3. Настроить конфигурацию:
```bash
cp config/config.example.yaml config/config.local.yaml
# Отредактировать config/config.local.yaml
```

4. Запустить базу данных:
```bash
docker-compose up postgres redis -d
```

5. Применить миграции:
```bash
make migrate-up
```

6. Запустить сервис:
```bash
make dev
```

### Docker

1. Собрать и запустить все сервисы:
```bash
docker-compose up -d
```

2. Применить миграции:
```bash
docker-compose exec file-service migrate -path migrations -database "postgres://postgres:password@postgres:5432/homecloud_files?sslmode=disable" up
```

### Тестирование

Запустите тестовый скрипт для проверки функциональности:
```bash
chmod +x test_upload_download.sh
./test_upload_download.sh
```

**Примечание**: Не забудьте заменить `AUTH_TOKEN` в скрипте на реальный токен аутентификации.

### Доступные команды

```bash
make help          # Показать справку
make deps          # Установить зависимости
make build         # Собрать приложение
make run           # Запустить приложение
make dev           # Запустить в режиме разработки
make test          # Запустить тесты
make test-coverage # Тесты с покрытием
make clean         # Очистить сборки
make migrate-up    # Применить миграции
make migrate-down  # Откатить миграции
make lint          # Запустить линтер
make fmt           # Форматировать код
make docker-build  # Собрать Docker образ
make docker-run    # Запустить в Docker
```

## Конфигурация

Основные параметры конфигурации:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "password"
  dbname: "homecloud_files"
  sslmode: "disable"

storage:
  base_path: "./storage"      # Базовая директория для хранения
  max_size: 1073741824        # 1GB - максимальный размер файла
  chunk_size: 1048576         # 1MB - размер чанка для возобновляемых загрузок
  temp_path: "./temp"         # Временная директория
  user_dir_name: "users"      # Имя директории для пользователей

logger:
  level: "debug"
  encoding: "console"
  output_paths: ["stdout"]
  error_output_paths: ["stderr"]

auth:
  host: "localhost"
  port: 9092
```

## Разработка

### Добавление новых функций

1. Создать интерфейс в `internal/interfaces/`
2. Реализовать в `internal/repository/` или `internal/service/`
3. Добавить HTTP обработчик в `internal/transport/http/api/`
4. Написать тесты
5. Обновить документацию

### Тестирование

```bash
# Запуск всех тестов
make test

# Тесты с покрытием
make test-coverage

# Запуск конкретного теста
go test -v ./internal/service -run TestCreateFile
```

### Линтинг и форматирование

```bash
# Форматирование кода
make fmt

# Проверка кода
make vet

# Линтинг
make lint
```

## Мониторинг

- **Health Check**: `GET /health`
- **Метрики**: Prometheus метрики (планируется)
- **Логи**: Структурированные логи в JSON формате
- **Трейсинг**: OpenTelemetry (планируется)

## Безопасность

- Аутентификация через JWT токены (gRPC с auth сервисом)
- Авторизация на уровне файлов
- Проверка контрольных сумм
- Валидация входных данных
- Защита от path traversal атак
- Rate limiting (планируется)
- CORS настройки

## Производительность

- Кэширование в Redis
- Пагинация результатов
- Ленивая загрузка метаданных
- Оптимизированные SQL запросы
- Возобновляемые загрузки/скачивания
- Сжатие файлов (планируется)

## Лицензия

MIT License
