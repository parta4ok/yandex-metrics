package cases

import (
	"context"

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

	metrics := entities.NewMetrics()
	storedMetrics, err := s.storage.GetAgentData(ctx)
	if err != nil && !errors.Is(err, entities.ErrNotFound) {
		return errors.Wrap(err, "update metrics. get agent data")
	}
	if err == nil {
		metrics = storedMetrics
	}

	if err := s.updateMetrics(metrics, actualMetrics); err != nil {
		return errors.Wrap(err, "update metrics. update agent data")
	}

	if err := s.storage.SaveAgentData(ctx, metrics); err != nil {
		return errors.Wrap(err, "update metrics. save agent data")
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

func (s *AgentService) updateMetrics(metrics *entities.Metrics, actualMetrics *entities.Metrics) error {
	actualMetricList, err := actualMetrics.All()
	if err != nil {
		return errors.Wrap(err, "update metrics. list actual metrics")
	}

	for _, actualMetric := range actualMetricList {
		if actualMetric.MType() != entities.Gauge {
			return errors.Wrap(entities.ErrInvalidParam, "update metrics. actual metric type is not gauge")
		}
		if err := metrics.UpdateGauge(actualMetric.Name(), *actualMetric.Value()); err != nil {
			return errors.Wrap(err, "update metrics. update gauge")
		}
	}

	if err := metrics.IncrementCounter(entities.PollCount, 1); err != nil {
		return errors.Wrap(err, "update metrics. increment poll count")
	}

	return nil
}

func (s *AgentService) sendMetrics(ctx context.Context, metrics *entities.Metrics) error {
	metricList, err := metrics.All()
	if err != nil {
		return errors.Wrap(err, "send metrics. list metrics")
	}

	for _, metric := range metricList {
		_ = s.metricServiceClient.UpdateAgentData(ctx, metric)
	}

	return nil
}
