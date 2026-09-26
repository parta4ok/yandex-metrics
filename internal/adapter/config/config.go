package config

import (
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/internal/entities"
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

func (c *Config) GetPublicHTTPAddr() string {
	return c.String(publicHTTPAddressKey)
}

func (c *Config) GetGracefulShutdownTimeout() time.Duration {
	return c.Duration(gracefulShutdownTimeoutKey)
}

func (c *Config) IsPublicHTTPTLSEnabled() bool {
	return c.Bool(publicHTTPTLSEnabledKey)
}

func (c *Config) GetPublicHTTPTLSCertificateFile() string {
	return c.String(publicHTTPTLSCertificateFileKey)
}

func (c *Config) GetPublicHTTPTLSKeyFile() string {
	return c.String(publicHTTPTLSKeyFileKey)
}
