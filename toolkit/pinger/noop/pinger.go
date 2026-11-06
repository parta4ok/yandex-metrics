package noop

import (
	"context"

	"github.com/pkg/errors"
)

var ErrNotConfigured = errors.New("pinger is not configured")

type Pinger struct{}

func New() *Pinger {
	return &Pinger{}
}

func (p *Pinger) Ping(_ context.Context) error {
	return ErrNotConfigured
}
