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

func TestServerUpdateMetric_InvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		target      string
		contentType string
		status      int
	}{
		{
			name:        "invalid content type",
			method:      http.MethodPost,
			target:      "/update/counter/metric/1",
			contentType: "application/json",
			status:      http.StatusBadRequest,
		},
		{
			name:        "content type parameters are accepted",
			method:      http.MethodPost,
			target:      "/update/gauge/metric/1.5",
			contentType: "text/plain; charset=utf-8",
			status:      http.StatusOK,
		},
		{
			name:        "invalid metric type",
			method:      http.MethodPost,
			target:      "/update/unknown/metric/1",
			contentType: "text/plain",
			status:      http.StatusBadRequest,
		},
		{
			name:        "invalid counter value",
			method:      http.MethodPost,
			target:      "/update/counter/metric/not-a-number",
			contentType: "text/plain",
			status:      http.StatusBadRequest,
		},
		{
			name:        "missing route value",
			method:      http.MethodPost,
			target:      "/update/counter/metric",
			contentType: "text/plain",
			status:      http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, service := newTestServer(t)
			if tt.status == http.StatusOK {
				service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).Return(nil)
			}

			req := httptest.NewRequest(tt.method, tt.target, nil)
			req.Header.Set("Content-Type", tt.contentType)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			require.Equal(t, tt.status, resp.Code)
		})
	}
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
