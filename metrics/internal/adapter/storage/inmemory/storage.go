package inmemory

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

var (
	_ cases.MetricsStorage = (*Storage)(nil)
)

type metricKey struct {
	id    string
	mType entities.MType
}

type Storage struct {
	mu      sync.RWMutex
	metrics map[metricKey]*entities.Metrics
}

func NewStorage() *Storage {
	return &Storage{
		metrics: make(map[metricKey]*entities.Metrics),
	}
}

func (s *Storage) UpdateMetric(ctx context.Context, metric *entities.Metrics) error {
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

func (s *Storage) processCounter(metric *entities.Metrics) error {
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

func cloneMetric(metric *entities.Metrics) (*entities.Metrics, error) {
	metricCopy, err := entities.NewMetrics(metric.ID(), metric.MType())
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
