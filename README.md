# HomeCloud File Service

Файловый сервис для системы HomeCloud, предоставляющий REST API для управления файлами и папками с поддержкой версионирования, прав доступа и проверки целостности.

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
│   ├── errdefs/
│   │   └── errdefs.go             # Определения ошибок
│   ├── interfaces/
│   │   ├── repository.go          # Интерфейсы репозиториев
│   │   └── service.go             # Интерфейсы сервисов
│   ├── logger/
│   │   └── logger.go              # Логирование
│   ├── models/
│   │   └── file_model.go          # Модели данных
│   ├── repository/
│   │   ├── file_repository.go     # Репозиторий файлов
│   │   └── storage_repository.go  # Репозиторий хранилища (с логикой из Python)
│   ├── service/
│   │   ├── file_service.go        # Сервис файлов
│   │   └── storage_service.go     # Сервис хранилища
│   └── transport/
│       ├── grpc/
│       │   └── protos/
│       │       ├── auth_grpc.pb.go # Сгенерированный gRPC код
│       │       ├── auth.pb.go
│       │       └── auth.proto     # Proto файл для аутентификации
│       └── http/
│           └── api/
│               └── handler.go     # HTTP обработчики (с логикой из Python)
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
4. **Models** (`internal/models/`) - Структуры данных
5. **Interfaces** (`internal/interfaces/`) - Контракты между слоями

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

## API Endpoints

### Прямые маршруты (как в Python скриптах)
- `POST /api/v1/upload` - Загрузить файл
- `GET /api/v1/download` - Скачать файл

### Возобновляемые операции
- `POST /api/v1/upload/resumable` - Инициализация возобновляемой загрузки
- `POST /api/v1/upload/resumable/{sessionID}` - Загрузка части файла
- `GET /api/v1/download/resumable` - Инициализация возобновляемого скачивания
- `GET /api/v1/download/resumable/{sessionID}` - Скачивание части файла

### Файлы
- `POST /api/v1/files` - Создать файл
- `GET /api/v1/files` - Список файлов
- `GET /api/v1/files/{id}` - Получить файл
- `PUT /api/v1/files/{id}` - Обновить файл
- `DELETE /api/v1/files/{id}` - Удалить файл
- `POST /api/v1/files/{id}/restore` - Восстановить файл

### Загрузка и скачивание по ID
- `POST /api/v1/files/{id}/upload` - Загрузить файл по ID
- `GET /api/v1/files/{id}/download` - Скачать файл по ID
- `GET /api/v1/files/{id}/content` - Получить содержимое

### Папки
- `POST /api/v1/folders` - Создать папку
- `GET /api/v1/folders/{id}/contents` - Содержимое папки

### Поиск и фильтры
- `GET /api/v1/files/search` - Поиск файлов
- `GET /api/v1/files/starred` - Избранные файлы
- `GET /api/v1/files/trashed` - Удаленные файлы

### Ревизии
- `GET /api/v1/files/{id}/revisions` - Список ревизий
- `GET /api/v1/files/{id}/revisions/{revisionId}` - Получить ревизию
- `POST /api/v1/files/{id}/revisions/{revisionId}/restore` - Восстановить ревизию

### Права доступа
- `GET /api/v1/files/{id}/permissions` - Список прав
- `POST /api/v1/files/{id}/permissions` - Предоставить права
- `DELETE /api/v1/files/{id}/permissions/{granteeId}` - Отозвать права

### Специальные операции
- `POST /api/v1/files/{id}/star` - Добавить в избранное
- `POST /api/v1/files/{id}/unstar` - Убрать из избранного
- `POST /api/v1/files/{id}/move` - Переместить файл
- `POST /api/v1/files/{id}/copy` - Копировать файл
- `POST /api/v1/files/{id}/rename` - Переименовать файл

### Метаданные
- `GET /api/v1/files/{id}/metadata` - Получить метаданные
- `PUT /api/v1/files/{id}/metadata` - Обновить метаданные

### Целостность
- `POST /api/v1/files/{id}/verify` - Проверить целостность
- `POST /api/v1/files/{id}/checksums` - Вычислить контрольные суммы

### Хранилище
- `GET /api/v1/storage/info` - Информация о хранилище
- `POST /api/v1/storage/cleanup` - Очистка хранилища
- `POST /api/v1/storage/optimize` - Оптимизация хранилища

## Примеры использования

### Загрузка файла
```bash
curl -X POST "http://localhost:8080/api/v1/upload" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"filePath": "documents/report.pdf"}' \
  --data-binary @report.pdf
```

### Скачивание файла
```bash
curl -X GET "http://localhost:8080/api/v1/download" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"filePath": "documents/report.pdf"}' \
  -o downloaded_report.pdf
```

### Возобновляемая загрузка
```bash
# 1. Инициализация сессии
curl -X POST "http://localhost:8080/api/v1/upload/resumable" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"filePath": "large_file.zip", "size": 1048576, "sha256": "hash"}'

# 2. Загрузка частей
curl -X POST "http://localhost:8080/api/v1/upload/resumable/SESSION_ID" \
  -H "Content-Type: application/octet-stream" \
  -H "Content-Range: bytes 0-1048575/1048576" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  --data-binary @chunk1.bin
```

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
