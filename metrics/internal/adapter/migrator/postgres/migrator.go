package postgres

import (
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/deployment/migrations"
	toolkitmigrator "github.com/parta4ok/yandex-metrics/toolkit/migrator"
	"github.com/parta4ok/yandex-metrics/toolkit/migrator/gomigrate"
)

const migrationsDir = "postgres"

func NewMigrator(connStr string) (toolkitmigrator.Migrator, error) {
	migrator, err := gomigrate.New(connStr, migrations.FS, migrationsDir)
	if err != nil {
		return nil, errors.Wrap(err, "new postgres migrator")
	}

	return migrator, nil
}
