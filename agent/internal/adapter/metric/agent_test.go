package metric_test

import (
	"context"
	"testing"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/metric"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
)

func TestAgent_GetActualAgentData(t *testing.T) {
	t.Parallel()

	agent := metric.NewAgent(noop.New())
	metrics, err := agent.GetActualAgentData(context.Background())
	require.NoError(t, err)

	actual, err := metrics.All()
	require.NoError(t, err)
	require.Len(t, actual, 28)

	names := make(map[entities.MName]*entities.Metric, len(actual))
	for _, metric := range actual {
		names[metric.Name()] = metric
		require.Equal(t, entities.Gauge, metric.MType())
		require.NotNil(t, metric.Value())
	}

	require.Contains(t, names, entities.Alloc)
	require.Contains(t, names, entities.RandomValue)
	require.NotContains(t, names, entities.PollCount)
}

func TestAgent_GetActualAgentData_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := metric.NewAgent(noop.New()).GetActualAgentData(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, metrics)
}
