package cases

import (
	"context"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/internal/entities"
)

type MetricsService struct {
	storage MetricsStorage
}

func NewMetricsService(storage MetricsStorage) (*MetricsService, error) {
	if storage == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. storage is nil")
	}

	return &MetricsService{
		storage: storage,
	}, nil
}

func (s *MetricsService) UpdateMetric(ctx context.Context, metric *entities.Metrics) error {
	if err := metric.Validate(); err != nil {
		return errors.Wrap(err, "update metric. validate metric")
	}

	if err := s.storage.UpdateMetric(ctx, metric); err != nil {
		return errors.Wrap(err, "update metric. update storage")
	}

	return nil
}
