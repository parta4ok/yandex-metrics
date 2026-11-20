package gomigrate_test

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/migrator"
	"github.com/parta4ok/yandex-metrics/toolkit/migrator/gomigrate"
)

const testDSNEnv = "TEST_DATABASE_DSN"

func newMigrations() fstest.MapFS {
	return fstest.MapFS{
		"sql/1_init.up.sql":   {Data: []byte("BEGIN;\nCREATE TABLE migrator_test (id TEXT PRIMARY KEY);\nEND;\n")},
		"sql/1_init.down.sql": {Data: []byte("BEGIN;\nDROP TABLE IF EXISTS migrator_test;\nEND;\n")},
	}
}

func TestNew_InvalidParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		connStr    string
		migrations fs.FS
		dir        string
	}{
		{name: "empty connection string", migrations: newMigrations(), dir: "sql"},
		{name: "nil migrations", connStr: "postgres://x", dir: "sql"},
		{name: "empty directory", connStr: "postgres://x", migrations: newMigrations()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := gomigrate.New(tt.connStr, tt.migrations, tt.dir)

			require.ErrorIs(t, err, migrator.ErrInvalidParam)
			require.Nil(t, result)
		})
	}
}

func TestMigrator_Up(t *testing.T) {
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", testDSNEnv)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS migrator_test, schema_migrations")
		_ = conn.Close(ctx)
	})

	migrations, err := gomigrate.New(dsn, newMigrations(), "sql")
	require.NoError(t, err)

	require.NoError(t, migrations.Up(ctx))
	require.NoError(t, migrations.Up(ctx))

	var exists bool
	require.NoError(t, conn.QueryRow(ctx, "SELECT to_regclass('migrator_test') IS NOT NULL").Scan(&exists))
	require.True(t, exists)
}

func TestMigrator_Up_Unavailable(t *testing.T) {
	t.Parallel()

	migrations, err := gomigrate.New("postgres://u:p@localhost:1/db?sslmode=disable", newMigrations(), "sql")
	require.NoError(t, err)

	require.ErrorIs(t, migrations.Up(context.Background()), migrator.ErrInternal)
}
