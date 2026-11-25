package cases

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

const sendChunkSize = 10

type AgentService struct {
	metricServiceClient MetricServiceClient
	dataProvider        DataProvider
	systemDataProvider  SystemDataProvider
	storage             Storage
	rateLimit           int
	logger              toolkitlogger.Logger
}

type sendResult struct {
	metrics []*entities.Metric
	err     error
}

func NewAgentService(
	metricServiceClient MetricServiceClient,
	dataProvider DataProvider,
	systemDataProvider SystemDataProvider,
	storage Storage,
	rateLimit int,
	logger toolkitlogger.Logger,
) (*AgentService, error) {
	if metricServiceClient == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. metric service client is nil")
	}
	if dataProvider == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. data provider is nil")
	}
	if systemDataProvider == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. system data provider is nil")
	}
	if storage == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. storage is nil")
	}
	if rateLimit <= 0 {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. rate limit is not positive")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new agent service. logger is nil")
	}

	return &AgentService{
		metricServiceClient: metricServiceClient,
		dataProvider:        dataProvider,
		systemDataProvider:  systemDataProvider,
		storage:             storage,
		rateLimit:           rateLimit,
		logger:              logger,
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

func (s *AgentService) UpdateSystemMetrics(ctx context.Context) error {
	systemMetrics, err := s.systemDataProvider.GetSystemData(ctx)
	if err != nil {
		s.logger.Warn("update system metrics. get system data failed", "error", err)

		return nil
	}

	if err := s.updateMetrics(systemMetrics); err != nil {
		return errors.Wrap(err, "update system metrics. validate system data")
	}
	if err := s.storage.UpdateGauges(ctx, systemMetrics); err != nil {
		return errors.Wrap(err, "update system metrics. update gauges")
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
		if errors.Is(err, entities.ErrEmptyBatch) {
			return nil
		}

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
	if len(metricList) == 0 {
		return errors.Wrap(entities.ErrEmptyBatch, "send metrics. nothing to send")
	}

	chunks := s.splitMetrics(metricList)
	jobs := make(chan []*entities.Metric)
	results := make(chan sendResult, len(chunks))

	var wg sync.WaitGroup
	for range min(s.rateLimit, len(chunks)) {
		wg.Add(1)
		go s.sendWorker(ctx, &wg, jobs, results)
	}

enqueue:
	for _, chunk := range chunks {
		select {
		case jobs <- chunk:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			s.logger.Warn("send metrics. update batch failed", "error", result.err)
			continue
		}
		if err := s.acknowledgeCounters(ctx, result.metrics); err != nil {
			return errors.Wrap(err, "send metrics. acknowledge counters")
		}
	}

	return nil
}

func (s *AgentService) sendWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	jobs <-chan []*entities.Metric,
	results chan<- sendResult,
) {
	defer wg.Done()

	for chunk := range jobs {
		results <- sendResult{
			metrics: chunk,
			err:     s.metricServiceClient.UpdateAgentDataBatch(ctx, chunk),
		}
	}
}

func (s *AgentService) splitMetrics(metrics []*entities.Metric) [][]*entities.Metric {
	chunks := make([][]*entities.Metric, 0, (len(metrics)+sendChunkSize-1)/sendChunkSize)
	for start := 0; start < len(metrics); start += sendChunkSize {
		chunks = append(chunks, metrics[start:min(start+sendChunkSize, len(metrics))])
	}

	return chunks
}

func (s *AgentService) acknowledgeCounters(ctx context.Context, metrics []*entities.Metric) error {
	for _, metric := range metrics {
		if metric.MType() != entities.Counter {
			continue
		}
		if err := s.storage.AcknowledgeCounter(ctx, metric.Name(), *metric.Delta()); err != nil {
			return err
		}
	}

	return nil
}
