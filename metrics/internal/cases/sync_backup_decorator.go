package cases

import (
	"context"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type SyncBackupDecorator struct {
	port.MetricServiceProvider
	logger toolkitlogger.Logger
}

func NewSyncBackupDecorator(
	service port.MetricServiceProvider,
	logger toolkitlogger.Logger,
) (*SyncBackupDecorator, error) {
	if service == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new sync backup decorator. service is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new sync backup decorator. logger is nil")
	}

	return &SyncBackupDecorator{
		MetricServiceProvider: service,
		logger:                logger,
	}, nil
}

func (s *SyncBackupDecorator) UpdateMetric(
	ctx context.Context,
	metric *entities.Metric,
) (*entities.Metric, error) {
	stored, err := s.MetricServiceProvider.UpdateMetric(ctx, metric)
	if err != nil {
		return nil, errors.Wrap(err, "update metric. update service")
	}

	if err := s.MakeBackup(ctx); err != nil {
		s.logger.Warn("update metric. make backup failed", "error", err)
	}

	return stored, nil
}
