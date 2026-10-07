package noop

import toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"

type Logger struct{}

var _ toolkitlogger.Logger = (*Logger)(nil)

func New() *Logger {
	return &Logger{}
}

func (l *Logger) Debug(_ string, _ ...any) {}

func (l *Logger) Info(_ string, _ ...any) {}

func (l *Logger) Warn(_ string, _ ...any) {}

func (l *Logger) Error(_ string, _ ...any) {}
