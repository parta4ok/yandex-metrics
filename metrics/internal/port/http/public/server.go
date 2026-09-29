package public

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port"
)

const (
	contentTypeHeader    = "Content-Type"
	textPlainMediaType   = "text/plain"
	textPlainContentType = textPlainMediaType + "; charset=utf-8"
	metricTypeParam      = "type"
	metricNameParam      = "name"
	metricValueParam     = "value"
)

var (
	updateBasePath   = "/update"
	updateMetricPath = fmt.Sprintf(
		"%s/{%s}/{%s}/{%s}",
		updateBasePath,
		metricTypeParam,
		metricNameParam,
		metricValueParam,
	)
)

type Server struct {
	service         port.MetricServiceProvider
	handler         http.Handler
	httpServer      *http.Server
	certificateFile string
	keyFile         string
}

type Option func(*Server)

func WithTLS(certificateFile string, keyFile string) Option {
	return func(server *Server) {
		server.certificateFile = certificateFile
		server.keyFile = keyFile
	}
}

func NewServer(address string, service port.MetricServiceProvider, options ...Option) (*Server, error) {
	if address == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. address is empty")
	}
	if service == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. metrics service is nil")
	}

	server := &Server{
		service: service,
	}
	for _, option := range options {
		if option != nil {
			option(server)
		}
	}
	if (server.certificateFile == "") != (server.keyFile == "") {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. TLS certificate and key are required")
	}

	router := chi.NewRouter()
	server.registerRoutes(router)
	server.handler = router

	server.httpServer = &http.Server{
		Addr:    address,
		Handler: server,
	}

	return server, nil
}

func (s *Server) Start(ctx context.Context) error {
	_ = ctx

	if s.certificateFile != "" {
		return s.httpServer.ListenAndServeTLS(s.certificateFile, s.keyFile)
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	s.handler.ServeHTTP(resp, req)
}

func (s *Server) registerRoutes(router chi.Router) {
	router.Method(http.MethodPost, updateMetricPath, http.HandlerFunc(s.updateMetric))
}

func (s *Server) updateMetric(resp http.ResponseWriter, req *http.Request) {
	if err := s.validateContentType(req); err != nil {
		s.handleError(resp, err)
		return
	}

	metric, err := s.newMetric(
		chi.URLParam(req, metricNameParam),
		entities.MType(chi.URLParam(req, metricTypeParam)),
		chi.URLParam(req, metricValueParam),
	)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	if err := s.service.UpdateMetric(req.Context(), metric); err != nil {
		s.handleError(resp, err)
		return
	}

	resp.Header().Set(contentTypeHeader, textPlainContentType)
	resp.WriteHeader(http.StatusOK)
}

func (s *Server) handleError(resp http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, entities.ErrInvalidParam):
		status = http.StatusBadRequest
	case errors.Is(err, entities.ErrNotFound):
		status = http.StatusNotFound
	}

	http.Error(resp, http.StatusText(status), status)
}

func (s *Server) validateContentType(req *http.Request) error {
	contentType, _, err := mime.ParseMediaType(req.Header.Get(contentTypeHeader))
	if err != nil || contentType != textPlainMediaType {
		return errors.Wrap(entities.ErrInvalidParam, "validate content type. expected text/plain")
	}

	return nil
}

func (s *Server) newMetric(id string, mType entities.MType, value string) (*entities.Metrics, error) {
	metric, err := entities.NewMetrics(id, mType)
	if err != nil {
		return nil, errors.Wrap(err, "new metric. create metric")
	}

	switch mType {
	case entities.Counter:
		delta, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, errors.Wrap(entities.ErrInvalidParam, "new metric. counter value is invalid")
		}
		metric.SetDelta(&delta)
	case entities.Gauge:
		gaugeValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.Wrap(entities.ErrInvalidParam, "new metric. gauge value is invalid")
		}
		metric.SetValue(&gaugeValue)
	}

	return metric, nil
}
