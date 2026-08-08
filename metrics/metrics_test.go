package metrics_test

import (
	"testing"

	"github.com/andyle182810/gframework/metrics"
	"github.com/stretchr/testify/require"
)

func TestEnabledCanBeToggled(t *testing.T) {
	t.Parallel()

	t.Cleanup(metrics.Enable)

	metrics.Disable()
	require.False(t, metrics.Enabled())

	metrics.Enable()
	require.True(t, metrics.Enabled())

	metrics.SetEnabled(false)
	require.False(t, metrics.Enabled())
}
