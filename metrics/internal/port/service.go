package port

import (
	"context"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

//go:generate mockgen -source=service.go -destination=./testdata/service.go -package=testdata

type MetricServiceProvider interface {
	UpdateMetric(ctx context.Context, metric *entities.Metrics) error
}
