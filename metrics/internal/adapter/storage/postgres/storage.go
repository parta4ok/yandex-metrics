package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitmigrator "github.com/parta4ok/yandex-metrics/toolkit/migrator"
)

const (
	startupTimeout = 10 * time.Second

	selectColumns = "id, mtype, delta, value, hash"

	upsertCounterQuery = `
INSERT INTO metrics (id, mtype, delta, hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id, mtype) DO UPDATE
SET delta = metrics.delta + EXCLUDED.delta, hash = EXCLUDED.hash
RETURNING ` + selectColumns

	upsertGaugeQuery = `
INSERT INTO metrics (id, mtype, value, hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id, mtype) DO UPDATE
SET value = EXCLUDED.value, hash = EXCLUDED.hash
RETURNING ` + selectColumns

	getMetricQuery = "SELECT " + selectColumns + " FROM metrics WHERE id = $1 AND mtype = $2"

	listMetricsQuery = "SELECT " + selectColumns + " FROM metrics ORDER BY id, mtype"
)

type startStopper interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

var (
	_ cases.Pinger         = (*Storage)(nil)
	_ cases.MetricsStorage = (*Storage)(nil)
	_ startStopper         = (*Storage)(nil)
)

type Storage struct {
	pool     *pgxpool.Pool
	done     chan struct{}
	stopOnce sync.Once
}

func NewStorage(ctx context.Context, connStr string, migrator toolkitmigrator.Migrator) (*Storage, error) {
	if connStr == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new postgres storage. connection string is empty")
	}
	if migrator == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new postgres storage. migrator is nil")
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, errors.Wrapf(entities.ErrInvalidParam, "new postgres storage. create pool: %v", err)
	}

	storage := &Storage{
		pool: pool,
		done: make(chan struct{}),
	}

	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	if err := storage.Ping(startupCtx); err != nil {
		pool.Close()

		return nil, errors.Wrap(err, "new postgres storage. check connection")
	}
	if err := migrator.Up(startupCtx); err != nil {
		pool.Close()

		return nil, errors.Wrap(err, "new postgres storage. apply migrations")
	}

	return storage, nil
}

func (s *Storage) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return errors.Wrap(err, "ping postgres")
	}

	return nil
}

func (s *Storage) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-s.done:
	}

	return nil
}

func (s *Storage) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() {
		close(s.done)
	})

	closed := make(chan struct{})
	go func() {
		s.pool.Close()
		close(closed)
	}()

	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "stop postgres storage. close pool")
	}
}

func (s *Storage) UpdateMetric(ctx context.Context, metric *entities.Metric) (*entities.Metric, error) {
	var row pgx.Row

	switch metric.MType() {
	case entities.Counter:
		row = s.pool.QueryRow(ctx, upsertCounterQuery, metric.ID(), string(metric.MType()), metric.Delta(), metric.Hash())
	case entities.Gauge:
		row = s.pool.QueryRow(ctx, upsertGaugeQuery, metric.ID(), string(metric.MType()), metric.Value(), metric.Hash())
	default:
		return nil, errors.Wrap(entities.ErrInvalidParam, "update metric. metric type is invalid")
	}

	stored, err := s.scanMetric(row)
	if err != nil {
		return nil, errors.Wrap(err, "update metric. upsert")
	}

	return stored, nil
}

func (s *Storage) GetMetric(ctx context.Context, id string, mType entities.MType) (*entities.Metric, error) {
	metric, err := s.scanMetric(s.pool.QueryRow(ctx, getMetricQuery, id, string(mType)))
	if err != nil {
		return nil, errors.Wrap(err, "get metric. select")
	}

	return metric, nil
}

func (s *Storage) ListMetrics(ctx context.Context) ([]*entities.Metric, error) {
	rows, err := s.pool.Query(ctx, listMetricsQuery)
	if err != nil {
		return nil, errors.Wrap(err, "list metrics. select")
	}
	defer rows.Close()

	metrics := make([]*entities.Metric, 0)
	for rows.Next() {
		metric, err := s.scanMetric(rows)
		if err != nil {
			return nil, errors.Wrap(err, "list metrics. scan")
		}

		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "list metrics. rows")
	}

	return metrics, nil
}

func (s *Storage) scanMetric(row pgx.Row) (*entities.Metric, error) {
	var (
		id    string
		mType string
		delta *int64
		value *float64
		hash  *string
	)

	if err := row.Scan(&id, &mType, &delta, &value, &hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.Wrap(entities.ErrNotFound, "scan metric. metric not found")
		}

		return nil, errors.Wrap(err, "scan metric")
	}

	metric, err := entities.NewMetric(id, entities.MType(mType))
	if err != nil {
		return nil, errors.Wrap(err, "scan metric. create metric")
	}
	metric.SetDelta(delta)
	metric.SetValue(value)
	metric.SetHash(hash)

	return metric, nil
}
