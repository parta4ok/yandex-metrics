package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "metrics.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestNewConfig(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
metrics:
  application:
    graceful_shutdown_timeout: 7s
  http:
    public:
      address: :8080
      tls:
        enabled: true
        certificate_file: cert.pem
        key_file: key.pem
  backup:
    store_interval: 10s
    file_storage_path: file.json
    restore: true
`)

	loadedConfig, err := config.NewConfig([]string{"-config", path}, nil)
	require.NoError(t, err)
	require.Equal(t, ":8080", loadedConfig.HTTPAddress())
	require.Equal(t, 7*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.True(t, loadedConfig.TLSEnabled())
	require.Equal(t, "cert.pem", loadedConfig.TLSCertificateFile())
	require.Equal(t, "key.pem", loadedConfig.TLSKeyFile())
	require.Equal(t, 10*time.Second, loadedConfig.StoreInterval())
	require.Equal(t, "file.json", loadedConfig.FileStoragePath())
	require.True(t, loadedConfig.Restore())
}

func TestNewConfig_Defaults(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "metrics: {}\n")

	loadedConfig, err := config.NewConfig([]string{"-config", path}, nil)
	require.NoError(t, err)
	require.Equal(t, "localhost:8080", loadedConfig.HTTPAddress())
	require.Equal(t, 5*time.Second, loadedConfig.GracefulShutdownTimeout())
	require.False(t, loadedConfig.TLSEnabled())
	require.Equal(t, 300*time.Second, loadedConfig.StoreInterval())
	require.Equal(t, "metrics-storage.json", loadedConfig.FileStoragePath())
	require.False(t, loadedConfig.Restore())
}

func TestNewConfig_Priority(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
metrics:
  http:
    public:
      address: file:8080
  backup:
    store_interval: 10s
    file_storage_path: file.json
    restore: true
`)

	tests := []struct {
		name         string
		args         []string
		environ      []string
		wantAddress  string
		wantInterval time.Duration
		wantPath     string
		wantRestore  bool
	}{
		{
			name:         "file over defaults",
			wantAddress:  "file:8080",
			wantInterval: 10 * time.Second,
			wantPath:     "file.json",
			wantRestore:  true,
		},
		{
			name:         "flags over file, zero interval and false restore",
			args:         []string{"-a", "flag:8081", "-i", "0", "-f", "flag.json", "-r=false"},
			wantAddress:  "flag:8081",
			wantInterval: 0,
			wantPath:     "flag.json",
		},
		{
			name:         "environment over flags",
			args:         []string{"-a", "flag:8081", "-i", "0", "-f", "flag.json", "-r=false"},
			environ:      []string{"ADDRESS=env:8082", "STORE_INTERVAL=5", "FILE_STORAGE_PATH=env.json", "RESTORE=true"},
			wantAddress:  "env:8082",
			wantInterval: 5 * time.Second,
			wantPath:     "env.json",
			wantRestore:  true,
		},
		{
			name:         "unrelated environment is ignored",
			environ:      []string{"OTHER=value"},
			wantAddress:  "file:8080",
			wantInterval: 10 * time.Second,
			wantPath:     "file.json",
			wantRestore:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loadedConfig, err := config.NewConfig(append([]string{"-config", path}, tt.args...), tt.environ)
			require.NoError(t, err)
			require.Equal(t, tt.wantAddress, loadedConfig.HTTPAddress())
			require.Equal(t, tt.wantInterval, loadedConfig.StoreInterval())
			require.Equal(t, tt.wantPath, loadedConfig.FileStoragePath())
			require.Equal(t, tt.wantRestore, loadedConfig.Restore())
		})
	}
}

func TestNewConfig_ConfigPath(t *testing.T) {
	t.Parallel()

	flagPath := writeConfig(t, "metrics:\n  http:\n    public:\n      address: flag-file:1\n")
	envPath := writeConfig(t, "metrics:\n  http:\n    public:\n      address: env-file:2\n")

	loadedConfig, err := config.NewConfig(
		[]string{"-config", flagPath},
		[]string{"METRIC_CONFIG_PATH=" + envPath},
	)
	require.NoError(t, err)
	require.Equal(t, "flag-file:1", loadedConfig.HTTPAddress())

	loadedConfig, err = config.NewConfig(nil, []string{"METRIC_CONFIG_PATH=" + envPath})
	require.NoError(t, err)
	require.Equal(t, "env-file:2", loadedConfig.HTTPAddress())

	loadedConfig, err = config.NewConfig(
		[]string{"-config", filepath.Join(t.TempDir(), "missing.yml")},
		nil,
	)
	require.Error(t, err)
	require.Nil(t, loadedConfig)
}

func TestNewConfig_InvalidValues(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "metrics: {}\n")
	brokenPath := writeConfig(t, "metrics:\n  backup:\n    restore: maybe\n")

	tests := []struct {
		name    string
		args    []string
		environ []string
	}{
		{name: "env interval is not a number", environ: []string{"STORE_INTERVAL=abc"}},
		{name: "env interval is negative", environ: []string{"STORE_INTERVAL=-1"}},
		{name: "env interval is empty", environ: []string{"STORE_INTERVAL="}},
		{name: "env restore is not a boolean", environ: []string{"RESTORE=maybe"}},
		{name: "flag interval is not a number", args: []string{"-i", "abc"}},
		{name: "flag interval is negative", args: []string{"-i", "-1"}},
		{name: "flag restore is not a boolean", args: []string{"-r=maybe"}},
		{name: "unknown flag", args: []string{"-x"}},
		{name: "unexpected argument", args: []string{"extra"}},
		{name: "file restore is not a boolean", args: []string{"-config", brokenPath}},
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
