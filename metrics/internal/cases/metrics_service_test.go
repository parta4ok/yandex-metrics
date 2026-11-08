package cases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/cases/testdata"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewMetricsService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		newStorage func(*gomock.Controller) cases.MetricsStorage
		wantErr    error
	}{
		{
			name: "nil storage",
			newStorage: func(*gomock.Controller) cases.MetricsStorage {
				return nil
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name: "storage",
			newStorage: func(ctrl *gomock.Controller) cases.MetricsStorage {
				return testdata.NewMockMetricsStorage(ctrl)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := tt.newStorage(ctrl)

			service, err := cases.NewMetricsService(storage, testdata.NewMockBackupProvider(ctrl), testdata.NewMockPinger(ctrl), noop.New())

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

func TestNewMetricsService_NilLogger(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	service, err := cases.NewMetricsService(
		testdata.NewMockMetricsStorage(ctrl),
		testdata.NewMockBackupProvider(ctrl),
		testdata.NewMockPinger(ctrl),
		nil,
	)

	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, service)
}

func TestMetricsService_UpdateMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		metric       *entities.Metric
		setupStorage func(*testdata.MockMetricsStorage)
		wantErr      error
	}{
		{
			name:    "invalid metric",
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "storage error",
			metric:  newCounterMetric(t, "requests", 1),
			wantErr: testdata.ErrTest,
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().
					UpdateMetric(gomock.Any(), gomock.Any()).
					Return(nil, testdata.ErrTest)
			},
		},
		{
			name:   "success",
			metric: newCounterMetric(t, "requests", 1),
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().
					UpdateMetric(gomock.Any(), gomock.Any()).
					Return(newCounterMetric(t, "requests", 1), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := testdata.NewMockMetricsStorage(ctrl)
			service, err := cases.NewMetricsService(storage, testdata.NewMockBackupProvider(ctrl), testdata.NewMockPinger(ctrl), noop.New())
			require.NoError(t, err)

			if tt.setupStorage != nil {
				tt.setupStorage(storage)
			}

			_, err = service.UpdateMetric(context.Background(), tt.metric)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestMetricsService_GetMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		id           string
		mType        entities.MType
		setupStorage func(*testdata.MockMetricsStorage)
		wantErr      error
	}{
		{
			name:    "empty ID",
			mType:   entities.Counter,
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "invalid metric type",
			id:      "requests",
			mType:   "unknown",
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:  "not found",
			id:    "requests",
			mType: entities.Counter,
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().
					GetMetric(gomock.Any(), "requests", entities.Counter).
					Return(nil, entities.ErrNotFound)
			},
			wantErr: entities.ErrNotFound,
		},
		{
			name:  "success",
			id:    "requests",
			mType: entities.Counter,
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().
					GetMetric(gomock.Any(), "requests", entities.Counter).
					Return(newCounterMetric(t, "requests", 3), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := testdata.NewMockMetricsStorage(ctrl)
			service, err := cases.NewMetricsService(storage, testdata.NewMockBackupProvider(ctrl), testdata.NewMockPinger(ctrl), noop.New())
			require.NoError(t, err)

			if tt.setupStorage != nil {
				tt.setupStorage(storage)
			}

			metric, err := service.GetMetric(context.Background(), tt.id, tt.mType)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, metric)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "requests", metric.ID())
			require.Equal(t, entities.Counter, metric.MType())
			require.Equal(t, int64(3), *metric.Delta())
		})
	}
}

