package inmemory

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

var _ cases.Storage = (*Storage)(nil)

type Storage struct {
	mu      sync.RWMutex
	metrics map[entities.MName]*entities.Metric
}

func NewStorage() *Storage {
	return &Storage{
		metrics: make(map[entities.MName]*entities.Metric),
	}
}

func (s *Storage) GetAgentData(ctx context.Context) (*entities.Metrics, error) {
	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "get agent data. context")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := validateContext(ctx); err != nil {
		return nil, errors.Wrap(err, "get agent data. context")
	}
	if len(s.metrics) == 0 {
		return nil, errors.Wrap(entities.ErrNotFound, "get agent data. metrics not found")
	}

	metrics, err := newMetrics(s.metrics)
	if err != nil {
		return nil, errors.Wrap(err, "get agent data. create metrics")
	}

	return metrics, nil
}

func (s *Storage) SaveAgentData(ctx context.Context, metrics *entities.Metrics) error {
	if err := validateContext(ctx); err != nil {
		return errors.Wrap(err, "save agent data. context")
	}

	metricMap, err := newMetricMap(metrics)
	if err != nil {
		return errors.Wrap(err, "save agent data. create metric map")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateContext(ctx); err != nil {
		return errors.Wrap(err, "save agent data. context")
	}

	s.metrics = metricMap

	return nil
}

func validateContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context is cancelled")
	}

	return nil
}

func newMetricMap(metrics *entities.Metrics) (map[entities.MName]*entities.Metric, error) {
	metricList, err := metrics.All()
	if err != nil {
		return nil, errors.Wrap(err, "create metric map. list metrics")
	}

	metricMap := make(map[entities.MName]*entities.Metric, len(metricList))
	for _, metric := range metricList {
		metricCopy, err := metric.Clone()
		if err != nil {
			return nil, errors.Wrap(err, "create metric map. clone metric")
		}

		metricMap[metricCopy.Name()] = metricCopy
	}

	return metricMap, nil
}

func newMetrics(metricMap map[entities.MName]*entities.Metric) (*entities.Metrics, error) {
	metrics := entities.NewMetrics()
	for _, metric := range metricMap {
		if err := updateMetric(metrics, metric); err != nil {
			return nil, errors.Wrap(err, "create metrics. update metric")
		}
	}

	return metrics, nil
}

func updateMetric(metrics *entities.Metrics, metric *entities.Metric) error {
	if metric == nil {
		return errors.Wrap(entities.ErrInternalError, "update metric. metric is nil")
	}

	switch metric.MType() {
	case entities.Counter:
		if metric.Delta() == nil {
			return errors.Wrap(entities.ErrInternalError, "update metric. counter delta is nil")
		}

		if err := metrics.IncrementCounter(metric.Name(), *metric.Delta()); err != nil {
			return errors.Wrap(err, "update metric. increment counter")
		}
	case entities.Gauge:
		if metric.Value() == nil {
			return errors.Wrap(entities.ErrInternalError, "update metric. gauge value is nil")
		}

		if err := metrics.UpdateGauge(metric.Name(), *metric.Value()); err != nil {
			return errors.Wrap(err, "update metric. update gauge")
		}
	default:
		return errors.Wrap(entities.ErrInternalError, "update metric. metric type is invalid")
	}

	return nil
}
