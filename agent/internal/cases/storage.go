package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

//go:generate mockgen -source=storage.go -destination=./testdata/storage.go -package=testdata

type Storage interface {
	GetAgentData(ctx context.Context) (*entities.Metrics, error)
	UpdateGauges(ctx context.Context, metrics *entities.Metrics) error
	IncrementCounter(ctx context.Context, name entities.MName, delta int64) error
	AcknowledgeCounter(ctx context.Context, name entities.MName, delta int64) error
}
