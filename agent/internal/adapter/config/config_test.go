package config_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
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

	loadedConfig, err := config.NewConfig(path, noop.New())
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.Equal(t, 2*time.Second, loadedConfig.PollInterval())
	require.Equal(t, 10*time.Second, loadedConfig.ReportInterval())
	require.Equal(t, "http://localhost:8080", loadedConfig.MetricsHTTPAddress())
	require.Equal(t, 3*time.Second, loadedConfig.MetricsHTTPTimeout())
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

func TestNewConfig_Priority(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agent.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
agent:
  ticker:
    poll_interval: 2s
    report_interval: 10s
  http:
    metrics:
      address: file:8080
`), 0o600))

	newFlags := func(args ...string) *flag.FlagSet {
		flags := flag.NewFlagSet("test", flag.ContinueOnError)
		flags.String("config", "", "")
		flags.String("a", "", "")
		flags.Int("r", 0, "")
		flags.Int("p", 0, "")
		require.NoError(t, flags.Parse(args))

		return flags
	}

	tests := []struct {
		name       string
		flags      *flag.FlagSet
		environ    []string
		wantAddr   string
		wantPoll   time.Duration
		wantReport time.Duration
	}{
		{name: "file only", wantAddr: "file:8080", wantPoll: 2 * time.Second, wantReport: 10 * time.Second},
		{
			name:     "unset flags keep file values",
			flags:    newFlags(),
			wantAddr: "file:8080", wantPoll: 2 * time.Second, wantReport: 10 * time.Second,
		},
		{
			name:     "flags over file",
			flags:    newFlags("-a", "flag:8081", "-r", "20", "-p", "3"),
			wantAddr: "flag:8081", wantPoll: 3 * time.Second, wantReport: 20 * time.Second,
		},
		{
			name:       "environment over flags",
			flags:      newFlags("-a", "flag:8081", "-r", "20", "-p", "3"),
			environ:    []string{"ADDRESS=env:8082", "REPORT_INTERVAL=30", "POLL_INTERVAL=4", "OTHER=1"},
			wantAddr:   "env:8082",
			wantPoll:   4 * time.Second,
			wantReport: 30 * time.Second,
		},
		{
			name:     "partial environment",
			flags:    newFlags("-r", "20"),
			environ:  []string{"POLL_INTERVAL=4"},
			wantAddr: "file:8080", wantPoll: 4 * time.Second, wantReport: 20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []config.Option
			if tt.flags != nil {
				opts = append(opts, config.WithFlags(tt.flags))
			}
			if tt.environ != nil {
				opts = append(opts, config.WithEnviron(tt.environ))
			}

			loadedConfig, err := config.NewConfig(path, noop.New(), opts...)
			require.NoError(t, err)
			require.Equal(t, tt.wantAddr, loadedConfig.MetricsHTTPAddress())
			require.Equal(t, tt.wantPoll, loadedConfig.PollInterval())
			require.Equal(t, tt.wantReport, loadedConfig.ReportInterval())
		})
	}
}

func TestNewConfig_InvalidIntervals(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agent.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
agent:
  ticker:
    poll_interval: 2s
    report_interval: 10s
`), 0o600))

	for _, value := range []string{"abc", "0", "-1", "1.5"} {
		loadedConfig, err := config.NewConfig(path, noop.New(),
			config.WithEnviron([]string{"POLL_INTERVAL=" + value}))
		require.ErrorIs(t, err, entities.ErrInvalidParam, value)
		require.Nil(t, loadedConfig)
	}
}
