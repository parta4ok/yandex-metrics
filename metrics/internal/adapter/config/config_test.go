package config_test

import (
	"flag"
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

func TestNewConfig_AddressPriority(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "metrics.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
metrics:
  http:
    public:
      address: file:8080
`), 0o600))

	newFlags := func(args ...string) *flag.FlagSet {
		flags := flag.NewFlagSet("test", flag.ContinueOnError)
		flags.String("config", "", "")
		flags.String("a", "", "")
		require.NoError(t, flags.Parse(args))

		return flags
	}

	tests := []struct {
		name    string
		flags   *flag.FlagSet
		environ []string
		want    string
	}{
		{name: "file only", want: "file:8080"},
		{name: "unset flag keeps file value", flags: newFlags(), want: "file:8080"},
		{name: "flag over file", flags: newFlags("-a", "flag:8081"), want: "flag:8081"},
		{
			name:    "environment over flag",
			flags:   newFlags("-a", "flag:8081"),
			environ: []string{"ADDRESS=env:8082", "OTHER=value"},
			want:    "env:8082",
		},
		{
			name:    "unrelated environment is ignored",
			flags:   newFlags("-a", "flag:8081"),
			environ: []string{"OTHER=value"},
			want:    "flag:8081",
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
			require.Equal(t, tt.want, loadedConfig.HTTPAddress())
		})
	}
}
