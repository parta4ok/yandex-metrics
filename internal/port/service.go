package port

import (
	"context"

	"github.com/parta4ok/yandex-metrics/internal/entities"
)

type MetricServiceProvider interface {
	UpdateMetric(ctx context.Context, metric *entities.Metrics) error
}
