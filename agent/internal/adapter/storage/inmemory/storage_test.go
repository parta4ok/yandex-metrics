package inmemory_test

import (
	"context"
	"testing"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage(noop.New())
	_, err := storage.GetAgentData(context.Background())
	require.ErrorIs(t, err, entities.ErrNotFound)

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 1.5))
	require.NoError(t, storage.UpdateGauges(context.Background(), metrics))
	require.NoError(t, storage.IncrementCounter(context.Background(), entities.PollCount, 2))

	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 9.5))
	require.NoError(t, storage.UpdateGauges(context.Background(), metrics))
	actual, err := storage.GetAgentData(context.Background())
	require.NoError(t, err)
	metricList, err := actual.All()
	require.NoError(t, err)
	require.Equal(t, 9.5, *metricList[0].Value())

	require.NoError(t, metricList[0].SetValue(7.5))
	actual, err = storage.GetAgentData(context.Background())
	require.NoError(t, err)
	metricList, err = actual.All()
	require.NoError(t, err)
	require.Equal(t, 9.5, *metricList[0].Value())
}

func TestStorage_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := inmemory.NewStorage(noop.New())

	_, err := storage.GetAgentData(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, storage.UpdateGauges(ctx, entities.NewMetrics()), context.Canceled)
	require.ErrorIs(t, storage.IncrementCounter(ctx, entities.PollCount, 1), context.Canceled)
	require.ErrorIs(t, storage.AcknowledgeCounter(ctx, entities.PollCount, 1), context.Canceled)
}

func TestStorage_AcknowledgeCounter(t *testing.T) {
	t.Parallel()

	storage := inmemory.NewStorage(noop.New())
	metrics := entities.NewMetrics()
	require.NoError(t, storage.UpdateGauges(context.Background(), metrics))
	require.NoError(t, storage.IncrementCounter(context.Background(), entities.PollCount, 5))

	require.NoError(t, storage.AcknowledgeCounter(context.Background(), entities.PollCount, 3))
	actual, err := storage.GetAgentData(context.Background())
	require.NoError(t, err)
	metricList, err := actual.All()
	require.NoError(t, err)
	require.Equal(t, int64(2), *metricList[0].Delta())

	require.ErrorIs(
		t,
		storage.AcknowledgeCounter(context.Background(), entities.Alloc, 1),
		entities.ErrInvalidParam,
	)
	require.ErrorIs(
		t,
		storage.AcknowledgeCounter(context.Background(), entities.PollCount, -1),
		entities.ErrInvalidParam,
	)
	require.ErrorIs(
		t,
		storage.AcknowledgeCounter(context.Background(), entities.PollCount, 3),
		entities.ErrInternalError,
	)
	require.ErrorIs(
		t,
		inmemory.NewStorage(noop.New()).AcknowledgeCounter(context.Background(), entities.PollCount, 1),
		entities.ErrNotFound,
	)
}
