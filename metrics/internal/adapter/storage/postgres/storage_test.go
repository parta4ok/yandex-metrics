package postgres_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	pgmigrator "github.com/parta4ok/yandex-metrics/metrics/internal/adapter/migrator/postgres"
	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/postgres"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/migrator/testdata"
)

const testDSNEnv = "TEST_DATABASE_DSN"

func TestNewStorage_InvalidParams(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage, err := postgres.NewStorage(context.Background(), "", testdata.NewMockMigrator(ctrl))
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, storage)

	storage, err = postgres.NewStorage(context.Background(), "postgres://x", nil)
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, storage)
}

func TestNewStorage_Unavailable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage, err := postgres.NewStorage(
		context.Background(),
		"postgres://u:p@localhost:1/db?sslmode=disable",
		testdata.NewMockMigrator(ctrl),
	)

	require.Error(t, err)
	require.Nil(t, storage)
}

func TestNewStorage_Migrations(t *testing.T) {
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", testDSNEnv)
	}

	errMigrate := errors.New("migrate failed")

	tests := []struct {
		name       string
		migrateErr error
	}{
		{name: "migrations applied"},
		{name: "migrations failed", migrateErr: errMigrate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			migrator := testdata.NewMockMigrator(ctrl)
			migrator.EXPECT().Up(gomock.Any()).Return(tt.migrateErr)

			storage, err := postgres.NewStorage(context.Background(), dsn, migrator)
			if tt.migrateErr != nil {
				require.ErrorIs(t, err, entities.ErrInternalError)
				require.Nil(t, storage)
				return
			}

			require.NoError(t, err)
			require.NoError(t, storage.Ping(context.Background()))
			require.NoError(t, storage.Stop(context.Background()))
		})
	}
}

func newStorage(t *testing.T) *postgres.Storage {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", testDSNEnv)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	dropTables := func() {
		_, err := pool.Exec(ctx, "DROP TABLE IF EXISTS metrics, schema_migrations")
		require.NoError(t, err)
	}
	dropTables()

	migrator, err := pgmigrator.NewMigrator(dsn)
	require.NoError(t, err)

	storage, err := postgres.NewStorage(ctx, dsn, migrator)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = storage.Stop(ctx)
		dropTables()
		pool.Close()
	})

	return storage
}

func newCounter(t *testing.T, id string, delta int64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Counter)
	require.NoError(t, err)
	metric.SetDelta(&delta)

	return metric
}

func newGauge(t *testing.T, id string, value float64) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(id, entities.Gauge)
	require.NoError(t, err)
	metric.SetValue(&value)

	return metric
}

func TestStorage_UpdateMetric(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	_, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 2))
	require.NoError(t, err)
	counter, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 3))
	require.NoError(t, err)
	require.Equal(t, int64(5), *counter.Delta())
	require.Nil(t, counter.Value())

	_, err = storage.UpdateMetric(ctx, newGauge(t, "temperature", 1.5))
	require.NoError(t, err)
	gauge, err := storage.UpdateMetric(ctx, newGauge(t, "temperature", 3.5))
	require.NoError(t, err)
	require.Equal(t, 3.5, *gauge.Value())
	require.Nil(t, gauge.Delta())
}

func TestStorage_UpdateMetric_Hash(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	hash := "abc"
	metric := newGauge(t, "temperature", 1.5)
	metric.SetHash(&hash)

	stored, err := storage.UpdateMetric(ctx, metric)
	require.NoError(t, err)
	require.Equal(t, "abc", *stored.Hash())
}

func TestStorage_GetMetric(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	_, err := storage.UpdateMetric(ctx, newGauge(t, "temperature", 1.5))
	require.NoError(t, err)

	metric, err := storage.GetMetric(ctx, "temperature", entities.Gauge)
	require.NoError(t, err)
	require.Equal(t, 1.5, *metric.Value())

	_, err = storage.GetMetric(ctx, "temperature", entities.Counter)
	require.ErrorIs(t, err, entities.ErrNotFound)

	_, err = storage.GetMetric(ctx, "missing", entities.Gauge)
	require.ErrorIs(t, err, entities.ErrNotFound)
}

func TestStorage_ListMetrics(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	metrics, err := storage.ListMetrics(ctx)
	require.NoError(t, err)
	require.Empty(t, metrics)

	for _, metric := range []*entities.Metric{
		newGauge(t, "b", 1),
		newCounter(t, "b", 2),
		newGauge(t, "a", 3),
	} {
		_, err := storage.UpdateMetric(ctx, metric)
		require.NoError(t, err)
	}

	metrics, err = storage.ListMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, metrics, 3)
	require.Equal(t, "a", metrics[0].ID())
	require.Equal(t, entities.Counter, metrics[1].MType())
	require.Equal(t, entities.Gauge, metrics[2].MType())
}

func TestStorage_AccumulatesCounterConcurrently(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	const workers = 50

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 1))
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	metric, err := storage.GetMetric(ctx, "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(workers), *metric.Delta())
}

func TestStorage_CancelledContext(t *testing.T) {
	storage := newStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 1))
	require.ErrorIs(t, err, entities.ErrInternalError)
	_, err = storage.GetMetric(ctx, "requests", entities.Counter)
	require.ErrorIs(t, err, entities.ErrInternalError)
	_, err = storage.ListMetrics(ctx)
	require.ErrorIs(t, err, entities.ErrInternalError)
}

