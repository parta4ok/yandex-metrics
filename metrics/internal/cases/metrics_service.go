package cases

import (
	"context"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
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

func (s *MetricsService) UpdateMetric(ctx context.Context, metric *entities.Metric) error {
	if err := metric.Validate(); err != nil {
		return errors.Wrap(err, "update metric. validate metric")
	}

	if err := s.storage.UpdateMetric(ctx, metric); err != nil {
		return errors.Wrap(err, "update metric. update storage")
	}

	return nil
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
