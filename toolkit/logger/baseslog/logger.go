package baseslog

import "log/slog"

import toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"

type Logger struct{}

var _ toolkitlogger.Logger = (*Logger)(nil)

func New() *Logger {
	return &Logger{}
}

func (l *Logger) Debug(message string, fields ...any) {
	slog.Debug(message, fields...)
}

func (l *Logger) Info(message string, fields ...any) {
	slog.Info(message, fields...)
}

func (l *Logger) Warn(message string, fields ...any) {
	slog.Warn(message, fields...)
}

func (l *Logger) Error(message string, fields ...any) {
	slog.Error(message, fields...)
}
