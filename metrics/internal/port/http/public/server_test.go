package public_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/http/public"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/testdata"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	nooppinger "github.com/parta4ok/yandex-metrics/toolkit/pinger/noop"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_ServerUpdateMetric_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := testdata.NewMockMetricServiceProvider(ctrl)
	testaddress := ":8080"

	server, err := public.NewServer(serverConfig(testaddress), service, noop.New())
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

	metric, err := entities.NewMetric(
		id,
		entities.MType(mtype),
	)
	require.NoError(t, err)
	require.NotNil(t, metric)

	metric.SetDelta(&delta)
	require.NoError(t, metric.Validate())

	service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, metric *entities.Metric) (*entities.Metric, error) {
			require.Equal(t, entities.Counter, metric.MType())
			require.Equal(t, id, metric.ID())
			require.Equal(t, delta, *metric.Delta())

			return metric, nil
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
		Return(nil, entities.ErrNotFound)

	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, newUpdateMetricRequest())

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func Test_ServerUpdateMetric_InvalidParam(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		Return(nil, entities.ErrInvalidParam)

	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, newUpdateMetricRequest())

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func Test_ServerUpdateMetric_InternalError(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		Return(nil, entities.ErrInternalError)

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
				service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).Return(nil, nil)
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
					Return(&entities.Metric{}, nil)
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
			[]*entities.Metric{
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
		service.EXPECT().ListMetrics(gomock.Any()).Return([]*entities.Metric{nil}, nil)

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
	server, err := public.NewServer(serverConfig(":8080"), service, noop.New())
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

func newCounterMetric(t *testing.T, id string, delta int64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Counter)
	require.NoError(t, err)
	metric.SetDelta(&delta)

	return metric
}

func newGaugeMetric(t *testing.T, id string, value float64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Gauge)
	require.NoError(t, err)
	metric.SetValue(&value)

	return metric
}

type serverConfig string

func (c serverConfig) HTTPAddress() string {
	return string(c)
}

func newJSONRequest(target string, body string, contentType string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req
}

func TestServerUpdateMetricJSON_ReturnsStoredMetric(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetric(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, metric *entities.Metric) (*entities.Metric, error) {
			require.Equal(t, "requests", metric.ID())
			require.Equal(t, entities.Counter, metric.MType())
			require.Equal(t, int64(2), *metric.Delta())

			return newCounterMetric(t, "requests", 5), nil
		})

	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, newJSONRequest(
		"/update",
		`{"id":"requests","type":"counter","delta":2}`,
		"application/json; charset=utf-8",
	))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	require.JSONEq(t, `{"id":"requests","type":"counter","delta":5}`, resp.Body.String())
}

func TestServerUpdateMetricJSON_InvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{name: "missing content type", body: `{"id":"a","type":"gauge","value":1}`},
		{name: "text content type", body: `{"id":"a","type":"gauge","value":1}`, contentType: "text/plain"},
		{name: "malformed body", body: `{"id":`, contentType: "application/json"},
		{name: "unknown type", body: `{"id":"a","type":"unknown"}`, contentType: "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := newTestServer(t)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, newJSONRequest("/update", tt.body, tt.contentType))

			require.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func TestServerGetMetricJSON_ReturnsMetric(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		GetMetric(gomock.Any(), "memory", entities.Gauge).
		Return(newGaugeMetric(t, "memory", 1.5), nil)

	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, newJSONRequest(
		"/value",
		`{"id":"memory","type":"gauge"}`,
		"application/json",
	))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	require.JSONEq(t, `{"id":"memory","type":"gauge","value":1.5}`, resp.Body.String())
}

func TestServerGetMetricJSON_NotFound(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		GetMetric(gomock.Any(), "memory", entities.Gauge).
		Return(nil, entities.ErrNotFound)

	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, newJSONRequest(
		"/value",
		`{"id":"memory","type":"gauge"}`,
		"application/json",
	))

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestServerPing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
	}{
		{name: "database is available", wantStatus: http.StatusOK},
		{name: "pinger is not configured", pingErr: nooppinger.ErrNotConfigured, wantStatus: http.StatusInternalServerError},
		{name: "database is unavailable", pingErr: errors.New("connection refused"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, service := newTestServer(t)
			service.EXPECT().Ping(gomock.Any()).Return(tt.pingErr)

			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))

			require.Equal(t, tt.wantStatus, resp.Code)
			if tt.pingErr != nil {
				require.NotContains(t, resp.Body.String(), tt.pingErr.Error())
			}
		})
	}
}

