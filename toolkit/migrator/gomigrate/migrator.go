package gomigrate

import (
	"context"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pkg/errors"

	toolkitmigrator "github.com/parta4ok/yandex-metrics/toolkit/migrator"
)

const (
	pgxScheme = "pgx5://"
)

var _ toolkitmigrator.Migrator = (*Migrator)(nil)

type Migrator struct {
	connStr    string
	migrations fs.FS
	dir        string
}

func New(connStr string, migrations fs.FS, dir string) (*Migrator, error) {
	if connStr == "" {
		return nil, errors.Wrap(toolkitmigrator.ErrInvalidParam, "new migrator. connection string is empty")
	}
	if migrations == nil {
		return nil, errors.Wrap(toolkitmigrator.ErrInvalidParam, "new migrator. migrations are nil")
	}
	if dir == "" {
		return nil, errors.Wrap(toolkitmigrator.ErrInvalidParam, "new migrator. migrations directory is empty")
	}

	return &Migrator{
		connStr:    connStr,
		migrations: migrations,
		dir:        dir,
	}, nil
}

func (m *Migrator) Up(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "migrate up. context")
	}

	source, err := iofs.New(m.migrations, m.dir)
	if err != nil {
		return errors.Wrapf(toolkitmigrator.ErrInternal, "migrate up. open source: %v", err)
	}

	migration, err := migrate.NewWithSourceInstance("iofs", source, m.databaseURL())
	if err != nil {
		return errors.Wrapf(toolkitmigrator.ErrInternal, "migrate up. create migration: %v", err)
	}
	defer migration.Close()

	if err := migration.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.Wrapf(toolkitmigrator.ErrInternal, "migrate up. apply: %v", err)
	}

	return nil
}

func (m *Migrator) databaseURL() string {
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(m.connStr, scheme) {
			return pgxScheme + strings.TrimPrefix(m.connStr, scheme)
		}
	}

	return m.connStr
}
