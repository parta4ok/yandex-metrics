package noop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/backup/noop"
)

func TestBackup(t *testing.T) {
	t.Parallel()

	backup := noop.New[string]()

	require.NoError(t, backup.MakeBackup(context.Background(), []string{"a"}))

	restored, err := backup.RestoreBackup(context.Background())
	require.NoError(t, err)
	require.Empty(t, restored)
}
