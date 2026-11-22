package cases_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/cases/testdata"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewAgentService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		client         bool
		dataProvider   bool
		systemProvider bool
		storage        bool
		rateLimit      int
		wantErr        error
	}{
		{name: "client is nil", dataProvider: true, systemProvider: true, storage: true, rateLimit: 1, wantErr: entities.ErrInvalidParam},
		{name: "provider is nil", client: true, systemProvider: true, storage: true, rateLimit: 1, wantErr: entities.ErrInvalidParam},
		{name: "system provider is nil", client: true, dataProvider: true, storage: true, rateLimit: 1, wantErr: entities.ErrInvalidParam},
		{name: "storage is nil", client: true, dataProvider: true, systemProvider: true, rateLimit: 1, wantErr: entities.ErrInvalidParam},
		{name: "rate limit is zero", client: true, dataProvider: true, systemProvider: true, storage: true, wantErr: entities.ErrInvalidParam},
		{name: "rate limit is negative", client: true, dataProvider: true, systemProvider: true, storage: true, rateLimit: -1, wantErr: entities.ErrInvalidParam},
		{name: "success", client: true, dataProvider: true, systemProvider: true, storage: true, rateLimit: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			var client cases.MetricServiceClient
			if tt.client {
				client = testdata.NewMockMetricServiceClient(ctrl)
			}
			var provider cases.DataProvider
			if tt.dataProvider {
				provider = testdata.NewMockDataProvider(ctrl)
			}
			var systemProvider cases.SystemDataProvider
			if tt.systemProvider {
				systemProvider = testdata.NewMockSystemDataProvider(ctrl)
			}
			var storage cases.Storage
			if tt.storage {
				storage = testdata.NewMockStorage(ctrl)
			}

			service, err := cases.NewAgentService(client, provider, systemProvider, storage, tt.rateLimit, noop.New())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, service)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, service)
		})
	}
}

func TestNewAgentService_NilLogger(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	service, err := cases.NewAgentService(
		testdata.NewMockMetricServiceClient(ctrl),
		testdata.NewMockDataProvider(ctrl),
		testdata.NewMockSystemDataProvider(ctrl),
		testdata.NewMockStorage(ctrl),
		1,
		nil,
	)

	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, service)
}

