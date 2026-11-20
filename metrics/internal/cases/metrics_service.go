package cases

import (
	"context"
	"sort"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type MetricsService struct {
	storage MetricsStorage
	backup  BackupProvider
	pinger  Pinger
	logger  toolkitlogger.Logger
}

func NewMetricsService(
	storage MetricsStorage,
	backup BackupProvider,
	pinger Pinger,
	logger toolkitlogger.Logger,
) (*MetricsService, error) {
	if storage == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. storage is nil")
	}
	if backup == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. backup provider is nil")
	}
	if pinger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. pinger is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics service. logger is nil")
	}

	return &MetricsService{
		storage: storage,
		backup:  backup,
		pinger:  pinger,
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

func (s *MetricsService) UpdateMetrics(
	ctx context.Context,
	metrics []*entities.Metric,
) ([]*entities.Metric, error) {
	if len(metrics) == 0 {
		return nil, errors.Wrap(entities.ErrInvalidParam, "update metrics. batch is empty")
	}

	for _, metric := range metrics {
		if err := metric.Validate(); err != nil {
			return nil, errors.Wrap(err, "update metrics. validate metric")
		}
	}

	merged, err := s.mergeMetrics(metrics)
	if err != nil {
		return nil, errors.Wrap(err, "update metrics. merge metrics")
	}

	stored, err := s.storage.UpdateMetrics(ctx, merged)
	if err != nil {
		return nil, errors.Wrap(err, "update metrics. update storage")
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

func (s *MetricsService) Ping(ctx context.Context) error {
	if err := s.pinger.Ping(ctx); err != nil {
		return errors.Wrap(err, "ping. ping storage")
	}

	return nil
}

func (s *MetricsService) mergeMetrics(metrics []*entities.Metric) ([]*entities.Metric, error) {
	type metricKey struct {
		id    string
		mType entities.MType
	}

	merged := make(map[metricKey]*entities.Metric, len(metrics))
	for _, metric := range metrics {
		key := metricKey{id: metric.ID(), mType: metric.MType()}

		current, ok := merged[key]
		if ok && metric.MType() == entities.Counter {
			delta := *current.Delta() + *metric.Delta()
			current.SetDelta(&delta)
			current.SetHash(metric.Hash())

			continue
		}

		metricCopy, err := entities.NewMetric(metric.ID(), metric.MType())
		if err != nil {
			return nil, errors.Wrap(err, "merge metrics. create metric")
		}
		metricCopy.SetDelta(metric.Delta())
		metricCopy.SetValue(metric.Value())
		metricCopy.SetHash(metric.Hash())

		merged[key] = metricCopy
	}

	result := make([]*entities.Metric, 0, len(merged))
	for _, metric := range merged {
		result = append(result, metric)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].ID() == result[j].ID() {
			return result[i].MType() < result[j].MType()
		}

		return result[i].ID() < result[j].ID()
	})

	return result, nil
}
