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

func TestStorage_GetMetricAndListMetrics(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage()
	require.NoError(t, storage.UpdateMetric(context.Background(), newCounter(t, "requests", 2)))
	require.NoError(t, storage.UpdateMetric(context.Background(), newCounter(t, "requests", 3)))
	require.NoError(t, storage.UpdateMetric(context.Background(), newGauge(t, "memory", 1.5)))

	metric, err := storage.GetMetric(context.Background(), "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(5), *metric.Delta())

	metric.SetDelta(new(int64))
	storedMetric, err := storage.GetMetric(context.Background(), "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(5), *storedMetric.Delta())

	_, err = storage.GetMetric(context.Background(), "unknown", entities.Gauge)
	require.ErrorIs(t, err, entities.ErrNotFound)

	metrics, err := storage.ListMetrics(context.Background())
	require.NoError(t, err)
	require.Len(t, metrics, 2)
	require.Equal(t, "memory", metrics[0].ID())
	require.Equal(t, "requests", metrics[1].ID())
}

func TestStorage_ReadCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := inmemory.NewStorage()

	_, err := storage.GetMetric(ctx, "requests", entities.Counter)
	require.ErrorIs(t, err, context.Canceled)
	_, err = storage.ListMetrics(ctx)
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
