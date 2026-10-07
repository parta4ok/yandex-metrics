package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "metrics.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
metrics:
  application:
    graceful_shutdown_timeout: 5s
  http:
    public:
      address: :8080
      tls:
        enabled: true
        certificate_file: cert.pem
        key_file: key.pem
`), 0o600))

	loadedConfig, err := config.NewConfig(path, noop.New())
	require.NoError(t, err)
	require.Equal(t, ":8080", loadedConfig.HTTPAddress())
	require.Equal(t, 5*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.True(t, loadedConfig.TLSEnabled())
	require.Equal(t, "cert.pem", loadedConfig.TLSCertificateFile())
	require.Equal(t, "key.pem", loadedConfig.TLSKeyFile())
}

func TestNewConfig_InvalidPath(t *testing.T) {
	t.Parallel()

	loadedConfig, err := config.NewConfig("", noop.New())
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, loadedConfig)

	loadedConfig, err = config.NewConfig(filepath.Join(t.TempDir(), "missing.yml"), noop.New())
	require.Error(t, err)
	require.Nil(t, loadedConfig)
}
