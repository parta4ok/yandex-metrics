package unconfigured_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parta4ok/yandex-metrics/toolkit/pinger/unconfigured"
)

func TestPinger_Ping(t *testing.T) {
	t.Parallel()

	err := unconfigured.New().Ping(context.Background())

	require.ErrorIs(t, err, unconfigured.ErrNotConfigured)
}
