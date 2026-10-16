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

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

const (
	gracefulShutdownTimeoutKey      = "metrics.application.graceful_shutdown_timeout"
	publicHTTPAddressKey            = "metrics.http.public.address"
	publicHTTPTLSEnabledKey         = "metrics.http.public.tls.enabled"
	publicHTTPTLSCertificateFileKey = "metrics.http.public.tls.certificate_file"
	publicHTTPTLSKeyFileKey         = "metrics.http.public.tls.key_file"

	storeIntervalKey   = "metrics.backup.store_interval"
	fileStoragePathKey = "metrics.backup.file_storage_path"
	restoreKey         = "metrics.backup.restore"

	defaultStoreInterval   = 300 * time.Second
	defaultFileStoragePath = "metrics-storage.json"
	defaultRestore         = false
)

var (
	flagKeys = map[string]string{
		"a": publicHTTPAddressKey,
		"i": storeIntervalKey,
		"f": fileStoragePathKey,
		"r": restoreKey,
	}
	envKeys = map[string]string{
		"ADDRESS":           publicHTTPAddressKey,
		"STORE_INTERVAL":    storeIntervalKey,
		"FILE_STORAGE_PATH": fileStoragePathKey,
		"RESTORE":           restoreKey,
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
	if err := setBackupDefaults(config); err != nil {
		return nil, errors.Wrap(err, "new config. set defaults")
	}
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
	if err := loaded.validateBackup(); err != nil {
		return nil, errors.Wrap(err, "new config. validate backup")
	}

	return loaded, nil
}

func setBackupDefaults(config *koanf.Koanf) error {
	defaults := map[string]any{
		storeIntervalKey:   defaultStoreInterval,
		fileStoragePathKey: defaultFileStoragePath,
		restoreKey:         defaultRestore,
	}
	for key, value := range defaults {
		if err := config.Set(key, value); err != nil {
			return errors.Wrapf(err, "set default %s", key)
		}
	}

	return nil
}

func (c *Config) validateBackup() error {
	if c.Duration(storeIntervalKey) < 0 {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must not be negative", storeIntervalKey)
	}
	if c.String(fileStoragePathKey) == "" {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must not be empty", fileStoragePathKey)
	}
	if _, ok := c.Get(restoreKey).(bool); !ok {
		return errors.Wrapf(entities.ErrInvalidParam, "%s must be a boolean", restoreKey)
	}

	return nil
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

func (c *Config) StoreInterval() time.Duration {
	return c.Duration(storeIntervalKey)
}

func (c *Config) FileStoragePath() string {
	return c.String(fileStoragePathKey)
}

func (c *Config) Restore() bool {
	return c.Bool(restoreKey)
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
	switch key {
	case storeIntervalKey:
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return -time.Second
		}

		return time.Duration(seconds) * time.Second
	case restoreKey:
		restore, err := strconv.ParseBool(value)
		if err != nil {
			return value
		}

		return restore
	default:
		return value
	}
}
