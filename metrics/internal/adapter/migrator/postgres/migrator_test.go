package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	pgmigrator "github.com/parta4ok/yandex-metrics/metrics/internal/adapter/migrator/postgres"
	toolkitmigrator "github.com/parta4ok/yandex-metrics/toolkit/migrator"
)

const testDSNEnv = "TEST_DATABASE_DSN"

func TestNewMigrator_EmptyConnStr(t *testing.T) {
	t.Parallel()

	migrator, err := pgmigrator.NewMigrator("")

	require.ErrorIs(t, err, toolkitmigrator.ErrInvalidParam)
	require.Nil(t, migrator)
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
		_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS metrics, schema_migrations")
		_ = conn.Close(ctx)
	})

	migrator, err := pgmigrator.NewMigrator(dsn)
	require.NoError(t, err)

	require.NoError(t, migrator.Up(ctx))
	require.NoError(t, migrator.Up(ctx))

	_, err = conn.Exec(ctx, "INSERT INTO metrics (id, mtype, delta) VALUES ('c', 'counter', 1)")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO metrics (id, mtype, value) VALUES ('g', 'gauge', 1.5)")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO metrics (id, mtype, delta, value) VALUES ('bad', 'counter', 1, 1.5)")
	require.Error(t, err)
}
