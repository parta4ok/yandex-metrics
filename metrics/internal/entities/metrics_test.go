package entities_test

import (
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/stretchr/testify/require"
)

func TestMType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mType entities.MType
		want  bool
	}{
		{name: "counter", mType: entities.Counter, want: true},
		{name: "gauge", mType: entities.Gauge, want: true},
		{name: "invalid", mType: "invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, tt.mType.IsValid())
		})
	}
}

func TestNewMetrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		mType   entities.MType
		wantErr error
	}{
		{name: "counter", id: "requests", mType: entities.Counter},
		{name: "gauge", id: "memory", mType: entities.Gauge},
		{name: "empty ID", mType: entities.Counter, wantErr: entities.ErrInvalidParam},
		{name: "invalid metric type", id: "metric", mType: "invalid", wantErr: entities.ErrInvalidParam},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metric, err := entities.NewMetric(tt.id, tt.mType)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, metric)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.id, metric.ID())
			require.Equal(t, tt.mType, metric.MType())
		})
	}
}

func TestMetrics_OptionalFields(t *testing.T) {
	t.Parallel()

	metric, err := entities.NewMetric("metric", entities.Counter)
	require.NoError(t, err)

	delta := int64(42)
	value := 1.5
	hash := "hash"

	metric.SetDelta(&delta)
	metric.SetValue(&value)
	metric.SetHash(&hash)

	require.Equal(t, &delta, metric.Delta())
	require.Equal(t, &value, metric.Value())
	require.Equal(t, &hash, metric.Hash())
}

func TestMetrics_Validate(t *testing.T) {
	t.Parallel()

	counter, err := entities.NewMetric("requests", entities.Counter)
	require.NoError(t, err)

	gauge, err := entities.NewMetric("memory", entities.Gauge)
	require.NoError(t, err)

	counterDelta := int64(1)
	validCounter, err := entities.NewMetric("requests", entities.Counter)
	require.NoError(t, err)
	validCounter.SetDelta(&counterDelta)

	gaugeValue := 1.5
	validGauge, err := entities.NewMetric("memory", entities.Gauge)
	require.NoError(t, err)
	validGauge.SetValue(&gaugeValue)

	tests := []struct {
		name    string
		metric  *entities.Metric
		wantErr error
	}{
		{name: "nil metric", wantErr: entities.ErrInvalidParam},
		{name: "empty metric", metric: &entities.Metric{}, wantErr: entities.ErrInvalidParam},
		{name: "counter without delta", metric: counter, wantErr: entities.ErrInvalidParam},
		{name: "gauge without value", metric: gauge, wantErr: entities.ErrInvalidParam},
		{name: "valid counter", metric: validCounter},
		{name: "valid gauge", metric: validGauge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.metric.Validate()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}
