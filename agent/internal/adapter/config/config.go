package config

import (
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

const (
	gracefulShutdownTimeoutKey = "agent.application.graceful_shutdown_timeout"
	pollIntervalKey            = "agent.ticker.poll_interval"
	reportIntervalKey          = "agent.ticker.report_interval"
	metricsHTTPAddressKey      = "agent.http.metrics.address"
	metricsHTTPTimeoutKey      = "agent.http.metrics.timeout"
)

type Config struct {
	*koanf.Koanf
}

func NewConfig(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new config. file path is empty")
	}

	config := koanf.New(".")
	if err := config.Load(file.Provider(filePath), yaml.Parser()); err != nil {
		return nil, errors.Wrap(err, "new config. load file")
	}

	return &Config{
		Koanf: config,
	}, nil
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
