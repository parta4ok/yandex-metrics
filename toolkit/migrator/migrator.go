package migrator

import (
	"context"

	"github.com/pkg/errors"
)

var (
	ErrInvalidParam = errors.New("invalid param")
	ErrInternal     = errors.New("internal error")
)

//go:generate mockgen -source=migrator.go -destination=./testdata/migrator.go -package=testdata

type Migrator interface {
	Up(ctx context.Context) error
}
