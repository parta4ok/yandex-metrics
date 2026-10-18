package config

import (
	"os"
	"strconv"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

const (
	gracefulShutdownTimeoutKey      = "metrics.application.graceful_shutdown_timeout"
	publicHTTPAddressKey            = "metrics.http.public.address"
	publicHTTPTLSEnabledKey         = "metrics.http.public.tls.enabled"
	publicHTTPTLSCertificateFileKey = "metrics.http.public.tls.certificate_file"
	publicHTTPTLSKeyFileKey         = "metrics.http.public.tls.key_file"
	storeIntervalKey                = "metrics.backup.store_interval"
	fileStoragePathKey              = "metrics.backup.file_storage_path"
	restoreKey                      = "metrics.backup.restore"

	defaultGracefulShutdownTimeout = 5 * time.Second
	defaultPublicHTTPAddress       = "localhost:8080"
	defaultTLSEnabled              = false
	defaultStoreInterval           = 300 * time.Second
	defaultFileStoragePath         = "metrics-storage.json"
	defaultRestore                 = false

	configPathEnv        = "METRIC_CONFIG_PATH"
	defaultConfigPath    = "config/metrics.yml"
	defaultConfigExample = "config/metrics.example.yml"
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
		publicHTTPAddressKey:       defaultPublicHTTPAddress,
		publicHTTPTLSEnabledKey:    defaultTLSEnabled,
		storeIntervalKey:           defaultStoreInterval,
		fileStoragePathKey:         defaultFileStoragePath,
		restoreKey:                 defaultRestore,
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
		if err := c.set(publicHTTPAddressKey, flags.address); err != nil {
			return err
		}
	}
	if flags.isSet(storeIntervalFlag) {
		if err := c.set(storeIntervalKey, time.Duration(flags.storeInterval)*time.Second); err != nil {
			return err
		}
	}
	if flags.isSet(fileStoragePathFlag) {
		if err := c.set(fileStoragePathKey, flags.fileStoragePath); err != nil {
			return err
		}
	}
	if flags.isSet(restoreFlag) {
		if err := c.set(restoreKey, flags.restore); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) applyEnviron(env environ) error {
	if address, ok := env.lookup(addressEnv); ok {
		if err := c.set(publicHTTPAddressKey, address); err != nil {
			return err
		}
	}
	if value, ok := env.lookup(storeIntervalEnv); ok {
		interval, err := c.parseDuration(value)
		if err != nil {
			return errors.Wrapf(err, "%s", storeIntervalEnv)
		}
		if err := c.set(storeIntervalKey, interval); err != nil {
			return err
		}
	}
	if path, ok := env.lookup(fileStoragePathEnv); ok {
		if err := c.set(fileStoragePathKey, path); err != nil {
			return err
		}
	}
	if value, ok := env.lookup(restoreEnv); ok {
		restore, err := strconv.ParseBool(value)
		if err != nil {
			return errors.Wrapf(entities.ErrInvalidParam, "%s must be true or false", restoreEnv)
		}
		if err := c.set(restoreKey, restore); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validate() error {
	if c.settings.Duration(storeIntervalKey) < 0 {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must not be negative", storeIntervalKey)
	}
	if c.settings.String(fileStoragePathKey) == "" {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must not be empty", fileStoragePathKey)
	}
	if _, ok := c.settings.Get(restoreKey).(bool); !ok {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must be a boolean", restoreKey)
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

func (c *Config) HTTPAddress() string {
	return c.settings.String(publicHTTPAddressKey)
}

func (c *Config) GracefulShutdownTimeout() time.Duration {
	return c.settings.Duration(gracefulShutdownTimeoutKey)
}

func (c *Config) TLSEnabled() bool {
	return c.settings.Bool(publicHTTPTLSEnabledKey)
}

func (c *Config) TLSCertificateFile() string {
	return c.settings.String(publicHTTPTLSCertificateFileKey)
}

func (c *Config) TLSKeyFile() string {
	return c.settings.String(publicHTTPTLSKeyFileKey)
}

func (c *Config) StoreInterval() time.Duration {
	return c.settings.Duration(storeIntervalKey)
}

func (c *Config) FileStoragePath() string {
	return c.settings.String(fileStoragePathKey)
}

func (c *Config) Restore() bool {
	return c.settings.Bool(restoreKey)
}
