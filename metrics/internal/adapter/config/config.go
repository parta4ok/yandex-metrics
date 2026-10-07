package config

import (
	"time"

	"github.com/knadh/koanf/parsers/yaml"
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
)

type Config struct {
	*koanf.Koanf
	logger toolkitlogger.Logger
}

func NewConfig(filePath string, logger toolkitlogger.Logger) (*Config, error) {
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
