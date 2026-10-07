package entities

import "github.com/pkg/errors"

type Metric struct {
	id    string
	mType MType
	delta *int64
	value *float64
	hash  *string
}

func NewMetric(id string, mType MType) (*Metric, error) {
	if id == "" {
		return nil, errors.Wrap(ErrInvalidParam, "new metrics. id is empty")
	}

	if !mType.IsValid() {
		return nil, errors.Wrap(ErrInvalidParam, "new metrics. metric type is invalid")
	}

	return &Metric{
		id:    id,
		mType: mType,
	}, nil
}

func (m *Metric) ID() string {
	return m.id
}

func (m *Metric) MType() MType {
	return m.mType
}

func (m *Metric) Delta() *int64 {
	return m.delta
}

func (m *Metric) SetDelta(delta *int64) {
	m.delta = delta
}

func (m *Metric) Value() *float64 {
	return m.value
}

func (m *Metric) SetValue(value *float64) {
	m.value = value
}

func (m *Metric) Hash() *string {
	return m.hash
}

func (m *Metric) SetHash(hash *string) {
	m.hash = hash
}

func (m *Metric) Validate() error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "validate metrics. metrics is nil")
	}
	if m.id == "" || !m.mType.IsValid() {
		return errors.Wrap(ErrInvalidParam, "validate metrics. required fields are invalid")
	}

	switch m.mType {
	case Counter:
		if m.delta == nil {
			return errors.Wrap(ErrInvalidParam, "validate metrics. counter delta is nil")
		}
	case Gauge:
		if m.value == nil {
			return errors.Wrap(ErrInvalidParam, "validate metrics. gauge value is nil")
		}
	}

	return nil
}
