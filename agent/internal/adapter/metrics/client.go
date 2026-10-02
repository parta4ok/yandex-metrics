package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

const (
	contentTypeHeader  = "Content-Type"
	textPlainMediaType = "text/plain"
	updatePath         = "update"
)

var _ cases.MetricServiceClient = (*Client)(nil)

type Client struct {
	serverAddress string
	client        *http.Client
}

func NewClient(serverAddress string, client *http.Client) (*Client, error) {
	if serverAddress == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. server address is empty")
	}
	if client == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. HTTP client is nil")
	}

	address, err := url.Parse(serverAddress)
	if err != nil || address.Scheme == "" || address.Host == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new metrics client. server address is invalid")
	}

	return &Client{
		serverAddress: strings.TrimRight(serverAddress, "/"),
		client:        client,
	}, nil
}

func (c *Client) UpdateAgentData(ctx context.Context, metric *entities.Metric) error {
	if metric == nil {
		return errors.Wrap(entities.ErrInvalidParam, "update agent data. metric is nil")
	}

	value, err := metricValue(metric)
	if err != nil {
		return errors.Wrap(err, "update agent data. get metric value")
	}

	requestURL := fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		c.serverAddress,
		updatePath,
		metric.MType(),
		metric.Name(),
		value,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return errors.Wrap(err, "update agent data. create request")
	}
	req.Header.Set(contentTypeHeader, textPlainMediaType)

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

func metricValue(metric *entities.Metric) (string, error) {
	switch metric.MType() {
	case entities.Counter:
		if metric.Delta() == nil {
			return "", errors.Wrap(entities.ErrInvalidParam, "get metric value. counter delta is nil")
		}

		return strconv.FormatInt(*metric.Delta(), 10), nil
	case entities.Gauge:
		if metric.Value() == nil {
			return "", errors.Wrap(entities.ErrInvalidParam, "get metric value. gauge value is nil")
		}

		return strconv.FormatFloat(*metric.Value(), 'f', -1, 64), nil
	default:
		return "", errors.Wrap(entities.ErrInvalidParam, "get metric value. metric type is invalid")
	}
}
