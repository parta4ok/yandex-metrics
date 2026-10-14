package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

const (
	contentTypeHeader = "Content-Type"
	jsonMediaType     = "application/json"
	updatePath        = "update"
)

var _ cases.MetricServiceClient = (*Client)(nil)

type Client struct {
	serverAddress string
	client        *http.Client
	logger        toolkitlogger.Logger
}

func NewClient(
	serverAddress string,
	client *http.Client,
	logger toolkitlogger.Logger,
) (*Client, error) {
	if serverAddress == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. server address is empty")
	}
	if client == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. HTTP client is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. logger is nil")
	}

	if !strings.Contains(serverAddress, "://") {
		serverAddress = "http://" + serverAddress
	}

	address, err := url.Parse(serverAddress)
	if err != nil || address.Scheme == "" || address.Host == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. server address is invalid")
	}

	return &Client{
		serverAddress: strings.TrimRight(serverAddress, "/"),
		client:        client,
		logger:        logger,
	}, nil
}

func (c *Client) UpdateAgentData(ctx context.Context, metric *entities.Metric) error {
	if metric == nil {
		return errors.Wrap(entities.ErrInvalidParam, "update agent data. metric is nil")
	}

	request, err := c.newUpdateRequestDTO(metric)
	if err != nil {
		return errors.Wrap(err, "update agent data. build request body")
	}

	body, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "update agent data. marshal request body")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.serverAddress+"/"+updatePath,
		bytes.NewReader(body),
	)
	if err != nil {
		return errors.Wrap(err, "update agent data. create request")
	}
	req.Header.Set(contentTypeHeader, jsonMediaType)

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "update agent data. send request")
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return errors.Wrap(err, "update agent data. read response body")
	}
	return c.handleResponseStatus(resp.StatusCode)
}

func (c *Client) handleResponseStatus(statusCode int) error {
	switch statusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		return errors.Wrap(entities.ErrInvalidParam, "update agent data. bad request")
	case http.StatusNotFound:
		return errors.Wrap(entities.ErrNotFound, "update agent data. metric not found")
	case http.StatusInternalServerError:
		return errors.Wrap(entities.ErrInternalError, "update agent data. internal server error")
	default:
		return errors.Wrapf(
			entities.ErrInternalError,
			"update agent data. unexpected response status: %d",
			statusCode,
		)
	}
}

func (c *Client) newUpdateRequestDTO(metric *entities.Metric) (*UpdateMetricRequestDTO, error) {
	request := &UpdateMetricRequestDTO{
		ID:    string(metric.Name()),
		MType: string(metric.MType()),
	}

	switch metric.MType() {
	case entities.Counter:
		if metric.Delta() == nil {
			return nil, errors.Wrap(entities.ErrInvalidParam, "new update request. counter delta is nil")
		}
		request.Delta = metric.Delta()
	case entities.Gauge:
		if metric.Value() == nil {
			return nil, errors.Wrap(entities.ErrInvalidParam, "new update request. gauge value is nil")
		}
		request.Value = metric.Value()
	default:
		return nil, errors.Wrap(entities.ErrInvalidParam, "new update request. metric type is invalid")
	}

	return request, nil
}
