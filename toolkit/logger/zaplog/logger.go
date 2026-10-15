package zaplog

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

var ErrInternal = errors.New("internal error")

type Logger struct {
	sugar *zap.SugaredLogger
}

var _ toolkitlogger.Logger = (*Logger)(nil)

func New() (*Logger, error) {
	base, err := zap.NewProduction(zap.AddCallerSkip(1))
	if err != nil {
		return nil, errors.Wrapf(ErrInternal, "new zap logger. build production logger: %v", err)
	}

	return &Logger{sugar: base.Sugar()}, nil
}

func (l *Logger) Debug(message string, fields ...any) {
	l.sugar.Debugw(message, fields...)
}

func (l *Logger) Info(message string, fields ...any) {
	l.sugar.Infow(message, fields...)
}

func (l *Logger) Warn(message string, fields ...any) {
	l.sugar.Warnw(message, fields...)
}

func (l *Logger) Error(message string, fields ...any) {
	l.sugar.Errorw(message, fields...)
}
