package cases

import "context"

//go:generate mockgen -source=pinger.go -destination=./testdata/pinger.go -package=testdata

type Pinger interface {
	Ping(ctx context.Context) error
}
