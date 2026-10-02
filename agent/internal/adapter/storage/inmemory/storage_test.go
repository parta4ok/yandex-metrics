package inmemory_test

import (
	"context"
	"testing"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage()
	_, err := storage.GetAgentData(context.Background())
	require.ErrorIs(t, err, entities.ErrNotFound)

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 1.5))
	require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
	require.NoError(t, storage.SaveAgentData(context.Background(), metrics))

	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 9.5))
	actual, err := storage.GetAgentData(context.Background())
	require.NoError(t, err)
	metricList, err := actual.All()
	require.NoError(t, err)
	require.Equal(t, 1.5, *metricList[0].Value())

	require.NoError(t, metricList[0].SetValue(7.5))
	actual, err = storage.GetAgentData(context.Background())
	require.NoError(t, err)
	metricList, err = actual.All()
	require.NoError(t, err)
	require.Equal(t, 1.5, *metricList[0].Value())
}

func TestStorage_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := inmemory.NewStorage()

	_, err := storage.GetAgentData(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, storage.SaveAgentData(ctx, entities.NewMetrics()), context.Canceled)
}
