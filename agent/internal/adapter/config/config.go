package config

import (
	"flag"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/basicflag"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

const (
	gracefulShutdownTimeoutKey = "agent.application.graceful_shutdown_timeout"
	pollIntervalKey            = "agent.ticker.poll_interval"
	reportIntervalKey          = "agent.ticker.report_interval"
	metricsHTTPAddressKey      = "agent.http.metrics.address"
	metricsHTTPTimeoutKey      = "agent.http.metrics.timeout"
)

var (
	flagKeys = map[string]string{
		"a": metricsHTTPAddressKey,
		"r": reportIntervalKey,
		"p": pollIntervalKey,
	}
	envKeys = map[string]string{
		"ADDRESS":         metricsHTTPAddressKey,
		"REPORT_INTERVAL": reportIntervalKey,
		"POLL_INTERVAL":   pollIntervalKey,
	}
	secondsKeys = map[string]struct{}{
		reportIntervalKey: {},
		pollIntervalKey:   {},
	}
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

	loaded := &Config{
		Koanf:  config,
		logger: logger,
	}
	if err := loaded.validateIntervals(); err != nil {
		return nil, errors.Wrap(err, "new config. validate intervals")
	}

	return loaded, nil
}

func (c *Config) validateIntervals() error {
	for _, key := range []string{pollIntervalKey, reportIntervalKey} {
		if c.Duration(key) <= 0 {
			return errors.Wrapf(entities.ErrInvalidParam, "%s must be a positive duration", key)
		}
	}

	return nil
}

func (c *Config) GracefulShutdownTimeout() time.Duration {
	return c.Duration(gracefulShutdownTimeoutKey)
}

func (c *Config) PollInterval() time.Duration {
	return c.Duration(pollIntervalKey)
}

func (c *Config) ReportInterval() time.Duration {
	return c.Duration(reportIntervalKey)
}

func (c *Config) MetricsHTTPAddress() string {
	return c.String(metricsHTTPAddressKey)
}

func (c *Config) MetricsHTTPTimeout() time.Duration {
	return c.Duration(metricsHTTPTimeoutKey)
}

func flagToKey(name, value string) (string, any) {
	key, ok := flagKeys[name]
	if !ok {
		return "", nil
	}

	return key, convertValue(key, value)
}

func envToKey(name, value string) (string, any) {
	key, ok := envKeys[strings.ToUpper(name)]
	if !ok {
		return "", nil
	}

	return key, convertValue(key, value)
}

func convertValue(key, value string) any {
	if _, ok := secondsKeys[key]; !ok {
		return value
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return time.Duration(0)
	}

	return time.Duration(seconds) * time.Second
}
