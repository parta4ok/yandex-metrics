package application

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type Backuper interface {
	MakeBackup(ctx context.Context) error
}

type BackupTicker struct {
	interval time.Duration
	backuper Backuper
	logger   toolkitlogger.Logger
}

func NewBackupTicker(
	interval time.Duration,
	backuper Backuper,
	logger toolkitlogger.Logger,
) (*BackupTicker, error) {
	if interval <= 0 {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. interval is not positive")
	}
	if backuper == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. backuper is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. logger is nil")
	}

	return &BackupTicker{
		interval: interval,
		backuper: backuper,
		logger:   logger,
	}, nil
}

func (t *BackupTicker) Start(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := t.backuper.MakeBackup(ctx); err != nil {
				t.logger.Warn("backup ticker. make backup failed", "error", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *BackupTicker) Stop(ctx context.Context) error {
	if err := t.backuper.MakeBackup(ctx); err != nil {
		return errors.Wrap(err, "stop backup ticker. make final backup")
	}

	return nil
}
