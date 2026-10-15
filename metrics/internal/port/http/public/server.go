package public

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port"
	"github.com/parta4ok/yandex-metrics/metrics/pkg/dto"
	toolkitconfig "github.com/parta4ok/yandex-metrics/toolkit/config"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
	"github.com/parta4ok/yandex-metrics/toolkit/middleware"
)

const (
	contentTypeHeader    = "Content-Type"
	textPlainMediaType   = "text/plain"
	textPlainContentType = textPlainMediaType + "; charset=utf-8"
	textHTMLContentType  = "text/html; charset=utf-8"
	jsonMediaType        = "application/json"
	metricTypeParam      = "type"
	metricNameParam      = "name"
	metricValueParam     = "value"
)

const (
	updateBasePath = "/update"
	valueBasePath  = "/value"
)

//go:embed templates/metrics.html
var metricListTemplateContent string

var metricListTemplate = template.Must(
	template.New("metrics.html").Parse(metricListTemplateContent),
)

type Server struct {
	service         port.MetricServiceProvider
	handler         http.Handler
	httpServer      *http.Server
	certificateFile string
	keyFile         string
	logger          toolkitlogger.Logger
}

type Option func(*Server)

func WithTLS(certificateFile string, keyFile string) Option {
	return func(server *Server) {
		server.certificateFile = certificateFile
		server.keyFile = keyFile
	}
}

func NewServer(
	config toolkitconfig.HTTPServerConfig,
	service port.MetricServiceProvider,
	logger toolkitlogger.Logger,
	options ...Option,
) (*Server, error) {
	if config == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. config is nil")
	}

	address := config.HTTPAddress()
	if address == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. address is empty")
	}
	if service == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. metrics service is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new http server. logger is nil")
	}

	server := &Server{
		service: service,
		logger:  logger,
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
	router.Use(middleware.Logging(logger), middleware.Compressing())
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
	router.Get("/", s.listMetrics)
	router.Route(updateBasePath, func(router chi.Router) {
		router.Post("/", s.updateMetricJSON)
		router.Post(
			fmt.Sprintf("/{%s}/{%s}/{%s}", metricTypeParam, metricNameParam, metricValueParam),
			s.updateMetric,
		)
	})
	router.Route(valueBasePath, func(router chi.Router) {
		router.Post("/", s.getMetricJSON)
		router.Get(
			fmt.Sprintf("/{%s}/{%s}", metricTypeParam, metricNameParam),
			s.getMetric,
		)
	})
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

	if _, err := s.service.UpdateMetric(req.Context(), metric); err != nil {
		s.handleError(resp, err)
		return
	}

	resp.Header().Set(contentTypeHeader, textPlainContentType)
	resp.WriteHeader(http.StatusOK)
}

func (s *Server) updateMetricJSON(resp http.ResponseWriter, req *http.Request) {
	var request dto.UpdateMetricRequestDTO
	if err := s.decodeJSON(req, &request); err != nil {
		s.handleError(resp, err)
		return
	}

	metric, err := s.metricFromUpdateRequest(&request)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	stored, err := s.service.UpdateMetric(req.Context(), metric)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	s.writeMetricJSON(resp, stored)
}

func (s *Server) getMetricJSON(resp http.ResponseWriter, req *http.Request) {
	var request dto.GetMetricRequestDTO
	if err := s.decodeJSON(req, &request); err != nil {
		s.handleError(resp, err)
		return
	}

	metric, err := s.service.GetMetric(req.Context(), request.ID, entities.MType(request.MType))
	if err != nil {
		s.handleError(resp, err)
		return
	}

	s.writeMetricJSON(resp, metric)
}

func (s *Server) getMetric(resp http.ResponseWriter, req *http.Request) {
	metric, err := s.service.GetMetric(
		req.Context(),
		chi.URLParam(req, metricNameParam),
		entities.MType(chi.URLParam(req, metricTypeParam)),
	)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	value, err := metricTextValue(metric)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	resp.Header().Set(contentTypeHeader, textPlainContentType)
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write([]byte(value))
}

func (s *Server) listMetrics(resp http.ResponseWriter, req *http.Request) {
	metrics, err := s.service.ListMetrics(req.Context())
	if err != nil {
		s.handleError(resp, err)
		return
	}

	body, err := metricListBody(metrics)
	if err != nil {
		s.handleError(resp, err)
		return
	}

	resp.Header().Set(contentTypeHeader, textHTMLContentType)
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(body)
}

