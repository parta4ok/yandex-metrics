package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/middleware"
)

type entry struct {
	level   string
	message string
	fields  map[string]any
}

type recordingLogger struct {
	entries []entry
}

func (l *recordingLogger) record(level string, message string, fields []any) {
	values := make(map[string]any, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		values[fields[i].(string)] = fields[i+1]
	}

	l.entries = append(l.entries, entry{level: level, message: message, fields: values})
}

func (l *recordingLogger) Debug(message string, fields ...any) { l.record("debug", message, fields) }
func (l *recordingLogger) Info(message string, fields ...any)  { l.record("info", message, fields) }
func (l *recordingLogger) Warn(message string, fields ...any)  { l.record("warn", message, fields) }
func (l *recordingLogger) Error(message string, fields ...any) { l.record("error", message, fields) }

func TestLogging(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantSize   int
	}{
		{
			name: "explicit status and body",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				resp.WriteHeader(http.StatusCreated)
				_, _ = resp.Write([]byte("hello"))
			},
			wantStatus: http.StatusCreated,
			wantSize:   5,
		},
		{
			name: "implicit ok on write",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				_, _ = resp.Write([]byte("ab"))
				_, _ = resp.Write([]byte("cde"))
			},
			wantStatus: http.StatusOK,
			wantSize:   5,
		},
		{
			name:       "empty response",
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
			wantSize:   0,
		},
		{
			name: "error response",
			handler: func(resp http.ResponseWriter, _ *http.Request) {
				http.Error(resp, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantSize:   len(http.StatusText(http.StatusNotFound)) + 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &recordingLogger{}
			handler := middleware.Logging(logger)(tt.handler)

			req := httptest.NewRequest(http.MethodPost, "/update/gauge/a/1", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Len(t, logger.entries, 1)

			got := logger.entries[0]
			require.Equal(t, "info", got.level)
			require.Equal(t, "/update/gauge/a/1", got.fields["uri"])
			require.Equal(t, http.MethodPost, got.fields["method"])
			require.Equal(t, tt.wantStatus, got.fields["status"])
			require.Equal(t, tt.wantSize, got.fields["size"])
			require.IsType(t, time.Duration(0), got.fields["duration"])
		})
	}
}
