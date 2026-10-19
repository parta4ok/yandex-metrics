package application

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type BackupMaker interface {
	MakeBackup(ctx context.Context) error
}

type BackupTicker struct {
	interval    time.Duration
	backupMaker BackupMaker
	logger      toolkitlogger.Logger
}

func NewBackupTicker(
	interval time.Duration,
	backupMaker BackupMaker,
	logger toolkitlogger.Logger,
) (*BackupTicker, error) {
	if interval <= 0 {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. interval is not positive")
	}
	if backupMaker == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. backup maker is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup ticker. logger is nil")
	}

	return &BackupTicker{
		interval:    interval,
		backupMaker: backupMaker,
		logger:      logger,
	}, nil
}

func (t *BackupTicker) Start(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := t.backupMaker.MakeBackup(ctx); err != nil {
				t.logger.Warn("backup ticker. make backup failed", "error", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *BackupTicker) Stop(ctx context.Context) error {
	if err := t.backupMaker.MakeBackup(ctx); err != nil {
		return errors.Wrap(err, "stop backup ticker. make final backup")
	}

	return nil
}
