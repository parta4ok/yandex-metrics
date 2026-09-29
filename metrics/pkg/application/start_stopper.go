package application

import "context"

type StartStopper interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
