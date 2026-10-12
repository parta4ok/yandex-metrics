package config

import (
	"flag"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/basicflag"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

const (
	gracefulShutdownTimeoutKey      = "metrics.application.graceful_shutdown_timeout"
	publicHTTPAddressKey            = "metrics.http.public.address"
	publicHTTPTLSEnabledKey         = "metrics.http.public.tls.enabled"
	publicHTTPTLSCertificateFileKey = "metrics.http.public.tls.certificate_file"
	publicHTTPTLSKeyFileKey         = "metrics.http.public.tls.key_file"

	addressFlag = "a"
	addressEnv  = "ADDRESS"
)

type Config struct {
	*koanf.Koanf
	logger toolkitlogger.Logger
}

type options struct {
	flags   *flag.FlagSet
	environ []string
}

type Option func(*options)

func WithFlags(flags *flag.FlagSet) Option {
	return func(opts *options) {
		opts.flags = flags
	}
}

func WithEnviron(environ []string) Option {
	return func(opts *options) {
		opts.environ = environ
	}
}

func NewConfig(filePath string, logger toolkitlogger.Logger, opts ...Option) (*Config, error) {
	if filePath == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new config. file path is empty")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new config. logger is nil")
	}

	config := koanf.New(".")
	if err := config.Load(file.Provider(filePath), yaml.Parser()); err != nil {
		return nil, errors.Wrap(err, "new config. load file")
	}

	var options options
	for _, opt := range opts {
		opt(&options)
	}

	if options.flags != nil {
		flagProvider := basicflag.ProviderWithValue(options.flags, ".", flagToKey, config)
		if err := config.Load(flagProvider, nil); err != nil {
			return nil, errors.Wrap(err, "new config. load flags")
		}
	}

	if options.environ != nil {
		envProvider := env.Provider(".", env.Opt{
			TransformFunc: envToKey,
			EnvironFunc:   func() []string { return options.environ },
		})
		if err := config.Load(envProvider, nil); err != nil {
			return nil, errors.Wrap(err, "new config. load environment")
		}
	}

	return &Config{
		Koanf:  config,
		logger: logger,
	}, nil
}

func (c *Config) HTTPAddress() string {
	return c.String(publicHTTPAddressKey)
}

func (c *Config) GracefulShutdownTimeout() time.Duration {
	return c.Duration(gracefulShutdownTimeoutKey)
}

func (c *Config) TLSEnabled() bool {
	return c.Bool(publicHTTPTLSEnabledKey)
}

func (c *Config) TLSCertificateFile() string {
	return c.String(publicHTTPTLSCertificateFileKey)
}

func (c *Config) TLSKeyFile() string {
	return c.String(publicHTTPTLSKeyFileKey)
}

func flagToKey(name, value string) (string, any) {
	if name == addressFlag {
		return publicHTTPAddressKey, value
	}

	return "", nil
}

func envToKey(name, value string) (string, any) {
	if strings.EqualFold(name, addressEnv) {
		return publicHTTPAddressKey, value
	}

	return "", nil
}