func TestStorage_UpdateMetrics(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	_, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 2))
	require.NoError(t, err)

	stored, err := storage.UpdateMetrics(ctx, []*entities.Metric{
		newCounter(t, "requests", 3),
		newGauge(t, "temperature", 3.5),
	})
	require.NoError(t, err)
	require.Len(t, stored, 2)
	require.Equal(t, int64(5), *stored[0].Delta())
	require.Equal(t, 3.5, *stored[1].Value())

	metrics, err := storage.ListMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, metrics, 2)
}

func TestStorage_UpdateMetrics_RollsBackOnError(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	invalid, err := entities.NewMetric("broken", entities.Gauge)
	require.NoError(t, err)

	_, err = storage.UpdateMetrics(ctx, []*entities.Metric{
		newCounter(t, "requests", 1),
		invalid,
	})
	require.ErrorIs(t, err, entities.ErrInternalError)
	require.NotErrorIs(t, err, entities.ErrTemporary)

	metrics, err := storage.ListMetrics(ctx)
	require.NoError(t, err)
	require.Empty(t, metrics)
}

func TestStorage_UpdateMetrics_AccumulatesConcurrently(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	const workers = 30

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := storage.UpdateMetrics(ctx, []*entities.Metric{
				newCounter(t, "errors", 2),
				newCounter(t, "requests", 1),
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	requests, err := storage.GetMetric(ctx, "requests", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(workers), *requests.Delta())

	errorsMetric, err := storage.GetMetric(ctx, "errors", entities.Counter)
	require.NoError(t, err)
	require.Equal(t, int64(2*workers), *errorsMetric.Delta())
}

type tcpProxy struct {
	listener net.Listener
	mu       sync.Mutex
	conns    []net.Conn
}

func newTCPProxy(t *testing.T, target string) *tcpProxy {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxy := &tcpProxy{listener: listener}
	t.Cleanup(proxy.Close)

	go proxy.serve(target)

	return proxy
}

func (p *tcpProxy) Addr() string {
	return p.listener.Addr().String()
}

func (p *tcpProxy) Close() {
	_ = p.listener.Close()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.conns {
		_ = conn.Close()
	}
}

func (p *tcpProxy) serve(target string) {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}

		server, err := net.Dial("tcp", target)
		if err != nil {
			_ = client.Close()
			continue
		}

		p.mu.Lock()
		p.conns = append(p.conns, client, server)
		p.mu.Unlock()

		go func() { _, _ = io.Copy(server, client) }()
		go func() { _, _ = io.Copy(client, server) }()
	}
}

func TestStorage_TemporaryErrorWhenDatabaseIsDown(t *testing.T) {
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", testDSNEnv)
	}

	ctx := context.Background()
	real, err := url.Parse(dsn)
	require.NoError(t, err)

	newStorageThroughProxy := func(t *testing.T) (*postgres.Storage, *tcpProxy) {
		t.Helper()

		_ = newStorage(t)

		proxy := newTCPProxy(t, real.Host)
		proxied := *real
		proxied.Host = proxy.Addr()

		migrator, err := pgmigrator.NewMigrator(proxied.String())
		require.NoError(t, err)
		storage, err := postgres.NewStorage(ctx, proxied.String(), migrator)
		require.NoError(t, err)
		t.Cleanup(func() { _ = storage.Stop(ctx) })

		return storage, proxy
	}

	tests := []struct {
		name string
		call func(storage *postgres.Storage) error
	}{
		{
			name: "update metric",
			call: func(storage *postgres.Storage) error {
				_, err := storage.UpdateMetric(ctx, newCounter(t, "requests", 1))
				return err
			},
		},
		{
			name: "update metrics",
			call: func(storage *postgres.Storage) error {
				_, err := storage.UpdateMetrics(ctx, []*entities.Metric{newCounter(t, "requests", 1)})
				return err
			},
		},
		{
			name: "get metric",
			call: func(storage *postgres.Storage) error {
				_, err := storage.GetMetric(ctx, "requests", entities.Counter)
				return err
			},
		},
		{
			name: "list metrics",
			call: func(storage *postgres.Storage) error {
				_, err := storage.ListMetrics(ctx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, proxy := newStorageThroughProxy(t)
			proxy.Close()

			err := tt.call(storage)

			require.ErrorIs(t, err, entities.ErrTemporary, "got: %v", err)
		})
	}
}

func TestStorage_PingKeepsInternalErrorWhenDatabaseIsDown(t *testing.T) {
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", testDSNEnv)
	}

	ctx := context.Background()
	real, err := url.Parse(dsn)
	require.NoError(t, err)

	_ = newStorage(t)

	proxy := newTCPProxy(t, real.Host)
	proxied := *real
	proxied.Host = proxy.Addr()

	migrator, err := pgmigrator.NewMigrator(proxied.String())
	require.NoError(t, err)
	storage, err := postgres.NewStorage(ctx, proxied.String(), migrator)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Stop(ctx) })

	proxy.Close()

	err = storage.Ping(ctx)

	require.ErrorIs(t, err, entities.ErrInternalError)
	require.NotErrorIs(t, err, entities.ErrTemporary)
}
