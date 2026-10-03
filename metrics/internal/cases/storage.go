package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

//go:generate mockgen -source=storage.go -destination=./testdata/storage.go -package=testdata

type MetricsStorage interface {
	UpdateMetric(ctx context.Context, metric *entities.Metrics) error
	GetMetric(ctx context.Context, id string, mType entities.MType) (*entities.Metrics, error)
	ListMetrics(ctx context.Context) ([]*entities.Metrics, error)
}