func TestMetricsService_ListMetrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupStorage func(*testdata.MockMetricsStorage)
		wantErr      error
	}{
		{
			name: "storage error",
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().ListMetrics(gomock.Any()).Return(nil, testdata.ErrTest)
			},
			wantErr: testdata.ErrTest,
		},
		{
			name: "success",
			setupStorage: func(storage *testdata.MockMetricsStorage) {
				storage.EXPECT().ListMetrics(gomock.Any()).Return(
					[]*entities.Metric{newCounterMetric(t, "requests", 3)},
					nil,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := testdata.NewMockMetricsStorage(ctrl)
			service, err := cases.NewMetricsService(storage, testdata.NewMockBackupProvider(ctrl), testdata.NewMockPinger(ctrl), noop.New())
			require.NoError(t, err)

			tt.setupStorage(storage)

			metrics, err := service.ListMetrics(context.Background())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, metrics)
				return
			}

			require.NoError(t, err)
			require.Len(t, metrics, 1)
			require.Equal(t, "requests", metrics[0].ID())
		})
	}
}

func newCounterMetric(t *testing.T, id string, delta int64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Counter)
	require.NoError(t, err)

	metric.SetDelta(&delta)

	return metric
}

func TestMetricsService_MakeBackup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	storage := testdata.NewMockMetricsStorage(ctrl)
	backup := testdata.NewMockBackupProvider(ctrl)
	service, err := cases.NewMetricsService(storage, backup, testdata.NewMockPinger(ctrl), noop.New())
	require.NoError(t, err)

	metrics := []*entities.Metric{newCounterMetric(t, "requests", 5)}
	storage.EXPECT().ListMetrics(gomock.Any()).Return(metrics, nil)
	backup.EXPECT().MakeBackup(gomock.Any(), metrics).Return(nil)

	require.NoError(t, service.MakeBackup(context.Background()))
}

func TestMetricsService_MakeBackup_StorageError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	storage := testdata.NewMockMetricsStorage(ctrl)
	service, err := cases.NewMetricsService(storage, testdata.NewMockBackupProvider(ctrl), testdata.NewMockPinger(ctrl), noop.New())
	require.NoError(t, err)

	storage.EXPECT().ListMetrics(gomock.Any()).Return(nil, testdata.ErrTest)

	require.ErrorIs(t, service.MakeBackup(context.Background()), testdata.ErrTest)
}

func TestMetricsService_RestoreBackup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	storage := testdata.NewMockMetricsStorage(ctrl)
	backup := testdata.NewMockBackupProvider(ctrl)
	service, err := cases.NewMetricsService(storage, backup, testdata.NewMockPinger(ctrl), noop.New())
	require.NoError(t, err)

	metric := newCounterMetric(t, "requests", 5)
	backup.EXPECT().RestoreBackup(gomock.Any()).Return([]*entities.Metric{metric}, nil)
	storage.EXPECT().UpdateMetric(gomock.Any(), metric).Return(metric, nil)

	require.NoError(t, service.RestoreBackup(context.Background()))
}

func TestMetricsService_RestoreBackup_InvalidMetric(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	backup := testdata.NewMockBackupProvider(ctrl)
	service, err := cases.NewMetricsService(testdata.NewMockMetricsStorage(ctrl), backup, testdata.NewMockPinger(ctrl), noop.New())
	require.NoError(t, err)

	invalid, err := entities.NewMetric("requests", entities.Counter)
	require.NoError(t, err)
	backup.EXPECT().RestoreBackup(gomock.Any()).Return([]*entities.Metric{invalid}, nil)

	require.ErrorIs(t, service.RestoreBackup(context.Background()), entities.ErrInvalidParam)
}

func TestNewMetricsService_NilPinger(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	service, err := cases.NewMetricsService(
		testdata.NewMockMetricsStorage(ctrl),
		testdata.NewMockBackupProvider(ctrl),
		nil,
		noop.New(),
	)

	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, service)
}

