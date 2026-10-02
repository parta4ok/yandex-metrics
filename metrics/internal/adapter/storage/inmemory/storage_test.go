package inmemory_test

import (
	"context"
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/stretchr/testify/require"
)

func TestStorage_UpdateMetric(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage()
	counter := newCounter(t, "requests", 2)
	require.NoError(t, storage.UpdateMetric(context.Background(), counter))
	require.NoError(t, storage.UpdateMetric(context.Background(), newCounter(t, "requests", 3)))

	require.Equal(t, int64(2), *counter.Delta())

	gauge := newGauge(t, "temperature", 1.5)
	require.NoError(t, storage.UpdateMetric(context.Background(), gauge))
	require.NoError(t, storage.UpdateMetric(context.Background(), newGauge(t, "temperature", 3.5)))
}

func TestStorage_UpdateMetric_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := inmemory.NewStorage().UpdateMetric(ctx, newCounter(t, "requests", 1))
	require.ErrorIs(t, err, context.Canceled)
}

func newCounter(t *testing.T, id string, delta int64) *entities.Metrics {
	t.Helper()

	metric, err := entities.NewMetrics(id, entities.Counter)
	require.NoError(t, err)
	metric.SetDelta(&delta)
	return metric
}

func newGauge(t *testing.T, id string, value float64) *entities.Metrics {
	t.Helper()

	metric, err := entities.NewMetrics(id, entities.Gauge)
	require.NoError(t, err)
	metric.SetValue(&value)
	return metric
}
