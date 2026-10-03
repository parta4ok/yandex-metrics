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
			name:   "missing content type",
			method: http.MethodPost,
			target: "/update/gauge/metric/1.5",
			status: http.StatusOK,
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

func TestServerGetMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		setupService func(*testing.T, *testdata.MockMetricServiceProvider)
		status       int
		body         string
	}{
		{
			name:   "counter",
			target: "/value/counter/requests",
			setupService: func(t *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "requests", entities.Counter).
					Return(newCounterMetric(t, "requests", 42), nil)
			},
			status: http.StatusOK,
			body:   "42",
		},
		{
			name:   "gauge",
			target: "/value/gauge/memory",
			setupService: func(t *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "memory", entities.Gauge).
					Return(newGaugeMetric(t, "memory", 1.5), nil)
			},
			status: http.StatusOK,
			body:   "1.5",
		},
		{
			name:   "not found",
			target: "/value/gauge/memory",
			setupService: func(_ *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "memory", entities.Gauge).
					Return(nil, entities.ErrNotFound)
			},
			status: http.StatusNotFound,
		},
		{
			name:   "invalid type",
			target: "/value/unknown/memory",
			setupService: func(_ *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "memory", entities.MType("unknown")).
					Return(nil, entities.ErrInvalidParam)
			},
			status: http.StatusBadRequest,
		},
		{
			name:   "service error",
			target: "/value/gauge/memory",
			setupService: func(_ *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "memory", entities.Gauge).
					Return(nil, entities.ErrInternalError)
			},
			status: http.StatusInternalServerError,
		},
		{
			name:   "invalid metric from service",
			target: "/value/gauge/memory",
			setupService: func(_ *testing.T, service *testdata.MockMetricServiceProvider) {
				service.EXPECT().
					GetMetric(gomock.Any(), "memory", entities.Gauge).
					Return(&entities.Metrics{}, nil)
			},
			status: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, service := newTestServer(t)
			tt.setupService(t, service)

			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, tt.target, nil))

			require.Equal(t, tt.status, resp.Code)
			if tt.status == http.StatusOK {
				require.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
				require.Equal(t, tt.body, resp.Body.String())
			}
		})
	}
}

func TestServerListMetrics(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server, service := newTestServer(t)
		service.EXPECT().ListMetrics(gomock.Any()).Return(
			[]*entities.Metrics{
				newCounterMetric(t, "requests", 42),
				newGaugeMetric(t, "<memory>", 1.5),
			},
			nil,
		)

		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

		require.Equal(t, http.StatusOK, resp.Code)
		require.Equal(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))
		require.Contains(t, resp.Body.String(), "requests (counter): 42")
		require.Contains(t, resp.Body.String(), "&lt;memory&gt; (gauge): 1.5")
	})

	t.Run("service error", func(t *testing.T) {
		t.Parallel()

		server, service := newTestServer(t)
		service.EXPECT().ListMetrics(gomock.Any()).Return(nil, entities.ErrInternalError)

		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

		require.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("invalid metric from service", func(t *testing.T) {
		t.Parallel()

		server, service := newTestServer(t)
		service.EXPECT().ListMetrics(gomock.Any()).Return([]*entities.Metrics{nil}, nil)

		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

		require.Equal(t, http.StatusInternalServerError, resp.Code)
	})
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

func newCounterMetric(t *testing.T, id string, delta int64) *entities.Metrics {
	t.Helper()

	metric, err := entities.NewMetrics(id, entities.Counter)
	require.NoError(t, err)
	metric.SetDelta(&delta)

	return metric
}

func newGaugeMetric(t *testing.T, id string, value float64) *entities.Metrics {
	t.Helper()

	metric, err := entities.NewMetrics(id, entities.Gauge)
	require.NoError(t, err)
	metric.SetValue(&value)

	return metric
}
