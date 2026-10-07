package cases

import (
	"context"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

type AgentService struct {
	metricServiceClient MetricServiceClient
	dataProvider        DataProvider
	storage             Storage
}

func NewAgentService(
	metricServiceClient MetricServiceClient,
	dataProvider DataProvider,
	storage Storage,
) (*AgentService, error) {
	if metricServiceClient == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. metric service client is nil")
	}
	if dataProvider == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. data provider is nil")
	}
	if storage == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. storage is nil")
	}

	return &AgentService{
		metricServiceClient: metricServiceClient,
		dataProvider:        dataProvider,
		storage:             storage,
	}, nil
}

func (s *AgentService) UpdateMetrics(ctx context.Context) error {
	actualMetrics, err := s.dataProvider.GetActualAgentData(ctx)
	if err != nil {
		return errors.Wrap(err, "update metrics. get actual agent data")
	}

	if err := s.updateMetrics(actualMetrics); err != nil {
		return errors.Wrap(err, "update metrics. validate actual data")
	}
	if err := s.storage.UpdateGauges(ctx, actualMetrics); err != nil {
		return errors.Wrap(err, "update metrics. update gauges")
	}
	if err := s.storage.IncrementCounter(ctx, entities.PollCount, 1); err != nil {
		return errors.Wrap(err, "update metrics. increment poll count")
	}

	return nil
}

func (s *AgentService) SendMetrics(ctx context.Context) error {
	metrics, err := s.storage.GetAgentData(ctx)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil
		}

		return errors.Wrap(err, "send metrics. get agent data")
	}

	if err := s.sendMetrics(ctx, metrics); err != nil {
		return errors.Wrap(err, "send metrics. send agent data")
	}

	return nil
}

func (s *AgentService) updateMetrics(actualMetrics *entities.Metrics) error {
	actualMetricList, err := actualMetrics.All()
	if err != nil {
		return errors.Wrap(err, "update metrics. list actual metrics")
	}

	for _, actualMetric := range actualMetricList {
		if actualMetric.MType() != entities.Gauge {
			return errors.Wrap(entities.ErrInvalidParam, "update metrics. actual metric type is not gauge")
		}
	}

	return nil
}

func (s *AgentService) sendMetrics(ctx context.Context, metrics *entities.Metrics) error {
	metricList, err := metrics.All()
	if err != nil {
		return errors.Wrap(err, "send metrics. list metrics")
	}

	for _, metric := range metricList {
		if err := s.metricServiceClient.UpdateAgentData(ctx, metric); err != nil {
			slog.Info(
				"send metrics. update metric failed",
				"metric", metric.Name(),
				"error", err,
			)
			continue
		}

		if metric.MType() != entities.Counter {
			continue
		}
		if err := s.storage.AcknowledgeCounter(ctx, metric.Name(), *metric.Delta()); err != nil {
			return errors.Wrap(err, "send metrics. acknowledge counter")
		}
	}

	return nil
}
