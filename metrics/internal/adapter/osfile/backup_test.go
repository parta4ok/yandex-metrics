package osfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/osfile"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/noop"
)

func TestBackup_MakeAndRestore(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "backup.json")
	backup, err := osfile.NewBackup(path, noop.New())
	require.NoError(t, err)

	counter, err := entities.NewMetric("NumGC", entities.Counter)
	require.NoError(t, err)
	delta := int64(42)
	counter.SetDelta(&delta)

	gauge, err := entities.NewMetric("LastGC", entities.Gauge)
	require.NoError(t, err)
	value := 1.5
	gauge.SetValue(&value)

	require.NoError(t, backup.MakeBackup(context.Background(), []*entities.Metric{counter, gauge}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"id":"NumGC","type":"counter","delta":42},
		{"id":"LastGC","type":"gauge","value":1.5}
	]`, string(data))

	restored, err := backup.RestoreBackup(context.Background())
	require.NoError(t, err)
	require.Len(t, restored, 2)
	require.Equal(t, "NumGC", restored[0].ID())
	require.Equal(t, int64(42), *restored[0].Delta())
	require.Equal(t, "LastGC", restored[1].ID())
	require.Equal(t, 1.5, *restored[1].Value())
}

func TestBackup_RestoreMissingFile(t *testing.T) {
	t.Parallel()

	backup, err := osfile.NewBackup(filepath.Join(t.TempDir(), "missing.json"), noop.New())
	require.NoError(t, err)

	restored, err := backup.RestoreBackup(context.Background())
	require.NoError(t, err)
	require.Empty(t, restored)
}

func TestBackup_RestoreInvalidFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "backup.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
	backup, err := osfile.NewBackup(path, noop.New())
	require.NoError(t, err)

	restored, err := backup.RestoreBackup(context.Background())
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, restored)
}

func TestNewBackup_InvalidParams(t *testing.T) {
	t.Parallel()

	backup, err := osfile.NewBackup("", noop.New())
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, backup)

	backup, err = osfile.NewBackup("backup.json", nil)
	require.ErrorIs(t, err, entities.ErrInvalidParam)
	require.Nil(t, backup)
}
