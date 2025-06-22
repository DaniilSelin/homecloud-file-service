# HomeCloud File Service - gRPC Реализация

## 🎯 Что было реализовано

### 1. gRPC API для файлового сервиса

Создан полноценный gRPC сервис с методом для создания директорий пользователей:

#### Proto файл: `internal/transport/grpc/protos/file_service.proto`
```protobuf
service FileService {
    // Создание директории для пользователя при регистрации
    rpc CreateUserDirectory(CreateUserDirectoryRequest) returns (CreateUserDirectoryResponse);
}
```

#### Методы:
- **CreateUserDirectory** - создает директорию пользователя со стандартными подпапками

### 2. gRPC Сервер

#### Файл: `internal/transport/grpc/server.go`
- Реализация `FileServiceServer`
- Метод `CreateUserDirectory` создает:
  - Основную директорию пользователя: `storage/users/{user-id}/`
  - Стандартные поддиректории: `documents`, `photos`, `videos`, `music`, `downloads`

### 3. Интеграция с основным сервером

#### Файл: `cmd/server/main.go`
- Добавлен gRPC сервер на порту 9090 (настраивается в конфигурации)
- HTTP сервер на порту 8082
- Graceful shutdown для обоих серверов

### 4. Тестовый клиент

#### Файл: `test_grpc_client.go`
- gRPC клиент для тестирования
- Демонстрирует вызов `CreateUserDirectory`

## 🚀 Результаты тестирования

### ✅ Успешно протестировано:

1. **gRPC сервер запускается** на порту 9090
2. **HTTP сервер работает** на порту 8082
3. **CreateUserDirectory метод работает**:
   ```
   Success: true
   Message: User directory created successfully
   Directory Path: storage/users/550e8400-e29b-41d4-a716-446655440000
   ```
4. **Создаются стандартные директории**:
   ```
   documents/
   downloads/
   music/
   photos/
   videos/
   ```
5. **Возобновляемая загрузка файлов работает**:
   - Инициализация сессии: ✅
   - Загрузка файла: ✅
   - Session ID: `597cda08-6c31-4ce9-87ea-faaac07a5279`

## 📋 Конфигурация

### gRPC настройки в `config/config.local.yaml`:
```yaml
grpc:
  host: "localhost"
  port: 9090
```

### HTTP настройки:
```yaml
server:
  host: "0.0.0.0"
  port: 8082
```

## 🔧 Использование

### Запуск сервиса:
```bash
go run ./cmd/server
```

### Тестирование gRPC:
```bash
go run test_grpc_client.go
```

### Тестирование HTTP API:
```bash
./test_resumable_upload.sh
```

## 🌐 API Endpoints

### gRPC (порт 9090):
- `CreateUserDirectory` - создание директории пользователя

### HTTP (порт 8082):
- `GET /health` - проверка состояния
- `POST /api/v1/upload/resumable` - инициализация возобновляемой загрузки
- `POST /api/v1/upload/resumable/{sessionID}` - загрузка части файла
- `GET /api/v1/download/resumable` - инициализация возобновляемого скачивания

## 📁 Структура директорий

После вызова `CreateUserDirectory` для пользователя `testuser`:
```
storage/
└── users/
    └── 550e8400-e29b-41d4-a716-446655440000/
        ├── documents/
        ├── downloads/
        ├── music/
        ├── photos/
        └── videos/
```

## 🔐 Интеграция с Auth Service

### Готово для интеграции:
- gRPC клиент для auth-service настроен
- Тестовый режим для разработки
- Поддержка JWT токенов

### Для production:
1. Настроить реальный auth-service
2. Убрать тестовый режим
3. Добавить валидацию токенов

## 📝 Следующие шаги

### Для интеграции с auth-service:
1. Вызывать `CreateUserDirectory` при регистрации пользователя
2. Передавать `user_id` и `username` из auth-service
3. Обрабатывать ошибки создания директорий

### Дополнительные методы gRPC:
1. `DeleteUserDirectory` - удаление директории при удалении пользователя
2. `GetUserStorageInfo` - информация о хранилище пользователя
3. `CleanupUserFiles` - очистка файлов пользователя

## 🎉 Заключение

gRPC сервис для файлового сервиса успешно реализован и протестирован. Метод `CreateUserDirectory` готов к интеграции с auth-service для автоматического создания директорий пользователей при регистрации. 