func (s *Server) handleError(resp http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, entities.ErrInvalidParam):
		status = http.StatusBadRequest
	case errors.Is(err, entities.ErrNotFound):
		status = http.StatusNotFound
	}
	if status == http.StatusInternalServerError {
		s.logger.Warn("handle HTTP request error", "error", err)
	}

	http.Error(resp, http.StatusText(status), status)
}

func (s *Server) decodeJSON(req *http.Request, target any) error {
	contentType, _, err := mime.ParseMediaType(req.Header.Get(contentTypeHeader))
	if err != nil || contentType != jsonMediaType {
		return errors.Wrap(entities.ErrInvalidParam, "decode json. expected application/json")
	}

	if err := json.NewDecoder(req.Body).Decode(target); err != nil {
		return errors.Wrap(entities.ErrInvalidParam, "decode json. body is invalid")
	}

	return nil
}

func (s *Server) writeMetricJSON(resp http.ResponseWriter, metric *entities.Metric) {
	body, err := json.Marshal(s.newMetricResponse(metric))
	if err != nil {
		s.handleError(resp, errors.Wrap(entities.ErrInternalError, "write metric json. marshal response"))
		return
	}

	resp.Header().Set(contentTypeHeader, jsonMediaType)
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(body)
}

func (s *Server) metricFromUpdateRequest(request *dto.UpdateMetricRequestDTO) (*entities.Metric, error) {
	metric, err := entities.NewMetric(request.ID, entities.MType(request.MType))
	if err != nil {
		return nil, errors.Wrap(err, "metric from update request. create metric")
	}

	metric.SetDelta(request.Delta)
	metric.SetValue(request.Value)
	metric.SetHash(request.Hash)

	return metric, nil
}

func (s *Server) newMetricResponse(metric *entities.Metric) dto.MetricResponseDTO {
	return dto.MetricResponseDTO{
		ID:    metric.ID(),
		MType: string(metric.MType()),
		Delta: metric.Delta(),
		Value: metric.Value(),
		Hash:  metric.Hash(),
	}
}

func (s *Server) validateContentType(req *http.Request) error {
	contentTypeHeaderValue := req.Header.Get(contentTypeHeader)
	if contentTypeHeaderValue == "" {
		return nil
	}

	contentType, _, err := mime.ParseMediaType(contentTypeHeaderValue)
	if err != nil || contentType != textPlainMediaType {
		return errors.Wrap(entities.ErrInvalidParam, "validate content type. expected text/plain")
	}

	return nil
}

func (s *Server) newMetric(id string, mType entities.MType, value string) (*entities.Metric, error) {
	metric, err := entities.NewMetric(id, mType)
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

func metricTextValue(metric *entities.Metric) (string, error) {
	if metric == nil {
		return "", errors.Wrap(entities.ErrInternalError, "metric text value. metric is nil")
	}

	switch metric.MType() {
	case entities.Counter:
		if metric.Delta() == nil {
			return "", errors.Wrap(entities.ErrInternalError, "metric text value. counter delta is nil")
		}

		return strconv.FormatInt(*metric.Delta(), 10), nil
	case entities.Gauge:
		if metric.Value() == nil {
			return "", errors.Wrap(entities.ErrInternalError, "metric text value. gauge value is nil")
		}

		return strconv.FormatFloat(*metric.Value(), 'f', -1, 64), nil
	default:
		return "", errors.Wrap(entities.ErrInternalError, "metric text value. metric type is invalid")
	}
}

func metricListBody(metrics []*entities.Metric) ([]byte, error) {
	type metricView struct {
		Name  string
		Type  entities.MType
		Value string
	}

	views := make([]metricView, 0, len(metrics))
	for _, metric := range metrics {
		value, err := metricTextValue(metric)
		if err != nil {
			return nil, errors.Wrap(err, "metric list body. get metric value")
		}

		views = append(views, metricView{
			Name:  metric.ID(),
			Type:  metric.MType(),
			Value: value,
		})
	}

	var body bytes.Buffer
	if err := metricListTemplate.Execute(&body, views); err != nil {
		return nil, errors.Wrap(err, "metric list body. execute template")
	}

	return body.Bytes(), nil
}
