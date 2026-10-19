package inmemory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
)

func TestStorage_UpdateMetric(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage(noop.New())
	counter := newCounter(t, "requests", 2)
	mustUpdate(t, storage, counter)
	stored := mustUpdate(t, storage, newCounter(t, "requests", 3))

	require.Equal(t, int64(2), *counter.Delta())
	require.Equal(t, int64(5), *stored.Delta())

	gauge := newGauge(t, "temperature", 1.5)
	mustUpdate(t, storage, gauge)
	mustUpdate(t, storage, newGauge(t, "temperature", 3.5))
}

func TestStorage_UpdateMetric_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := inmemory.NewStorage(noop.New()).UpdateMetric(ctx, newCounter(t, "requests", 1))
	require.ErrorIs(t, err, context.Canceled)
}

func TestStorage_GetMetricAndListMetrics(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage(noop.New())
	mustUpdate(t, storage, newCounter(t, "requests", 2))
	mustUpdate(t, storage, newCounter(t, "requests", 3))
	mustUpdate(t, storage, newGauge(t, "memory", 1.5))

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

	metrics[0].SetValue(new(float64))
	storedGauge, err := storage.GetMetric(context.Background(), "memory", entities.Gauge)
	require.NoError(t, err)
	require.Equal(t, 1.5, *storedGauge.Value())
}

func TestStorage_ReadCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := inmemory.NewStorage(noop.New())

	_, err := storage.GetMetric(ctx, "requests", entities.Counter)
	require.ErrorIs(t, err, context.Canceled)
	_, err = storage.ListMetrics(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestStorage_ReadEmptyAndSameNameDifferentTypes(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage(noop.New())
	metrics, err := storage.ListMetrics(context.Background())
	require.NoError(t, err)
	require.Empty(t, metrics)

	mustUpdate(t, storage, newCounter(t, "requests", 2))
	mustUpdate(t, storage, newGauge(t, "requests", 1.5))

	counter, err := storage.GetMetric(context.Background(), "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(2), *counter.Delta())

	gauge, err := storage.GetMetric(context.Background(), "requests", entities.Gauge)
	require.NoError(t, err)
	require.Equal(t, 1.5, *gauge.Value())
}

func TestStorage_AccumulatesCounterConcurrently(t *testing.T) {
	t.Parallel()

	const updates = 100

	storage := inmemory.NewStorage(noop.New())
	updatesToStore := make([]*entities.Metric, updates)
	for index := range updatesToStore {
		updatesToStore[index] = newCounter(t, "requests", 1)
	}

	errs := make(chan error, updates)
	var group sync.WaitGroup
	for _, update := range updatesToStore {
		update := update
		group.Go(func() {
			_, err := storage.UpdateMetric(context.Background(), update)
			errs <- err
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	metric, err := storage.GetMetric(context.Background(), "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(updates), *metric.Delta())
}

func newCounter(t *testing.T, id string, delta int64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Counter)
	require.NoError(t, err)
	metric.SetDelta(&delta)
	return metric
}

func newGauge(t *testing.T, id string, value float64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Gauge)
	require.NoError(t, err)
	metric.SetValue(&value)
	return metric
}

func mustUpdate(t *testing.T, storage *inmemory.Storage, metric *entities.Metric) *entities.Metric {
	t.Helper()

	stored, err := storage.UpdateMetric(context.Background(), metric)
	require.NoError(t, err)

	return stored
}