func TestServerUpdateMetricsJSON_ReturnsStoredMetrics(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetrics(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, metrics []*entities.Metric) ([]*entities.Metric, error) {
			require.Len(t, metrics, 2)
			require.Equal(t, "requests", metrics[0].ID())
			require.Equal(t, int64(2), *metrics[0].Delta())
			require.Equal(t, "memory", metrics[1].ID())
			require.Equal(t, 1.5, *metrics[1].Value())

			return []*entities.Metric{
				newCounterMetric(t, "requests", 5),
				newGaugeMetric(t, "memory", 1.5),
			}, nil
		})

	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, newJSONRequest(
		"/updates/",
		`[{"id":"requests","type":"counter","delta":2},{"id":"memory","type":"gauge","value":1.5}]`,
		"application/json",
	))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	require.JSONEq(
		t,
		`[{"id":"requests","type":"counter","delta":5},{"id":"memory","type":"gauge","value":1.5}]`,
		resp.Body.String(),
	)
}

func TestServerUpdateMetricsJSON_Gzip(t *testing.T) {
	t.Parallel()

	server, service := newTestServer(t)
	service.EXPECT().
		UpdateMetrics(gomock.Any(), gomock.Any()).
		Return([]*entities.Metric{newCounterMetric(t, "requests", 2)}, nil)

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(`[{"id":"requests","type":"counter","delta":2}]`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/updates/", &compressed)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))

	reader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"requests","type":"counter","delta":2}]`, string(body))
}

func TestServerUpdateMetricsJSON_InvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{name: "missing content type", body: `[{"id":"a","type":"gauge","value":1}]`},
		{name: "text content type", body: `[{"id":"a","type":"gauge","value":1}]`, contentType: "text/plain"},
		{name: "malformed body", body: `[{"id":`, contentType: "application/json"},
		{name: "object instead of array", body: `{"id":"a","type":"gauge","value":1}`, contentType: "application/json"},
		{name: "unknown type", body: `[{"id":"a","type":"unknown"}]`, contentType: "application/json"},
		{name: "empty id", body: `[{"id":"","type":"gauge","value":1}]`, contentType: "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := newTestServer(t)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, newJSONRequest("/updates/", tt.body, tt.contentType))

			require.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func TestServerUpdateMetricsJSON_ServiceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
	}{
		{name: "empty batch", body: `[]`, serviceErr: entities.ErrInvalidParam, wantStatus: http.StatusBadRequest},
		{name: "null batch", body: `null`, serviceErr: entities.ErrInvalidParam, wantStatus: http.StatusBadRequest},
		{
			name:       "internal error",
			body:       `[{"id":"a","type":"gauge","value":1}]`,
			serviceErr: entities.ErrInternalError,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, service := newTestServer(t)
			service.EXPECT().UpdateMetrics(gomock.Any(), gomock.Any()).Return(nil, tt.serviceErr)

			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, newJSONRequest("/updates/", tt.body, "application/json"))

			require.Equal(t, tt.wantStatus, resp.Code)
		})
	}
}

func TestServer_TemporaryErrorReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(service *testdata.MockMetricServiceProvider)
		request func() *http.Request
	}{
		{
			name: "update metric",
			expect: func(service *testdata.MockMetricServiceProvider) {
				service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).Return(nil, entities.ErrTemporary)
			},
			request: newUpdateMetricRequest,
		},
		{
			name: "update metric json",
			expect: func(service *testdata.MockMetricServiceProvider) {
				service.EXPECT().UpdateMetric(gomock.Any(), gomock.Any()).Return(nil, entities.ErrTemporary)
			},
			request: func() *http.Request {
				return newJSONRequest("/update", `{"id":"a","type":"gauge","value":1}`, "application/json")
			},
		},
		{
			name: "update metrics",
			expect: func(service *testdata.MockMetricServiceProvider) {
				service.EXPECT().UpdateMetrics(gomock.Any(), gomock.Any()).Return(nil, entities.ErrTemporary)
			},
			request: func() *http.Request {
				return newJSONRequest("/updates/", `[{"id":"a","type":"gauge","value":1}]`, "application/json")
			},
		},
		{
			name: "get metric",
			expect: func(service *testdata.MockMetricServiceProvider) {
				service.EXPECT().GetMetric(gomock.Any(), "a", entities.Gauge).Return(nil, entities.ErrTemporary)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/value/gauge/a", nil)
			},
		},
		{
			name: "list metrics",
			expect: func(service *testdata.MockMetricServiceProvider) {
				service.EXPECT().ListMetrics(gomock.Any()).Return(nil, entities.ErrTemporary)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, service := newTestServer(t)
			tt.expect(service)

			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, tt.request())

			require.Equal(t, http.StatusServiceUnavailable, resp.Code)
			require.NotContains(t, resp.Body.String(), entities.ErrTemporary.Error())
		})
	}
}
