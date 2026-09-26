package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/internal/entities"
)

type MetricsStorage interface {
	UpdateMetric(ctx context.Context, metric *entities.Metrics) error
}
