package retry_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/retry"
)

type doerFunc func(req *http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true

	return nil
}

func newResponse(status int) (*http.Response, *trackedBody) {
	body := &trackedBody{Reader: bytes.NewBufferString("response")}

	return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}, body
}

func newRequest(t *testing.T, ctx context.Context, payload string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://metrics.example/updates/", bytes.NewBufferString(payload))
	require.NoError(t, err)

	return req
}

func doRequest(t *testing.T, client *retry.Client, req *http.Request) (int, error) {
	t.Helper()

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func newClient(t *testing.T, doer retry.Doer) *retry.Client {
	t.Helper()

	client, err := retry.NewClient(doer, retry.WithDelays(10*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond))
	require.NoError(t, err)

	return client
}

func TestNewClient_NilDoer(t *testing.T) {
	t.Parallel()

	client, err := retry.NewClient(nil)

	require.ErrorIs(t, err, retry.ErrInvalidParam)
	require.Nil(t, client)
}

func TestClient_Do_SuccessWithoutRetry(t *testing.T) {
	t.Parallel()

	var calls int
	client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		calls++
		resp, _ := newResponse(http.StatusOK)

		return resp, nil
	}))

	status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 1, calls)
}

func TestClient_Do_RetriesServiceUnavailable(t *testing.T) {
	t.Parallel()

	var (
		calls  int
		bodies []string
		first  *trackedBody
	)
	client := newClient(t, doerFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		payload, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		bodies = append(bodies, string(payload))

		if calls < 3 {
			resp, body := newResponse(http.StatusServiceUnavailable)
			if first == nil {
				first = body
			}

			return resp, nil
		}
		resp, _ := newResponse(http.StatusOK)

		return resp, nil
	}))

	status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 3, calls)
	require.Equal(t, []string{"payload", "payload", "payload"}, bodies)
	require.True(t, first.closed)
}

func TestClient_Do_ExhaustedServiceUnavailable(t *testing.T) {
	t.Parallel()

	var calls int
	bodies := make([]*trackedBody, 0, 4)
	client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		calls++
		resp, body := newResponse(http.StatusServiceUnavailable)
		bodies = append(bodies, body)

		return resp, nil
	}))

	started := time.Now()
	status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

	require.Error(t, err)
	require.Zero(t, status)
	require.Equal(t, 4, calls)
	require.GreaterOrEqual(t, time.Since(started), 60*time.Millisecond)
	for _, body := range bodies {
		require.True(t, body.closed)
	}
}

func TestClient_Do_RetriesConnectionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "connection refused", err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
		{name: "eof", err: io.EOF},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int
			client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return nil, tt.err
				}
				resp, _ := newResponse(http.StatusOK)

				return resp, nil
			}))

			status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			require.Equal(t, 2, calls)
		})
	}
}

func TestClient_Do_ConnectionErrorExhausted(t *testing.T) {
	t.Parallel()

	var calls int
	connectionErr := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		calls++

		return nil, connectionErr
	}))

	status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

	require.ErrorIs(t, err, syscall.ECONNREFUSED)
	require.Zero(t, status)
	require.Equal(t, 4, calls)
}

func TestClient_Do_DoesNotRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		err    error
	}{
		{name: "bad request", status: http.StatusBadRequest},
		{name: "not found", status: http.StatusNotFound},
		{name: "internal server error", status: http.StatusInternalServerError},
		{name: "unknown error", err: errors.New("boom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int
			client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if tt.err != nil {
					return nil, tt.err
				}
				resp, _ := newResponse(tt.status)

				return resp, nil
			}))

			status, err := doRequest(t, client, newRequest(t, context.Background(), "payload"))

			require.Equal(t, 1, calls)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.status, status)
		})
	}
}

func TestClient_Do_StopsWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	client, err := retry.NewClient(
		doerFunc(func(*http.Request) (*http.Response, error) {
			calls++
			time.AfterFunc(20*time.Millisecond, cancel)

			return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
		}),
		retry.WithDelays(time.Hour, time.Hour, time.Hour),
	)
	require.NoError(t, err)

	started := time.Now()
	status, err := doRequest(t, client, newRequest(t, ctx, "payload"))

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, status)
	require.Equal(t, 1, calls)
	require.Less(t, time.Since(started), time.Second)
}

func TestClient_Do_InterruptsWaitOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var (
		calls int
		body  *trackedBody
	)
	client, err := retry.NewClient(
		doerFunc(func(*http.Request) (*http.Response, error) {
			calls++
			resp, trackedResponseBody := newResponse(http.StatusServiceUnavailable)
			body = trackedResponseBody

			return resp, nil
		}),
		retry.WithDelays(time.Hour, time.Hour, time.Hour),
	)
	require.NoError(t, err)

	started := time.Now()
	status, err := doRequest(t, client, newRequest(t, ctx, "payload"))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, status)
	require.True(t, body.closed)
	require.Equal(t, 1, calls)
	require.Less(t, time.Since(started), time.Second)
}

func TestClient_Do_DoesNotRetryNonReplayableBody(t *testing.T) {
	t.Parallel()

	var calls int
	client := newClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		calls++
		resp, _ := newResponse(http.StatusServiceUnavailable)

		return resp, nil
	}))

	req, err := http.NewRequest(http.MethodPost, "http://metrics.example/", io.NopCloser(bytes.NewBufferString("payload")))
	require.NoError(t, err)
	require.Nil(t, req.GetBody)

	status, err := doRequest(t, client, req)

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, 1, calls)
}

func TestClient_Do_WithHTTPClient(t *testing.T) {
	t.Parallel()

	var (
		calls  atomic.Int32
		bodies = make(chan string, 4)
	)
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		payload, _ := io.ReadAll(req.Body)
		bodies <- string(payload)

		if calls.Add(1) < 3 {
			http.Error(resp, "unavailable", http.StatusServiceUnavailable)
			return
		}
		resp.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newClient(t, server.Client())
	req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewBufferString("payload"))
	require.NoError(t, err)

	status, err := doRequest(t, client, req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, int32(3), calls.Load())
	for range 3 {
		require.Equal(t, "payload", <-bodies)
	}
}

func TestClient_Do_ServerIsDown(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	address := server.URL
	server.Close()

	client := newClient(t, http.DefaultClient)
	req, err := http.NewRequest(http.MethodPost, address, bytes.NewBufferString("payload"))
	require.NoError(t, err)

	status, err := doRequest(t, client, req)

	require.Error(t, err)
	require.Zero(t, status)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
}
