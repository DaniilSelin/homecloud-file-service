# Makefile для HomeCloud File Service

.PHONY: help build run test clean deps migrate-up migrate-down docker-build docker-run

# Переменные
BINARY_NAME=homecloud-file-service
BUILD_DIR=build
CONFIG_FILE=config/config.local.yaml

# Цвета для вывода
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

help: ## Показать справку
	@echo "$(GREEN)HomeCloud File Service$(NC)"
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'

deps: ## Установить зависимости
	@echo "$(GREEN)Установка зависимостей...$(NC)"
	go mod download
	go mod tidy

build: deps ## Собрать приложение
	@echo "$(GREEN)Сборка приложения...$(NC)"
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build ## Запустить приложение
	@echo "$(GREEN)Запуск приложения...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME)

dev: ## Запустить в режиме разработки
	@echo "$(GREEN)Запуск в режиме разработки...$(NC)"
	go run ./cmd/server

test: ## Запустить тесты
	@echo "$(GREEN)Запуск тестов...$(NC)"
	go test -v ./...

test-coverage: ## Запустить тесты с покрытием
	@echo "$(GREEN)Запуск тестов с покрытием...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Отчет о покрытии сохранен в coverage.html$(NC)"

clean: ## Очистить сборки
	@echo "$(GREEN)Очистка...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Миграции базы данных
migrate-up: ## Применить миграции
	@echo "$(GREEN)Применение миграций...$(NC)"
	migrate -path migrations -database "postgres://postgres:password@localhost:5432/homecloud_files?sslmode=disable" up

migrate-down: ## Откатить миграции
	@echo "$(GREEN)Откат миграций...$(NC)"
	migrate -path migrations -database "postgres://postgres:password@localhost:5432/homecloud_files?sslmode=disable" down

migrate-force: ## Принудительно установить версию миграции
	@echo "$(GREEN)Принудительная установка версии миграции...$(NC)"
	@read -p "Введите номер версии: " version; \
	migrate -path migrations -database "postgres://postgres:password@localhost:5432/homecloud_files?sslmode=disable" force $$version

# Docker команды
docker-build: ## Собрать Docker образ
	@echo "$(GREEN)Сборка Docker образа...$(NC)"
	docker build -t $(BINARY_NAME) .

docker-run: ## Запустить в Docker
	@echo "$(GREEN)Запуск в Docker...$(NC)"
	docker run -p 8080:8080 -v $(PWD)/storage:/app/storage $(BINARY_NAME)

# Линтинг и форматирование
lint: ## Запустить линтер
	@echo "$(GREEN)Проверка кода...$(NC)"
	golangci-lint run

fmt: ## Форматировать код
	@echo "$(GREEN)Форматирование кода...$(NC)"
	go fmt ./...

vet: ## Проверить код
	@echo "$(GREEN)Проверка кода...$(NC)"
	go vet ./...

# Генерация документации
docs: ## Генерировать документацию API
	@echo "$(GREEN)Генерация документации...$(NC)"
	swag init -g cmd/server/main.go

# Мониторинг
monitor: ## Запустить мониторинг
	@echo "$(GREEN)Запуск мониторинга...$(NC)"
	@echo "Сервис доступен по адресу: http://localhost:8080"
	@echo "API документация: http://localhost:8080/swagger/index.html"
	@echo "Логи: tail -f logs/app.log"

# Установка инструментов разработки
install-tools: ## Установить инструменты разработки
	@echo "$(GREEN)Установка инструментов разработки...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Проверка готовности к продакшену
check: fmt vet test lint ## Проверить готовность к продакшену
	@echo "$(GREEN)Все проверки пройдены!$(NC)"

# Полная сборка для продакшена
release: clean check build ## Полная сборка для продакшена
	@echo "$(GREEN)Сборка для продакшена завершена!$(NC)"
	@echo "Бинарный файл: $(BUILD_DIR)/$(BINARY_NAME)" 