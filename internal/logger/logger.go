package logger

import (
	"context"
	"fmt"

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

// GetLoggerFromCtxSafe безопасно получает logger из контекста, возвращает nil если logger не найден
func GetLoggerFromCtxSafe(ctx context.Context) *Logger {
	if ctx == nil {
		return nil
	}

	value := ctx.Value(LoggerKey)
	if value == nil {
		return nil
	}

	if logger, ok := value.(*Logger); ok {
		return logger
	}

	return nil
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	if ctx.Value(RequestID) != nil {
		fields = append(fields, zap.String(RequestID, ctx.Value(RequestID).(string)))
	}

	l.l.Info(msg, fields...)
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
}
