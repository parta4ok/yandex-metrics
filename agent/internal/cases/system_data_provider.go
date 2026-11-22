package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

//go:generate mockgen -source=system_data_provider.go -destination=./testdata/system_data_provider.go -package=testdata

type SystemDataProvider interface {
	GetSystemData(ctx context.Context) (*entities.Metrics, error)
}
