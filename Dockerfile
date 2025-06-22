# Многоэтапная сборка для оптимизации размера образа
FROM golang:1.23-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git ca-certificates tzdata

# Установка рабочей директории
WORKDIR /app

# Копирование файлов зависимостей
COPY go.mod go.sum ./

# Загрузка зависимостей
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# Финальный образ
FROM alpine:latest

# Установка необходимых пакетов
RUN apk --no-cache add ca-certificates tzdata

# Создание пользователя для безопасности
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Установка рабочей директории
WORKDIR /app

# Копирование бинарного файла из этапа сборки
COPY --from=builder /app/main .

# Копирование конфигурационных файлов
COPY --from=builder /app/config ./config

# Создание директории для файлов
RUN mkdir -p /app/storage && \
    chown -R appuser:appgroup /app

# Переключение на непривилегированного пользователя
USER appuser

# Открытие порта
EXPOSE 8080

# Проверка здоровья
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Запуск приложения
CMD ["./main"] 