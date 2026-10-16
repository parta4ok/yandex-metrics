package middleware

import (
	"net/http"
	"time"

	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func newResponseWriter(resp http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: resp,
		status:         http.StatusOK,
	}
}

func (w *responseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.wroteHeader = true

	size, err := w.ResponseWriter.Write(data)
	w.size += size

	return size, err
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func Logging(logger toolkitlogger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			start := time.Now()
			recorder := newResponseWriter(resp)

			next.ServeHTTP(recorder, req)

			logger.Info(
				"handle HTTP request",
				"uri", req.RequestURI,
				"method", req.Method,
				"duration", time.Since(start),
				"status", recorder.status,
				"size", recorder.size,
			)
		})
	}
}
