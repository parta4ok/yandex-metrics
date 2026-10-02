package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

//go:generate mockgen -source=metric_service_client.go -destination=./testdata/metric_service_client.go -package=testdata

type MetricServiceClient interface {
	UpdateAgentData(ctx context.Context, metric *entities.Metric) error
}
