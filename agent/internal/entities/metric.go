package entities

import "github.com/pkg/errors"

type Metric struct {
	name  MName
	mType MType
	delta *int64
	value *float64
}

func NewMetric(name MName) (*Metric, error) {
	if !name.IsValid() {
		return nil, errors.Wrap(ErrInvalidParam, "new metric. metric name is invalid")
	}

	return &Metric{
		name:  name,
		mType: name.MType(),
	}, nil
}

func (m *Metric) Name() MName {
	return m.name
}

func (m *Metric) MType() MType {
	return m.mType
}

func (m *Metric) Delta() *int64 {
	return m.delta
}

func (m *Metric) Value() *float64 {
	return m.value
}

func (m *Metric) SetDelta(delta int64) error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "set metric delta. metric is nil")
	}
	if m.mType != Counter {
		return errors.Wrap(ErrInvalidParam, "set metric delta. metric type is not counter")
	}

	m.delta = &delta

	return nil
}

func (m *Metric) SetValue(value float64) error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "set metric value. metric is nil")
	}
	if m.mType != Gauge {
		return errors.Wrap(ErrInvalidParam, "set metric value. metric type is not gauge")
	}

	m.value = &value

	return nil
}

func (m *Metric) Increment(delta int64) error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "increment metric. metric is nil")
	}
	if m.mType != Counter {
		return errors.Wrap(ErrInvalidParam, "increment metric. metric type is not counter")
	}

	if m.delta == nil {
		m.delta = new(int64)
	}
	*m.delta += delta

	return nil
}

func (m *Metric) Validate() error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "validate metric. metric is nil")
	}
	if !m.name.IsValid() || m.mType != m.name.MType() {
		return errors.Wrap(ErrInvalidParam, "validate metric. metric definition is invalid")
	}

	switch m.mType {
	case Counter:
		if m.delta == nil {
			return errors.Wrap(ErrInvalidParam, "validate metric. counter delta is nil")
		}
	case Gauge:
		if m.value == nil {
			return errors.Wrap(ErrInvalidParam, "validate metric. gauge value is nil")
		}
	}

	return nil
}

func (m *Metric) Clone() (*Metric, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Wrap(err, "clone metric. validate metric")
	}

	return m.clone(), nil
}

func (m *Metric) clone() *Metric {
	metric := &Metric{
		name:  m.name,
		mType: m.mType,
	}
	if m.delta != nil {
		delta := *m.delta
		metric.delta = &delta
	}
	if m.value != nil {
		value := *m.value
		metric.value = &value
	}

	return metric
}
