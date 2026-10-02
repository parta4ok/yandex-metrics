package entities

import (
	"sort"

	"github.com/pkg/errors"
)

type Metrics struct {
	metrics map[MName]*Metric
}

func NewMetrics() *Metrics {
	return &Metrics{
		metrics: make(map[MName]*Metric),
	}
}

func (m *Metrics) UpdateGauge(name MName, value float64) error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "update gauge. metrics is nil")
	}
	if name.MType() != Gauge {
		return errors.Wrap(ErrInvalidParam, "update gauge. metric type is not gauge")
	}

	metric := m.metric(name)
	metric.value = &value

	return nil
}

func (m *Metrics) IncrementCounter(name MName, delta int64) error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "increment counter. metrics is nil")
	}
	if name.MType() != Counter {
		return errors.Wrap(ErrInvalidParam, "increment counter. metric type is not counter")
	}

	metric := m.metric(name)
	if metric.delta == nil {
		metric.delta = new(int64)
	}
	*metric.delta += delta

	return nil
}

func (m *Metrics) All() ([]*Metric, error) {
	if m == nil {
		return nil, errors.Wrap(ErrInvalidParam, "list metrics. metrics is nil")
	}

	names := make([]MName, 0, len(m.metrics))
	for name := range m.metrics {
		names = append(names, name)
	}
	sort.Slice(names, func(i int, j int) bool {
		return names[i] < names[j]
	})

	metrics := make([]*Metric, 0, len(names))
	for _, name := range names {
		metrics = append(metrics, m.metrics[name].clone())
	}

	return metrics, nil
}

func (m *Metrics) metric(name MName) *Metric {
	if metric, ok := m.metrics[name]; ok {
		return metric
	}

	metric := &Metric{
		name:  name,
		mType: name.MType(),
	}

	m.metrics[name] = metric

	return metric
}
