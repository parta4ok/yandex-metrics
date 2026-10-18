package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/middleware"
)

func TestCompressing_CompressesResponse(t *testing.T) {
	t.Parallel()

	const body = `{"id":"memory","type":"gauge","value":1.5}`

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		acceptEncoding string
		wantCompressed bool
	}{
		{
			name: "json with explicit status",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				resp.WriteHeader(http.StatusOK)
				_, _ = resp.Write([]byte(body))
			},
			acceptEncoding: "gzip",
			wantCompressed: true,
		},
		{
			name: "html with charset and implicit status",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = resp.Write([]byte(body))
			},
			acceptEncoding: "gzip, deflate, br",
			wantCompressed: true,
		},
		{
			name: "gzip in a list with quality value, any case",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, _ = resp.Write([]byte(body))
			},
			acceptEncoding: "br, GZIP;q=0.8",
			wantCompressed: true,
		},
		{
			name: "similar encoding names are not gzip",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, _ = resp.Write([]byte(body))
			},
			acceptEncoding: "x-gzip, gzipx",
		},
		{
			name: "client does not accept gzip",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, _ = resp.Write([]byte(body))
			},
		},
		{
			name: "plain text is not compressed",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = resp.Write([]byte(body))
			},
			acceptEncoding: "gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			resp := httptest.NewRecorder()

			middleware.Compressing()(tt.handler).ServeHTTP(resp, req)

			require.Equal(t, http.StatusOK, resp.Code)
			if !tt.wantCompressed {
				require.Empty(t, resp.Header().Get("Content-Encoding"))
				require.Equal(t, body, resp.Body.String())
				return
			}

			require.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))
			require.Empty(t, resp.Header().Get("Content-Length"))
			require.Equal(t, body, gunzip(t, resp.Body.Bytes()))
		})
	}
}

func TestCompressing_DecompressesRequest(t *testing.T) {
	t.Parallel()

	const body = `{"id":"requests","type":"counter","delta":2}`

	var got string
	handler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		data, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		got = string(data)
		resp.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(gzipBytes(t, body)))
	req.Header.Set("Content-Encoding", "gzip")
	resp := httptest.NewRecorder()

	middleware.Compressing()(handler).ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, body, got)
}

func TestCompressing_InvalidGzipRequest(t *testing.T) {
	t.Parallel()

	called := false
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader([]byte("not gzip")))
	req.Header.Set("Content-Encoding", "gzip")
	resp := httptest.NewRecorder()

	middleware.Compressing()(handler).ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.False(t, called)
}

func gzipBytes(t *testing.T, value string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(value))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buffer.Bytes()
}

func gunzip(t *testing.T, data []byte) string {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)

	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(decoded)
}
