# HomeCloud File Service - Быстрый старт

## 🚀 Быстрый запуск

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

## 🧪 Тестирование

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
curl http://localhost:8080/api/v1/health

# Загрузка файла (замените TOKEN на реальный токен)
curl -X POST "http://localhost:8080/api/v1/upload" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"filePath": "test/file.txt"}' \
  --data-binary @file.txt
```

## 📁 Структура проекта

```
homecloud-file-service/
├── cmd/server/          # Точка входа
├── config/              # Конфигурация
├── internal/            # Внутренняя логика
│   ├── auth/           # Аутентификация
│   ├── models/         # Модели данных
│   ├── repository/     # Доступ к данным
│   ├── service/        # Бизнес-логика
│   └── transport/      # HTTP/gRPC API
├── migrations/         # Миграции БД
├── storage/           # Файловое хранилище
└── docker-compose.yml # Docker конфигурация
```

## 🔧 Основные команды

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

## 🌐 API Endpoints

### Основные маршруты

- `POST /api/v1/upload` - Загрузка файла
- `GET /api/v1/download` - Скачивание файла
- `POST /api/v1/upload/resumable` - Возобновляемая загрузка
- `GET /api/v1/download/resumable` - Возобновляемое скачивание

### Управление файлами

- `POST /api/v1/files` - Создать файл
- `GET /api/v1/files` - Список файлов
- `GET /api/v1/files/{id}` - Получить файл
- `PUT /api/v1/files/{id}` - Обновить файл
- `DELETE /api/v1/files/{id}` - Удалить файл

## 🔐 Аутентификация

Сервис использует JWT токены для аутентификации. Токен должен передаваться в заголовке:

```
Authorization: Bearer <your-jwt-token>
```

## 📊 Мониторинг

- **Health Check**: `GET /api/v1/health`
- **Логи**: Структурированные логи в JSON формате
- **Метрики**: Prometheus метрики (планируется)

## 🐛 Отладка

### Просмотр логов

```bash
# Docker логи
docker-compose logs -f file-service

# Локальные логи
tail -f logs/app.log
```

### Проверка состояния

```bash
# Проверка подключения к БД
docker-compose exec postgres psql -U postgres -d homecloud_files -c "SELECT version();"

# Проверка файлового хранилища
ls -la storage/users/
```

## 📝 Следующие шаги

1. Настройте интеграцию с auth-service
2. Добавьте реальные тесты
3. Настройте мониторинг и логирование
4. Добавьте CI/CD pipeline
5. Настройте production конфигурацию

## 🤝 Поддержка

При возникновении проблем:

1. Проверьте логи сервиса
2. Убедитесь, что все зависимости запущены
3. Проверьте конфигурацию в `config/config.local.yaml`
4. Создайте issue в репозитории 