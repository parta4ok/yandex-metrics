package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agent.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
agent:
  application:
    graceful_shutdown_timeout: 5s
  ticker:
    poll_interval: 2s
    report_interval: 10s
  http:
    metrics:
      address: http://localhost:8080
      timeout: 3s
`), 0o600))

	loadedConfig, err := config.NewConfig(path)
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.Equal(t, 2*time.Second, loadedConfig.PollInterval())
	require.Equal(t, 10*time.Second, loadedConfig.ReportInterval())
	require.Equal(t, "http://localhost:8080", loadedConfig.MetricsHTTPAddress())
	require.Equal(t, 3*time.Second, loadedConfig.MetricsHTTPTimeout())
}

func TestNewConfig_InvalidPath(t *testing.T) {
	t.Parallel()

	loadedConfig, err := config.NewConfig("")
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, loadedConfig)

	loadedConfig, err = config.NewConfig(filepath.Join(t.TempDir(), "missing.yml"))
	require.Error(t, err)
	require.Nil(t, loadedConfig)
}
