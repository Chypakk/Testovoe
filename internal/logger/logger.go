package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func Init() {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Добавляем контекстные поля по умолчанию
	Logger = Logger.With(slog.String("service", "task-manager"), slog.String("version", "1.0"))

	Logger.Info("Logger initialized")
}

// GetLogger возвращает экземпляр логгера с дополнительными полями
func GetLogger(attrs ...any) *slog.Logger {
	if Logger == nil {
		Init()
	}
	return Logger.With(attrs...)
}

// Info логирует информационное сообщение
func Info(message string, attrs ...any) {
	GetLogger(attrs...).Info(message)
}

// Error логирует сообщение об ошибке
func Error(message string, err error, attrs ...any) {
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	GetLogger(attrs...).Error(message)
}

// Debug логирует отладочное сообщение
func Debug(message string, attrs ...any) {
	GetLogger(attrs...).Debug(message)
}

// Warn логирует предупреждение
func Warn(message string, attrs ...any) {
	GetLogger(attrs...).Warn(message)
}

// WithAttrs создает новый логгер с дополнительными атрибутами
func WithAttrs(attrs ...any) *slog.Logger {
	return GetLogger(attrs...)
}