package postgres

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

type startStopper interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

var (
	_ cases.Pinger = (*Storage)(nil)
	_ startStopper = (*Storage)(nil)
)

type Storage struct {
	pool     *pgxpool.Pool
	done     chan struct{}
	stopOnce sync.Once
}

func NewStorage(connStr string) (*Storage, error) {
	if connStr == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new postgres storage. connection string is empty")
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, errors.Wrapf(entities.ErrInvalidParam, "new postgres storage. create pool: %v", err)
	}

	return &Storage{
		pool: pool,
		done: make(chan struct{}),
	}, nil
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
