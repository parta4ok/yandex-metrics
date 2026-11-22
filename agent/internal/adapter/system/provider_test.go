package system_test

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/system"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

func TestProvider_GetSystemData(t *testing.T) {
	t.Parallel()

	cpuCount, err := cpu.Counts(true)
	require.NoError(t, err)
	require.Positive(t, cpuCount)

	metrics, err := system.NewProvider().GetSystemData(context.Background())
	require.NoError(t, err)

	all, err := metrics.All()
	require.NoError(t, err)
	require.Len(t, all, 2+cpuCount)

	byName := make(map[entities.MName]float64, len(all))
	for _, metric := range all {
		require.Equal(t, entities.Gauge, metric.MType())
		require.NotNil(t, metric.Value())

		byName[metric.Name()] = *metric.Value()
	}

	require.Positive(t, byName[entities.TotalMemory])
	require.GreaterOrEqual(t, byName[entities.FreeMemory], 0.0)
	require.LessOrEqual(t, byName[entities.FreeMemory], byName[entities.TotalMemory])

	for i := 1; i <= cpuCount; i++ {
		name, err := entities.NewCPUUtilization(i)
		require.NoError(t, err)
		require.Contains(t, byName, name)
		require.GreaterOrEqual(t, byName[name], 0.0)
		require.LessOrEqual(t, byName[name], 100.0)
	}
}

func TestProvider_GetSystemData_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := system.NewProvider().GetSystemData(ctx)

	require.ErrorIs(t, err, entities.ErrInternalError)
	require.Nil(t, metrics)
}
