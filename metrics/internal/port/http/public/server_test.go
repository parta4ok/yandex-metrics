package public_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/http/public"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/testdata"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_ServerUpdateMetric_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := testdata.NewMockMetricServiceProvider(ctrl)
	testaddress := ":8080"

	server, err := public.NewServer(testaddress, service)
	require.NoError(t, err)
	require.NotNil(t, server)

	var (
		id    = "someMetric"
		mtype = "counter"
		delta = int64(527)
	)

	testUpdateRequest := fmt.Sprintf("http://localhost:8080/update/%s/%s/%d", mtype, id, delta)
	req := httptest.NewRequest(http.MethodPost, testUpdateRequest, nil)
	req.Header.Set("Content-Type", "text/plain")
	require.NotNil(t, req)

	resp := httptest.NewRecorder()

	metric, err := entities.NewMetrics(
		id,
		entities.MType(mtype),
	)
	require.NoError(t, err)
	require.NotNil(t, metric)

	metric.SetDelta(&delta)
	require.NoError(t, metric.Validate())

	service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, metric *entities.Metrics) error {
			require.Equal(t, entities.Counter, metric.MType())
			require.Equal(t, id, metric.ID())
			require.Equal(t, delta, *metric.Delta())

			return nil
		})

	server.ServeHTTP(resp, req)

	require.NotNil(t, resp)

	require.Equal(t, resp.Header().Get("Content-Type"), "text/plain; charset=utf-8")
	require.Equal(t, resp.Result().StatusCode, http.StatusOK)
}

func Test_ServerUpdateMetric_NotFound(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		Return(entities.ErrNotFound)

	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, newUpdateMetricRequest())

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func Test_ServerUpdateMetric_InvalidParam(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		Return(entities.ErrInvalidParam)

	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, newUpdateMetricRequest())

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func Test_ServerUpdateMetric_InternalError(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		Return(entities.ErrInternalError)

	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, newUpdateMetricRequest())

	require.Equal(t, http.StatusInternalServerError, resp.Code)
}

func newTestServer(t *testing.T) (*public.Server, *testdata.MockMetricServiceProvider) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	service := testdata.NewMockMetricServiceProvider(ctrl)
	server, err := public.NewServer(":8080", service)
	require.NoError(t, err)

	return server, service
}

func newUpdateMetricRequest() *http.Request {
	req := httptest.NewRequest(
		http.MethodPost,
		"http://localhost:8080/update/counter/someMetric/527",
		nil,
	)
	req.Header.Set("Content-Type", "text/plain")

	return req
}
