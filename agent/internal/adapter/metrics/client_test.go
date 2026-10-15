package metrics_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	adapter "github.com/parta4ok/yandex-metrics/agent/internal/adapter/metrics"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{}
	tests := []struct {
		name      string
		address   string
		client    *http.Client
		wantError error
	}{
		{name: "empty address", client: httpClient, wantError: entities.ErrInvalidParam},
		{name: "invalid address", address: "://metrics", client: httpClient, wantError: entities.ErrInvalidParam},
		{name: "nil client", address: "http://localhost:8080", wantError: entities.ErrInvalidParam},
		{name: "bare address", address: "localhost:8080", client: httpClient},
		{name: "URL address", address: "http://localhost:8080/", client: httpClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := adapter.NewClient(tt.address, tt.client, noop.New())
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				require.Nil(t, client)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestClient_UpdateAgentData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		metric     *entities.Metric
		statusCode int
		transport  error
		wantBody   string
		wantErr    error
	}{
		{name: "nil metric", wantErr: entities.ErrInvalidParam},
		{name: "counter without delta", metric: newMetric(t, entities.PollCount), wantErr: entities.ErrInvalidParam},
		{name: "gauge without value", metric: newMetric(t, entities.Alloc), wantErr: entities.ErrInvalidParam},
		{name: "counter", metric: counterMetric(t, 42), statusCode: http.StatusOK, wantBody: `{"id":"PollCount","type":"counter","delta":42}`},
		{name: "gauge", metric: gaugeMetric(t, 1.5), statusCode: http.StatusOK, wantBody: `{"id":"Alloc","type":"gauge","value":1.5}`},
		{name: "bad request", metric: gaugeMetric(t, 1), statusCode: http.StatusBadRequest, wantErr: entities.ErrInvalidParam},
		{name: "not found", metric: gaugeMetric(t, 1), statusCode: http.StatusNotFound, wantErr: entities.ErrNotFound},
		{name: "internal error", metric: gaugeMetric(t, 1), statusCode: http.StatusInternalServerError, wantErr: entities.ErrInternalError},
		{name: "unexpected status", metric: gaugeMetric(t, 1), statusCode: http.StatusTeapot, wantErr: entities.ErrInternalError},
		{name: "transport error", metric: gaugeMetric(t, 1), transport: errors.New("unavailable"), wantErr: errors.New("unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := adapter.NewClient("http://metrics.example/", &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, "/update", req.URL.Path)
					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					require.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
					require.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
					if tt.wantBody != "" {
						reader, err := gzip.NewReader(req.Body)
						require.NoError(t, err)
						body, err := io.ReadAll(reader)
						require.NoError(t, err)
						require.JSONEq(t, tt.wantBody, string(body))
					}
					if tt.transport != nil {
						return nil, tt.transport
					}

					header := make(http.Header)
					var responseBody bytes.Buffer
					if tt.statusCode == http.StatusOK {
						header.Set("Content-Encoding", "gzip")
						writer := gzip.NewWriter(&responseBody)
						_, err := writer.Write([]byte(`{"id":"Alloc","type":"gauge","value":1}`))
						require.NoError(t, err)
						require.NoError(t, writer.Close())
					} else {
						responseBody.WriteString("response")
					}

					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(&responseBody),
						Header:     header,
					}, nil
				}),
			}, noop.New())
			require.NoError(t, err)

			err = client.UpdateAgentData(context.Background(), tt.metric)
			if tt.wantErr != nil {
				if tt.transport != nil {
					require.ErrorContains(t, err, tt.transport.Error())
				} else {
					require.ErrorIs(t, err, tt.wantErr)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func newMetric(t *testing.T, name entities.MName) *entities.Metric {
	t.Helper()
	metric, err := entities.NewMetric(name)
	require.NoError(t, err)
	return metric
}

func counterMetric(t *testing.T, delta int64) *entities.Metric {
	t.Helper()
	metric := newMetric(t, entities.PollCount)
	require.NoError(t, metric.SetDelta(delta))
	return metric
}

func gaugeMetric(t *testing.T, value float64) *entities.Metric {
	t.Helper()
	metric := newMetric(t, entities.Alloc)
	require.NoError(t, metric.SetValue(value))
	return metric
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