func TestMetricsService_Ping(t *testing.T) {
	t.Parallel()

	errPing := errors.New("ping failed")

	tests := []struct {
		name    string
		pingErr error
	}{
		{name: "success"},
		{name: "pinger error", pingErr: errPing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pinger := testdata.NewMockPinger(ctrl)
			pinger.EXPECT().Ping(gomock.Any()).Return(tt.pingErr)

			service, err := cases.NewMetricsService(
				testdata.NewMockMetricsStorage(ctrl),
				testdata.NewMockBackupProvider(ctrl),
				pinger,
				noop.New(),
			)
			require.NoError(t, err)

			err = service.Ping(context.Background())
			if tt.pingErr != nil {
				require.ErrorIs(t, err, tt.pingErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func newGaugeMetric(t *testing.T, id string, value float64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Gauge)
	require.NoError(t, err)

	metric.SetValue(&value)

	return metric
}

func newBatchService(t *testing.T) (*cases.MetricsService, *testdata.MockMetricsStorage) {
	t.Helper()

	ctrl := gomock.NewController(t)
	storage := testdata.NewMockMetricsStorage(ctrl)

	service, err := cases.NewMetricsService(
		storage,
		testdata.NewMockBackupProvider(ctrl),
		testdata.NewMockPinger(ctrl),
		noop.New(),
	)
	require.NoError(t, err)

	return service, storage
}

func TestMetricsService_UpdateMetrics_MergesAndSorts(t *testing.T) {
	t.Parallel()

	service, storage := newBatchService(t)

	var passed []*entities.Metric
	storage.EXPECT().
		UpdateMetrics(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, metrics []*entities.Metric) ([]*entities.Metric, error) {
			passed = metrics

			return metrics, nil
		})

	stored, err := service.UpdateMetrics(context.Background(), []*entities.Metric{
		newGaugeMetric(t, "temperature", 1.5),
		newCounterMetric(t, "requests", 2),
		newGaugeMetric(t, "temperature", 3.5),
		newCounterMetric(t, "requests", 3),
		newGaugeMetric(t, "load", 0.5),
	})

	require.NoError(t, err)
	require.Equal(t, passed, stored)
	require.Len(t, passed, 3)
	require.Equal(t, "load", passed[0].ID())
	require.Equal(t, "requests", passed[1].ID())
	require.Equal(t, int64(5), *passed[1].Delta())
	require.Equal(t, "temperature", passed[2].ID())
	require.Equal(t, 3.5, *passed[2].Value())
}

func TestMetricsService_UpdateMetrics_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	service, storage := newBatchService(t)
	storage.EXPECT().UpdateMetrics(gomock.Any(), gomock.Any()).Return(nil, nil)

	first := newCounterMetric(t, "requests", 2)
	_, err := service.UpdateMetrics(context.Background(), []*entities.Metric{
		first,
		newCounterMetric(t, "requests", 3),
	})

	require.NoError(t, err)
	require.Equal(t, int64(2), *first.Delta())
}

func TestMetricsService_UpdateMetrics_InvalidBatch(t *testing.T) {
	t.Parallel()

	invalid, err := entities.NewMetric("requests", entities.Counter)
	require.NoError(t, err)

	tests := []struct {
		name    string
		metrics []*entities.Metric
	}{
		{name: "nil batch"},
		{name: "empty batch", metrics: []*entities.Metric{}},
		{name: "nil metric", metrics: []*entities.Metric{nil}},
		{name: "counter without delta", metrics: []*entities.Metric{invalid}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, _ := newBatchService(t)

			stored, err := service.UpdateMetrics(context.Background(), tt.metrics)

			require.ErrorIs(t, err, entities.ErrInvalidParam)
			require.Nil(t, stored)
		})
	}
}

func TestMetricsService_UpdateMetrics_StorageError(t *testing.T) {
	t.Parallel()

	service, storage := newBatchService(t)
	storage.EXPECT().UpdateMetrics(gomock.Any(), gomock.Any()).Return(nil, entities.ErrInternalError)

	stored, err := service.UpdateMetrics(context.Background(), []*entities.Metric{
		newCounterMetric(t, "requests", 1),
	})

	require.ErrorIs(t, err, entities.ErrInternalError)
	require.Nil(t, stored)
}