func TestAgentService_UpdateMetrics(t *testing.T) {
	t.Parallel()

	t.Run("updates gauges and increments poll count", func(t *testing.T) {
		t.Parallel()

		service, client, provider, storage := newService(t)
		actual := gaugeMetrics(t, entities.Alloc, 2.5)

		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(actual, nil)
		storage.EXPECT().UpdateGauges(gomock.Any(), actual).DoAndReturn(
			func(_ context.Context, metrics *entities.Metrics) error {
				metricList, err := metrics.All()
				require.NoError(t, err)
				require.Len(t, metricList, 1)
				require.Equal(t, 2.5, *metricList[0].Value())
				return nil
			},
		)
		storage.EXPECT().IncrementCounter(gomock.Any(), entities.PollCount, int64(1)).Return(nil)

		require.NoError(t, service.UpdateMetrics(context.Background()))
		require.NotNil(t, client)
	})

	t.Run("wraps provider error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, _ := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("wraps gauge update error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(gaugeMetrics(t, entities.Alloc, 1), nil)
		storage.EXPECT().UpdateGauges(gomock.Any(), gomock.Any()).Return(entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("rejects counter from provider", func(t *testing.T) {
		t.Parallel()

		service, _, provider, _ := newService(t)
		actual := entities.NewMetrics()
		require.NoError(t, actual.IncrementCounter(entities.PollCount, 1))
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(actual, nil)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("wraps nil provider result", func(t *testing.T) {
		t.Parallel()

		service, _, provider, _ := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(nil, nil)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("wraps poll count increment error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(gaugeMetrics(t, entities.Alloc, 1), nil)
		storage.EXPECT().UpdateGauges(gomock.Any(), gomock.Any()).Return(nil)
		storage.EXPECT().
			IncrementCounter(gomock.Any(), entities.PollCount, int64(1)).
			Return(entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})
}

func TestAgentService_SendMetrics(t *testing.T) {
	t.Parallel()

	t.Run("sends all metrics in one batch and acknowledges counter", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := gaugeMetrics(t, entities.Alloc, 1.5)
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().
			UpdateAgentDataBatch(gomock.Any(), gomock.Len(2)).
			Return(nil)
		storage.EXPECT().AcknowledgeCounter(gomock.Any(), entities.PollCount, int64(2)).Return(nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("retains counter after a failed send", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := entities.NewMetrics()
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().
			UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
			Return(entities.ErrInternalError)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("does not acknowledge gauges", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := gaugeMetrics(t, entities.Alloc, 1.5)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentDataBatch(gomock.Any(), gomock.Any()).Return(nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("does not send an empty batch", func(t *testing.T) {
		t.Parallel()

		service, _, _, storage := newService(t)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(entities.NewMetrics(), nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("keeps counter when the server answers not found", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := entities.NewMetrics()
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().
			UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
			Return(entities.ErrNotFound)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("wraps counter acknowledgement error", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := entities.NewMetrics()
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentDataBatch(gomock.Any(), gomock.Any()).Return(nil)
		storage.EXPECT().
			AcknowledgeCounter(gomock.Any(), entities.PollCount, int64(2)).
			Return(entities.ErrInternalError)

		require.ErrorIs(t, service.SendMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("wraps storage error", func(t *testing.T) {
		t.Parallel()

		service, _, _, storage := newService(t)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.ErrorIs(t, service.SendMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("skips reporting before the first snapshot", func(t *testing.T) {
		t.Parallel()

		service, _, _, storage := newService(t)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrNotFound)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("wraps nil snapshot", func(t *testing.T) {
		t.Parallel()

		service, _, _, storage := newService(t)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, nil)

		require.ErrorIs(t, service.SendMetrics(context.Background()), entities.ErrInvalidParam)
	})
}

func newService(t *testing.T) (
	*cases.AgentService,
	*testdata.MockMetricServiceClient,
	*testdata.MockDataProvider,
	*testdata.MockStorage,
) {
	t.Helper()

	service, client, provider, _, storage := newServiceWithRateLimit(t, 1)

	return service, client, provider, storage
}

func newServiceWithRateLimit(t *testing.T, rateLimit int) (
	*cases.AgentService,
	*testdata.MockMetricServiceClient,
	*testdata.MockDataProvider,
	*testdata.MockSystemDataProvider,
	*testdata.MockStorage,
) {
	t.Helper()

	ctrl := gomock.NewController(t)
	client := testdata.NewMockMetricServiceClient(ctrl)
	provider := testdata.NewMockDataProvider(ctrl)
	systemProvider := testdata.NewMockSystemDataProvider(ctrl)
	storage := testdata.NewMockStorage(ctrl)
	service, err := cases.NewAgentService(client, provider, systemProvider, storage, rateLimit, noop.New())
	require.NoError(t, err)

	return service, client, provider, systemProvider, storage
}

func gaugeMetrics(t *testing.T, name entities.MName, value float64) *entities.Metrics {
	t.Helper()

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(name, value))

	return metrics
}

func cpuMetrics(t *testing.T, count int) *entities.Metrics {
	t.Helper()

	metrics := entities.NewMetrics()
	for i := 1; i <= count; i++ {
		name, err := entities.NewCPUUtilization(i)
		require.NoError(t, err)
		require.NoError(t, metrics.UpdateGauge(name, float64(i)))
	}

	return metrics
}

func containsMetric(metrics []*entities.Metric, name entities.MName) bool {
	for _, metric := range metrics {
		if metric.Name() == name {
			return true
		}
	}

	return false
}

func TestAgentService_UpdateSystemMetrics(t *testing.T) {
	t.Parallel()

	t.Run("stores system gauges", func(t *testing.T) {
		t.Parallel()

		service, _, _, systemProvider, storage := newServiceWithRateLimit(t, 1)
		metrics := cpuMetrics(t, 2)
		require.NoError(t, metrics.UpdateGauge(entities.TotalMemory, 100))
		require.NoError(t, metrics.UpdateGauge(entities.FreeMemory, 40))
		systemProvider.EXPECT().GetSystemData(gomock.Any()).Return(metrics, nil)
		storage.EXPECT().UpdateGauges(gomock.Any(), metrics).Return(nil)

		require.NoError(t, service.UpdateSystemMetrics(context.Background()))
	})

	t.Run("skips the cycle when the provider fails", func(t *testing.T) {
		t.Parallel()

		service, _, _, systemProvider, _ := newServiceWithRateLimit(t, 1)
		systemProvider.EXPECT().GetSystemData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.NoError(t, service.UpdateSystemMetrics(context.Background()))
	})

	t.Run("rejects counters", func(t *testing.T) {
		t.Parallel()

		service, _, _, systemProvider, _ := newServiceWithRateLimit(t, 1)
		metrics := entities.NewMetrics()
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 1))
		systemProvider.EXPECT().GetSystemData(gomock.Any()).Return(metrics, nil)

		require.ErrorIs(t, service.UpdateSystemMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("wraps storage error", func(t *testing.T) {
		t.Parallel()

		service, _, _, systemProvider, storage := newServiceWithRateLimit(t, 1)
		metrics := cpuMetrics(t, 1)
		systemProvider.EXPECT().GetSystemData(gomock.Any()).Return(metrics, nil)
		storage.EXPECT().UpdateGauges(gomock.Any(), metrics).Return(entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateSystemMetrics(context.Background()), entities.ErrInternalError)
	})
}

func TestAgentService_SendMetrics_SplitsIntoChunks(t *testing.T) {
	t.Parallel()

	service, client, _, _, storage := newServiceWithRateLimit(t, 3)
	metrics := cpuMetrics(t, 25)
	require.NoError(t, metrics.IncrementCounter(entities.PollCount, 4))
	storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)

	var (
		mu    sync.Mutex
		sizes []int
		seen  = make(map[entities.MName]int)
	)
	client.EXPECT().
		UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, chunk []*entities.Metric) error {
			mu.Lock()
			defer mu.Unlock()

			sizes = append(sizes, len(chunk))
			for _, metric := range chunk {
				seen[metric.Name()]++
			}

			return nil
		}).
		Times(3)
	storage.EXPECT().AcknowledgeCounter(gomock.Any(), entities.PollCount, int64(4)).Return(nil)

	require.NoError(t, service.SendMetrics(context.Background()))

	require.ElementsMatch(t, []int{10, 10, 6}, sizes)
	require.Len(t, seen, 26)
	for name, count := range seen {
		require.Equal(t, 1, count, "metric %s", name)
	}
}

func TestAgentService_SendMetrics_RespectsRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rateLimit int
		wantMax   int32
	}{
		{name: "limit one", rateLimit: 1, wantMax: 1},
		{name: "limit two", rateLimit: 2, wantMax: 2},
		{name: "limit above chunk count", rateLimit: 10, wantMax: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, client, _, _, storage := newServiceWithRateLimit(t, tt.rateLimit)
			storage.EXPECT().GetAgentData(gomock.Any()).Return(cpuMetrics(t, 50), nil)

			var current, peak atomic.Int32
			client.EXPECT().
				UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
				DoAndReturn(func(context.Context, []*entities.Metric) error {
					now := current.Add(1)
					defer current.Add(-1)

					for {
						seenPeak := peak.Load()
						if now <= seenPeak || peak.CompareAndSwap(seenPeak, now) {
							break
						}
					}
					time.Sleep(40 * time.Millisecond)

					return nil
				}).
				Times(5)

			require.NoError(t, service.SendMetrics(context.Background()))

			require.Equal(t, tt.wantMax, peak.Load())
		})
	}
}

func TestAgentService_SendMetrics_AcknowledgesCounterPerChunk(t *testing.T) {
	t.Parallel()

	newMetrics := func(t *testing.T) *entities.Metrics {
		t.Helper()

		metrics := cpuMetrics(t, 25)
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 7))

		return metrics
	}

	t.Run("failed chunk with counter keeps it unacknowledged", func(t *testing.T) {
		t.Parallel()

		service, client, _, _, storage := newServiceWithRateLimit(t, 3)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(newMetrics(t), nil)
		client.EXPECT().
			UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, chunk []*entities.Metric) error {
				if containsMetric(chunk, entities.PollCount) {
					return entities.ErrInternalError
				}

				return nil
			}).
			Times(3)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("failed chunk without counter does not block acknowledgement", func(t *testing.T) {
		t.Parallel()

		service, client, _, _, storage := newServiceWithRateLimit(t, 3)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(newMetrics(t), nil)
		client.EXPECT().
			UpdateAgentDataBatch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, chunk []*entities.Metric) error {
				if containsMetric(chunk, entities.PollCount) {
					return nil
				}

				return entities.ErrInternalError
			}).
			Times(3)
		storage.EXPECT().AcknowledgeCounter(gomock.Any(), entities.PollCount, int64(7)).Return(nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})
}
