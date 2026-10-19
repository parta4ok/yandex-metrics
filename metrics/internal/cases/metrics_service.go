package cases

import (
	"context"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type MetricsService struct {
	storage MetricsStorage
	backup  BackupProvider
	logger  toolkitlogger.Logger
}

func NewMetricsService(
	storage MetricsStorage,
	backup BackupProvider,
	logger toolkitlogger.Logger,
) (*MetricsService, error) {
	if storage == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. storage is nil")
	}
	if backup == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. backup provider is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. logger is nil")
	}

	return &MetricsService{
		storage: storage,
		backup:  backup,
		logger:  logger,
	}, nil
}

func (s *MetricsService) UpdateMetric(
	ctx context.Context,
	metric *entities.Metric,
) (*entities.Metric, error) {
	if err := metric.Validate(); err != nil {
		return nil, errors.Wrap(err, "update metric. validate metric")
	}

	stored, err := s.storage.UpdateMetric(ctx, metric)
	if err != nil {
		return nil, errors.Wrap(err, "update metric. update storage")
	}

	return stored, nil
}

func (s *MetricsService) GetMetric(
	ctx context.Context,
	id string,
	mType entities.MType,
) (*entities.Metric, error) {
	if id == "" || !mType.IsValid() {
		return nil, errors.Wrap(entities.ErrInvalidParam, "get metric. required parameters are invalid")
	}

	metric, err := s.storage.GetMetric(ctx, id, mType)
	if err != nil {
		return nil, errors.Wrap(err, "get metric. get storage")
	}

	return metric, nil
}

func (s *MetricsService) ListMetrics(ctx context.Context) ([]*entities.Metric, error) {
	metrics, err := s.storage.ListMetrics(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list metrics. list storage")
	}

	return metrics, nil
}

func (s *MetricsService) MakeBackup(ctx context.Context) error {
	metrics, err := s.storage.ListMetrics(ctx)
	if err != nil {
		return errors.Wrap(err, "make backup. list storage")
	}

	if err := s.backup.MakeBackup(ctx, metrics); err != nil {
		return errors.Wrap(err, "make backup. save metrics")
	}

	return nil
}

func (s *MetricsService) RestoreBackup(ctx context.Context) error {
	metrics, err := s.backup.RestoreBackup(ctx)
	if err != nil {
		return errors.Wrap(err, "restore backup. load metrics")
	}

	for _, metric := range metrics {
		if _, err := s.UpdateMetric(ctx, metric); err != nil {
			return errors.Wrap(err, "restore backup. update metric")
		}
	}

	return nil
}
