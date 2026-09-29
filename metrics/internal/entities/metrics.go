package entities

import "github.com/pkg/errors"

type Metrics struct {
	id    string
	mType MType
	delta *int64
	value *float64
	hash  *string
}

func NewMetrics(id string, mType MType) (*Metrics, error) {
	if id == "" {
		return nil, errors.Wrap(ErrInvalidParam, "new metrics. id is empty")
	}

	if !mType.IsValid() {
		return nil, errors.Wrap(ErrInvalidParam, "new metrics. metric type is invalid")
	}

	return &Metrics{
		id:    id,
		mType: mType,
	}, nil
}

func (m *Metrics) ID() string {
	return m.id
}

func (m *Metrics) MType() MType {
	return m.mType
}

func (m *Metrics) Delta() *int64 {
	return m.delta
}

func (m *Metrics) SetDelta(delta *int64) {
	m.delta = delta
}

func (m *Metrics) Value() *float64 {
	return m.value
}

func (m *Metrics) SetValue(value *float64) {
	m.value = value
}

func (m *Metrics) Hash() *string {
	return m.hash
}

func (m *Metrics) SetHash(hash *string) {
	m.hash = hash
}

func (m *Metrics) Validate() error {
	if m == nil {
		return errors.Wrap(ErrInvalidParam, "validate metrics. metrics is nil")
	}
	if m.id == "" {
		return errors.Wrap(ErrInvalidParam, "validate metrics. id is empty")
	}
	if !m.mType.IsValid() {
		return errors.Wrap(ErrInvalidParam, "validate metrics. metric type is invalid")
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
