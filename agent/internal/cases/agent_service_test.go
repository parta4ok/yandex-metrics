package cases_test

import (
	"context"
	"testing"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/cases/testdata"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewAgentService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		client       bool
		dataProvider bool
		storage      bool
		wantErr      error
	}{
		{name: "client is nil", dataProvider: true, storage: true, wantErr: entities.ErrInvalidParam},
		{name: "provider is nil", client: true, storage: true, wantErr: entities.ErrInvalidParam},
		{name: "storage is nil", client: true, dataProvider: true, wantErr: entities.ErrInvalidParam},
		{name: "success", client: true, dataProvider: true, storage: true},
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
			var storage cases.Storage
			if tt.storage {
				storage = testdata.NewMockStorage(ctrl)
			}

			service, err := cases.NewAgentService(client, provider, storage)
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

func TestAgentService_UpdateMetrics(t *testing.T) {
	t.Parallel()

	t.Run("creates the first snapshot", func(t *testing.T) {
		t.Parallel()

		service, client, provider, storage := newService(t)
		actual := gaugeMetrics(t, entities.Alloc, 2.5)

		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(actual, nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrNotFound)
		storage.EXPECT().SaveAgentData(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, metrics *entities.Metrics) error {
				assertMetricValues(t, metrics, 2.5, 1)
				return nil
			},
		)

		require.NoError(t, service.UpdateMetrics(context.Background()))
		require.NotNil(t, client)
	})

	t.Run("updates an existing snapshot", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		actual := gaugeMetrics(t, entities.Alloc, 2.5)
		stored := gaugeMetrics(t, entities.Alloc, 1.5)
		require.NoError(t, stored.IncrementCounter(entities.PollCount, 4))

		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(actual, nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(stored, nil)
		storage.EXPECT().SaveAgentData(gomock.Any(), stored).DoAndReturn(
			func(_ context.Context, metrics *entities.Metrics) error {
				assertMetricValues(t, metrics, 2.5, 5)
				return nil
			},
		)

		require.NoError(t, service.UpdateMetrics(context.Background()))
	})

	t.Run("wraps provider error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, _ := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("wraps storage read error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(gaugeMetrics(t, entities.Alloc, 1), nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})

	t.Run("rejects counter from provider", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		actual := entities.NewMetrics()
		require.NoError(t, actual.IncrementCounter(entities.PollCount, 1))
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(actual, nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrNotFound)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("wraps nil provider result", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(nil, nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrNotFound)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("rejects nil stored snapshot with gauges", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(gaugeMetrics(t, entities.Alloc, 1), nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, nil)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("rejects nil stored snapshot without gauges", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(entities.NewMetrics(), nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, nil)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInvalidParam)
	})

	t.Run("wraps save error", func(t *testing.T) {
		t.Parallel()

		service, _, provider, storage := newService(t)
		provider.EXPECT().GetActualAgentData(gomock.Any()).Return(gaugeMetrics(t, entities.Alloc, 1), nil)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrNotFound)
		storage.EXPECT().SaveAgentData(gomock.Any(), gomock.Any()).Return(entities.ErrInternalError)

		require.ErrorIs(t, service.UpdateMetrics(context.Background()), entities.ErrInternalError)
	})
}

func TestAgentService_SendMetrics(t *testing.T) {
	t.Parallel()

	t.Run("sends all metrics despite individual errors", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := gaugeMetrics(t, entities.Alloc, 1.5)
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(entities.ErrInternalError)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("wraps storage error", func(t *testing.T) {
		t.Parallel()

		service, _, _, storage := newService(t)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(nil, entities.ErrInternalError)

		require.ErrorIs(t, service.SendMetrics(context.Background()), entities.ErrInternalError)
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

	ctrl := gomock.NewController(t)
	client := testdata.NewMockMetricServiceClient(ctrl)
	provider := testdata.NewMockDataProvider(ctrl)
	storage := testdata.NewMockStorage(ctrl)
	service, err := cases.NewAgentService(client, provider, storage)
	require.NoError(t, err)

	return service, client, provider, storage
}

func gaugeMetrics(t *testing.T, name entities.MName, value float64) *entities.Metrics {
	t.Helper()

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(name, value))

	return metrics
}

func assertMetricValues(t *testing.T, metrics *entities.Metrics, value float64, delta int64) {
	t.Helper()

	metricList, err := metrics.All()
	require.NoError(t, err)
	require.Len(t, metricList, 2)
	require.Equal(t, entities.Alloc, metricList[0].Name())
	require.Equal(t, value, *metricList[0].Value())
	require.Equal(t, entities.PollCount, metricList[1].Name())
	require.Equal(t, delta, *metricList[1].Delta())
}
