package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agent.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestNewConfig(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
agent:
  application:
    graceful_shutdown_timeout: 7s
  ticker:
    poll_interval: 3s
    report_interval: 11s
  http:
    metrics:
      address: metrics:8080
      timeout: 4s
`)

	loadedConfig, err := config.NewConfig([]string{"-config", path}, nil)
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.Equal(t, 3*time.Second, loadedConfig.PollInterval())
	require.Equal(t, 11*time.Second, loadedConfig.ReportInterval())
	require.Equal(t, "metrics:8080", loadedConfig.MetricsHTTPAddress())
	require.Equal(t, 4*time.Second, loadedConfig.MetricsHTTPTimeout())
}

func TestNewConfig_Defaults(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "agent: {}\n")

	loadedConfig, err := config.NewConfig([]string{"-config", path}, nil)
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.Equal(t, 2*time.Second, loadedConfig.PollInterval())
	require.Equal(t, 10*time.Second, loadedConfig.ReportInterval())
	require.Equal(t, "localhost:8080", loadedConfig.MetricsHTTPAddress())
	require.Equal(t, 5*time.Second, loadedConfig.MetricsHTTPTimeout())
}

func TestNewConfig_Priority(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
agent:
  ticker:
    poll_interval: 3s
    report_interval: 11s
  http:
    metrics:
      address: file:8080
`)

	tests := []struct {
		name        string
		args        []string
		environ     []string
		wantAddress string
		wantPoll    time.Duration
		wantReport  time.Duration
	}{
		{
			name:        "file over defaults",
			wantAddress: "file:8080",
			wantPoll:    3 * time.Second,
			wantReport:  11 * time.Second,
		},
		{
			name:        "flags over file",
			args:        []string{"-a", "flag:8081", "-p", "4", "-r", "12"},
			wantAddress: "flag:8081",
			wantPoll:    4 * time.Second,
			wantReport:  12 * time.Second,
		},
		{
			name:        "environment over flags",
			args:        []string{"-a", "flag:8081", "-p", "4", "-r", "12"},
			environ:     []string{"ADDRESS=env:8082", "POLL_INTERVAL=5", "REPORT_INTERVAL=13"},
			wantAddress: "env:8082",
			wantPoll:    5 * time.Second,
			wantReport:  13 * time.Second,
		},
		{
			name:        "unrelated environment is ignored",
			environ:     []string{"OTHER=value"},
			wantAddress: "file:8080",
			wantPoll:    3 * time.Second,
			wantReport:  11 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loadedConfig, err := config.NewConfig(append([]string{"-config", path}, tt.args...), tt.environ)
			require.NoError(t, err)
			require.Equal(t, tt.wantAddress, loadedConfig.MetricsHTTPAddress())
			require.Equal(t, tt.wantPoll, loadedConfig.PollInterval())
			require.Equal(t, tt.wantReport, loadedConfig.ReportInterval())
		})
	}
}

func TestNewConfig_ConfigPath(t *testing.T) {
	t.Parallel()

	flagPath := writeConfig(t, "agent:\n  http:\n    metrics:\n      address: flag-file:1\n")
	envPath := writeConfig(t, "agent:\n  http:\n    metrics:\n      address: env-file:2\n")

	loadedConfig, err := config.NewConfig(
		[]string{"-config", flagPath},
		[]string{"METRIC_CONFIG_PATH=" + envPath},
	)
	require.NoError(t, err)
	require.Equal(t, "flag-file:1", loadedConfig.MetricsHTTPAddress())

	loadedConfig, err = config.NewConfig(nil, []string{"METRIC_CONFIG_PATH=" + envPath})
	require.NoError(t, err)
	require.Equal(t, "env-file:2", loadedConfig.MetricsHTTPAddress())

	loadedConfig, err = config.NewConfig(
		[]string{"-config", filepath.Join(t.TempDir(), "missing.yml")},
		nil,
	)
	require.Error(t, err)
	require.Nil(t, loadedConfig)
}

func TestNewConfig_InvalidValues(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "agent: {}\n")
	brokenPath := writeConfig(t, "agent:\n  ticker:\n    poll_interval: 0s\n")

	tests := []struct {
		name    string
		args    []string
		environ []string
	}{
		{name: "env interval is not a number", environ: []string{"POLL_INTERVAL=abc"}},
		{name: "env interval is zero", environ: []string{"POLL_INTERVAL=0"}},
		{name: "env interval is negative", environ: []string{"REPORT_INTERVAL=-1"}},
		{name: "env interval is fractional", environ: []string{"POLL_INTERVAL=1.5"}},
		{name: "flag interval is not a number", args: []string{"-p", "abc"}},
		{name: "flag interval is zero", args: []string{"-r", "0"}},
		{name: "unknown flag", args: []string{"-x"}},
		{name: "unexpected argument", args: []string{"extra"}},
		{name: "file interval is not positive", args: []string{"-config", brokenPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := tt.args
			if args == nil || args[0] != "-config" {
				args = append([]string{"-config", path}, args...)
			}

			loadedConfig, err := config.NewConfig(args, tt.environ)
			require.ErrorIs(t, err, entities.ErrInvalidParam)
			require.Nil(t, loadedConfig)
		})
	}
}
