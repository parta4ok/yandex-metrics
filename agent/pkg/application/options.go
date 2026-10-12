package application

import (
	"flag"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/config"
)

type Option = config.Option

func WithFlags(flags *flag.FlagSet) Option {
	return config.WithFlags(flags)
}

func WithEnviron(environ []string) Option {
	return config.WithEnviron(environ)
}
