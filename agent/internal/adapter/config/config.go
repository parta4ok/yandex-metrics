package config

import (
	"os"
	"strconv"
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

	defaultGracefulShutdownTimeout = 5 * time.Second
	defaultPollInterval            = 2 * time.Second
	defaultReportInterval          = 10 * time.Second
	defaultMetricsHTTPAddress      = "localhost:8080"
	defaultMetricsHTTPTimeout      = 5 * time.Second

	configPathEnv        = "METRIC_CONFIG_PATH"
	defaultConfigPath    = "config/agent.yml"
	defaultConfigExample = "config/agent.example.yml"
)

type Config struct {
	settings *koanf.Koanf
}

func NewConfig(args []string, environ []string) (*Config, error) {
	config := &Config{settings: koanf.New(".")}

	flags := newCLIFlags()
	if err := flags.parse(args); err != nil {
		return nil, errors.Wrap(err, "new config. parse flags")
	}
	env := newEnviron(environ)

	if err := config.setDefaults(); err != nil {
		return nil, errors.Wrap(err, "new config. set defaults")
	}
	if err := config.loadFile(config.resolvePath(flags, env)); err != nil {
		return nil, errors.Wrap(err, "new config. load file")
	}
	if err := config.applyFlags(flags); err != nil {
		return nil, errors.Wrap(err, "new config. apply flags")
	}
	if err := config.applyEnviron(env); err != nil {
		return nil, errors.Wrap(err, "new config. apply environment")
	}
	if err := config.validate(); err != nil {
		return nil, errors.Wrap(err, "new config. validate")
	}

	return config, nil
}

func (c *Config) setDefaults() error {
	defaults := map[string]any{
		gracefulShutdownTimeoutKey: defaultGracefulShutdownTimeout,
		pollIntervalKey:            defaultPollInterval,
		reportIntervalKey:          defaultReportInterval,
		metricsHTTPAddressKey:      defaultMetricsHTTPAddress,
		metricsHTTPTimeoutKey:      defaultMetricsHTTPTimeout,
	}
	for key, value := range defaults {
		if err := c.set(key, value); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) resolvePath(flags *cliFlags, env environ) string {
	if flags.configPath != "" {
		return flags.configPath
	}
	if path, ok := env.lookup(configPathEnv); ok && path != "" {
		return path
	}
	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		return defaultConfigExample
	}

	return defaultConfigPath
}

func (c *Config) loadFile(path string) error {
	if err := c.settings.Load(file.Provider(path), yaml.Parser()); err != nil {
		return errors.Wrapf(err, "load %s", path)
	}

	return nil
}

func (c *Config) applyFlags(flags *cliFlags) error {
	if flags.isSet(addressFlag) {
		if err := c.set(metricsHTTPAddressKey, flags.address); err != nil {
			return err
		}
	}
	if flags.isSet(reportIntervalFlag) {
		if err := c.set(reportIntervalKey, time.Duration(flags.reportInterval)*time.Second); err != nil {
			return err
		}
	}
	if flags.isSet(pollIntervalFlag) {
		if err := c.set(pollIntervalKey, time.Duration(flags.pollInterval)*time.Second); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) applyEnviron(env environ) error {
	if address, ok := env.lookup(addressEnv); ok {
		if err := c.set(metricsHTTPAddressKey, address); err != nil {
			return err
		}
	}
	if value, ok := env.lookup(reportIntervalEnv); ok {
		interval, err := c.parseDuration(value)
		if err != nil {
			return errors.Wrapf(err, "%s", reportIntervalEnv)
		}
		if err := c.set(reportIntervalKey, interval); err != nil {
			return err
		}
	}
	if value, ok := env.lookup(pollIntervalEnv); ok {
		interval, err := c.parseDuration(value)
		if err != nil {
			return errors.Wrapf(err, "%s", pollIntervalEnv)
		}
		if err := c.set(pollIntervalKey, interval); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validate() error {
	for _, key := range []string{pollIntervalKey, reportIntervalKey} {
		if c.settings.Duration(key) <= 0 {
			return errors.Wrapf(entities.ErrInvalidParam, "%s must be a positive duration", key)
		}
	}

	return nil
}

func (c *Config) parseDuration(seconds string) (time.Duration, error) {
	value, err := strconv.Atoi(seconds)
	if err != nil {
		return 0, errors.Wrap(entities.ErrInvalidParam, "parse duration. value is not a whole number of seconds")
	}

	return time.Duration(value) * time.Second, nil
}

func (c *Config) set(key string, value any) error {
	if err := c.settings.Set(key, value); err != nil {
		return errors.Wrapf(err, "set %s", key)
	}

	return nil
}

func (c *Config) GracefulShutdownTimeout() time.Duration {
	return c.settings.Duration(gracefulShutdownTimeoutKey)
}

func (c *Config) PollInterval() time.Duration {
	return c.settings.Duration(pollIntervalKey)
}

func (c *Config) ReportInterval() time.Duration {
	return c.settings.Duration(reportIntervalKey)
}

func (c *Config) MetricsHTTPAddress() string {
	return c.settings.String(metricsHTTPAddressKey)
}

func (c *Config) MetricsHTTPTimeout() time.Duration {
	return c.settings.Duration(metricsHTTPTimeoutKey)
}
