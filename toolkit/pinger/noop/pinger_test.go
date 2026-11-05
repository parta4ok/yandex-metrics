package noop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/pinger/noop"
)

func TestPinger_Ping(t *testing.T) {
	t.Parallel()

	err := noop.New().Ping(context.Background())

	require.ErrorIs(t, err, noop.ErrNotConfigured)
}
