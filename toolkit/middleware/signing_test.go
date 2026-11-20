package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/middleware"
	"github.com/parta4ok/yandex-metrics/toolkit/signature"
)

const signingBody = `{"id":"requests","type":"counter","delta":2}`

func newTestSigner(t *testing.T) *signature.Signer {
	t.Helper()

	signer, err := signature.NewSigner("secret")
	require.NoError(t, err)

	return signer
}

func newEchoHandler(t *testing.T, called *bool) http.HandlerFunc {
	t.Helper()

	return func(resp http.ResponseWriter, req *http.Request) {
		*called = true

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		resp.Header().Set("Content-Type", "application/json")
		_, _ = resp.Write(body)
	}
}

func TestSigning_NilSignerIsPassThrough(t *testing.T) {
	t.Parallel()

	var called bool
	handler := middleware.Signing(nil)(newEchoHandler(t, &called))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(signingBody))
	req.Header.Set(signature.Header, "garbage")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, signingBody, resp.Body.String())
	require.Empty(t, resp.Header().Get(signature.Header))
}

func TestSigning_ValidRequestIsAcceptedAndResponseIsSigned(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	var called bool
	handler := middleware.Signing(signer)(newEchoHandler(t, &called))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(signingBody))
	req.Header.Set(signature.Header, signer.Sign([]byte(signingBody)))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	require.Equal(t, signingBody, resp.Body.String())
	require.True(t, signer.Verify(resp.Body.Bytes(), resp.Header().Get(signature.Header)))
}

func TestSigning_InvalidSignatureIsRejected(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	tests := []struct {
		name      string
		signature string
	}{
		{name: "wrong signature", signature: signer.Sign([]byte("other body"))},
		{name: "not base64", signature: "%%%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called bool
			handler := middleware.Signing(signer)(newEchoHandler(t, &called))

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(signingBody))
			req.Header.Set(signature.Header, tt.signature)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			require.False(t, called)
			require.Equal(t, http.StatusBadRequest, resp.Code)
			require.True(t, signer.Verify(resp.Body.Bytes(), resp.Header().Get(signature.Header)))
		})
	}
}

func TestSigning_RequestWithoutHeaderIsAccepted(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	var called bool
	handler := middleware.Signing(signer)(newEchoHandler(t, &called))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(signingBody))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, signingBody, resp.Body.String())
	require.True(t, signer.Verify(resp.Body.Bytes(), resp.Header().Get(signature.Header)))
}

func TestSigning_SignsEveryKindOfResponse(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name:       "implicit status",
			handler:    func(resp http.ResponseWriter, _ *http.Request) { _, _ = resp.Write([]byte("ok")) },
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "explicit status without body",
			handler:    func(resp http.ResponseWriter, _ *http.Request) { resp.WriteHeader(http.StatusNoContent) },
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "nothing written",
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
		},
		{
			name: "several writes and error status",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.WriteHeader(http.StatusNotFound)
				_, _ = resp.Write([]byte("not "))
				_, _ = resp.Write([]byte("found"))
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "not found",
		},
		{
			name: "explicit ok is not overwritten",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.WriteHeader(http.StatusOK)
				resp.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "status written twice keeps the first",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.WriteHeader(http.StatusAccepted)
				resp.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.Signing(signer)(tt.handler)

			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

			require.Equal(t, tt.wantStatus, resp.Code)
			require.Equal(t, tt.wantBody, resp.Body.String())
			require.NotEmpty(t, resp.Header().Get(signature.Header))
			require.True(t, signer.Verify([]byte(tt.wantBody), resp.Header().Get(signature.Header)))
		})
	}
}

func TestSigning_WithCompressionSignsUncompressedBody(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	var called bool
	handler := middleware.Compressing()(middleware.Signing(signer)(newEchoHandler(t, &called)))

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(signingBody))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &compressed)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set(signature.Header, signer.Sign([]byte(signingBody)))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))

	reader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, signingBody, string(body))
	require.True(t, signer.Verify(body, resp.Header().Get(signature.Header)))
}

func TestSigning_WithCompressionRejectsSignatureOfCompressedBody(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	var called bool
	handler := middleware.Compressing()(middleware.Signing(signer)(newEchoHandler(t, &called)))

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(signingBody))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(signature.Header, signer.Sign(compressed.Bytes()))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.False(t, called)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
