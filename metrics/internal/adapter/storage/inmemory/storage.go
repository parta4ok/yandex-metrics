package inmemory

import (
	"context"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

var (
	_ cases.MetricsStorage = (*Storage)(nil)
)

type metricKey struct {
	id    string
	mType entities.MType
}

type Storage struct {
	mu      sync.Mutex
	metrics map[metricKey]*entities.Metric
	logger  toolkitlogger.Logger
}

func NewStorage(logger toolkitlogger.Logger) *Storage {
	return &Storage{
		metrics: make(map[metricKey]*entities.Metric),
		logger:  logger,
	}
}

func (s *Storage) UpdateMetric(ctx context.Context, metric *entities.Metric) error {
	if err := validateContext(ctx); err != nil {
		return errors.Wrap(err, "update metric. context")
	}

	metricCopy, err := cloneMetric(metric)
	if err != nil {
		return errors.Wrap(err, "update metric. clone metric")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateContext(ctx); err != nil {
		return errors.Wrap(err, "update metric. context")
	}

	if metricCopy.MType() == entities.Counter {
		if err := s.processCounter(metricCopy); err != nil {
			return errors.Wrap(err, "update metric. process counter")
		}
	}

	key := metricKey{
		id:    metricCopy.ID(),
		mType: metricCopy.MType(),
	}
	s.metrics[key] = metricCopy

	return nil
}

func (s *Storage) GetMetric(
	ctx context.Context,
	id string,
	mType entities.MType,
) (*entities.Metric, error) {
	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "get metric. context")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "get metric. context")
	}

	metric, ok := s.metrics[metricKey{id: id, mType: mType}]
	if !ok {
		return nil, errors.Wrap(entities.ErrNotFound, "get metric. metric not found")
	}

	metricCopy, err := cloneMetric(metric)
	if err != nil {
		return nil, errors.Wrap(err, "get metric. clone metric")
	}

	return metricCopy, nil
}

func (s *Storage) ListMetrics(ctx context.Context) ([]*entities.Metric, error) {
	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "list metrics. context")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "list metrics. context")
	}

	keys := make([]metricKey, 0, len(s.metrics))
	for key := range s.metrics {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		if keys[i].id == keys[j].id {
			return keys[i].mType < keys[j].mType
		}

		return keys[i].id < keys[j].id
	})

	metrics := make([]*entities.Metric, 0, len(keys))
	for _, key := range keys {
		metricCopy, err := cloneMetric(s.metrics[key])
		if err != nil {
			return nil, errors.Wrap(err, "list metrics. clone metric")
		}

		metrics = append(metrics, metricCopy)
	}

	return metrics, nil
}

func (s *Storage) processCounter(metric *entities.Metric) error {
	key := metricKey{
		id:    metric.ID(),
		mType: metric.MType(),
	}
	storedMetric, ok := s.metrics[key]
	if !ok {
		return nil
	}
	if storedMetric == nil || storedMetric.Delta() == nil {
		return errors.Wrap(entities.ErrInternalError, "process counter. stored counter is invalid")
	}

	delta := *storedMetric.Delta() + *metric.Delta()
	metric.SetDelta(&delta)

	return nil
}

func validateContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context is cancelled")
	}

	return nil
}

func cloneMetric(metric *entities.Metric) (*entities.Metric, error) {
	metricCopy, err := entities.NewMetric(metric.ID(), metric.MType())
	if err != nil {
		return nil, errors.Wrap(err, "clone metric. create metric")
	}

	metricCopy.SetDelta(cloneValue(metric.Delta()))
	metricCopy.SetValue(cloneValue(metric.Value()))
	metricCopy.SetHash(cloneValue(metric.Hash()))

	return metricCopy, nil
}

func cloneValue[T any](value *T) *T {
	if value == nil {
		return nil
	}

	valueCopy := *value
	return &valueCopy
}
