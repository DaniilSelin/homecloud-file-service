package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"homecloud-file-service/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	RequestID = "RequestID"
	LoggerKey = "logger"
)

type Logger struct {
	l *zap.Logger
}

func New(cfg *config.Config) (*Logger, error) {
	// Добавляем энкодер времени вручную
	// Это функция, поэтому из yml его не достать
	cfg.Logger.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Logger.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	logger, err := cfg.Logger.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Logger{l: logger}, nil
}

func CtxWWithLogger(ctx context.Context, lg *Logger) context.Context {
	ctx = context.WithValue(ctx, LoggerKey, lg)
	return ctx
}

func GetLoggerFromCtx(ctx context.Context) *Logger {
	return ctx.Value(LoggerKey).(*Logger)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	if ctx.Value(RequestID) != nil {
		fields = append(fields, zap.String(RequestID, ctx.Value(RequestID).(string)))
	}

	l.l.Info(msg, fields...)

	go func() {
		logData := map[string]interface{}{
			"level":   "info",
			"message": msg,
			"fields":  fieldsToMap(fields),
		}
		sendLog(logData)
	}()
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if ctx.Value(RequestID) != nil {
		fields = append(fields, zap.String(RequestID, ctx.Value(RequestID).(string)))
	}

	l.l.Debug(msg, fields...)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...zap.Field) {
	if ctx.Value(RequestID) != nil {
		fields = append(fields, zap.String(RequestID, ctx.Value(RequestID).(string)))
	}

	l.l.Error(msg, fields...)

	go func() {
		logData := map[string]interface{}{
			"level":   "error",
			"message": msg,
			"fields":  fieldsToMap(fields),
		}
		sendLog(logData)
	}()
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if ctx.Value(RequestID) != nil {
		fields = append(fields, zap.String(RequestID, ctx.Value(RequestID).(string)))
	}

	l.l.Warn(msg, fields...)
}

func fieldsToMap(fields []zap.Field) map[string]interface{} {
	result := make(map[string]interface{})
	for _, field := range fields {
		result[field.Key] = field.Interface
	}
	return result
}

func sendLog(data map[string]interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", "http://localhost:8085/logcatcher", bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Используем http.Client с таймаутом, но не ждем ответ
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// игнорируем ответ
	go client.Do(req)
}
