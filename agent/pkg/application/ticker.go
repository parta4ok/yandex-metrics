package application

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

type Ticker struct {
	interval time.Duration
	action   func(context.Context) error
}

func NewTicker(interval time.Duration, action func(context.Context) error) (*Ticker, error) {
	if interval <= 0 {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new ticker. interval is not positive")
	}
	if action == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new ticker. action is nil")
	}

	return &Ticker{
		interval: interval,
		action:   action,
	}, nil
}

func (t *Ticker) Start(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := t.action(ctx); err != nil {
				return errors.Wrap(err, "start ticker. execute action")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *Ticker) Stop(_ context.Context) error {
	return nil
}
