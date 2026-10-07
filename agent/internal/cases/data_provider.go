package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

//go:generate mockgen -source=data_provider.go -destination=./testdata/data_provider.go -package=testdata

type DataProvider interface {
	GetActualAgentData(ctx context.Context) (*entities.Metrics, error)
}
