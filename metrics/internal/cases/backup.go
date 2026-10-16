package cases

import (
	"context"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

//go:generate mockgen -source=backup.go -destination=./testdata/backup.go -package=testdata

type BackupProvider interface {
	MakeBackup(ctx context.Context, metrics []*entities.Metric) error
	RestoreBackup(ctx context.Context) ([]*entities.Metric, error)
}
