package cases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/testdata"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
)

func TestSyncBackupDecorator_UpdateMetric_MakesBackup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	inner := testdata.NewMockMetricServiceProvider(ctrl)
	service, err := cases.NewSyncBackupDecorator(inner, noop.New())
	require.NoError(t, err)

	metric := newCounterMetric(t, "requests", 1)
	inner.EXPECT().UpdateMetric(gomock.Any(), metric).Return(metric, nil)
	inner.EXPECT().MakeBackup(gomock.Any()).Return(nil)

	stored, err := service.UpdateMetric(context.Background(), metric)
	require.NoError(t, err)
	require.Equal(t, metric, stored)
}

func TestSyncBackupDecorator_UpdateMetric_BackupErrorIsIgnored(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	inner := testdata.NewMockMetricServiceProvider(ctrl)
	service, err := cases.NewSyncBackupDecorator(inner, noop.New())
	require.NoError(t, err)

	metric := newCounterMetric(t, "requests", 1)
	inner.EXPECT().UpdateMetric(gomock.Any(), metric).Return(metric, nil)
	inner.EXPECT().MakeBackup(gomock.Any()).Return(entities.ErrInternalError)

	stored, err := service.UpdateMetric(context.Background(), metric)
	require.NoError(t, err)
	require.Equal(t, metric, stored)
}

func TestSyncBackupDecorator_UpdateMetric_UpdateErrorSkipsBackup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	inner := testdata.NewMockMetricServiceProvider(ctrl)
	service, err := cases.NewSyncBackupDecorator(inner, noop.New())
	require.NoError(t, err)

	metric := newCounterMetric(t, "requests", 1)
	inner.EXPECT().UpdateMetric(gomock.Any(), metric).Return(nil, entities.ErrInvalidParam)

	stored, err := service.UpdateMetric(context.Background(), metric)
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, stored)
}
