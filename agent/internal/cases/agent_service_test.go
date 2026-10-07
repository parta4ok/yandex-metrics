package cases_test

import (
	"context"
	"testing"

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

			service, err := cases.NewAgentService(client, provider, storage, noop.New())
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
		testdata.NewMockStorage(ctrl),
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

	t.Run("sends all metrics despite individual errors", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := gaugeMetrics(t, entities.Alloc, 1.5)
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(entities.ErrInternalError)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(nil)
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
			UpdateAgentData(gomock.Any(), gomock.Any()).
			Return(entities.ErrInternalError)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("does not acknowledge gauges", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := gaugeMetrics(t, entities.Alloc, 1.5)
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(nil)

		require.NoError(t, service.SendMetrics(context.Background()))
	})

	t.Run("wraps counter acknowledgement error", func(t *testing.T) {
		t.Parallel()

		service, client, _, storage := newService(t)
		metrics := entities.NewMetrics()
		require.NoError(t, metrics.IncrementCounter(entities.PollCount, 2))
		storage.EXPECT().GetAgentData(gomock.Any()).Return(metrics, nil)
		client.EXPECT().UpdateAgentData(gomock.Any(), gomock.Any()).Return(nil)
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

	ctrl := gomock.NewController(t)
	client := testdata.NewMockMetricServiceClient(ctrl)
	provider := testdata.NewMockDataProvider(ctrl)
	storage := testdata.NewMockStorage(ctrl)
	service, err := cases.NewAgentService(client, provider, storage, noop.New())
	require.NoError(t, err)

	return service, client, provider, storage
}

func gaugeMetrics(t *testing.T, name entities.MName, value float64) *entities.Metrics {
	t.Helper()

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(name, value))

	return metrics
}
