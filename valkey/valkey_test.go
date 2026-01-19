//nolint:exhaustruct,paralleltest,tparallel,usetesting,testifylint
package valkey_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/stretchr/testify/require"
)

func TestValkeyConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start the test container
	container := testutil.SetupValkeyContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &valkey.Config{
		Host:         container.Host,
		Port:         port,
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		MaxIdleConns: 5,
		MinIdleConns: 1,
		PingTimeout:  2 * time.Second,
	}

	v, err := valkey.New(opts)
	require.NoError(t, err)

	_, err = v.Ping(ctx).Result()
	require.NoError(t, err, "failed to ping Valkey")
}

func TestValkeyConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *valkey.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &valkey.Config{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			expectError: false,
		},
		{
			name: "missing host",
			config: &valkey.Config{
				Port: 6379,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "host is required",
		},
		{
			name: "invalid port - too low",
			config: &valkey.Config{
				Host: "localhost",
				Port: 0,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "port must be between",
		},
		{
			name: "invalid port - too high",
			config: &valkey.Config{
				Host: "localhost",
				Port: 70000,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "port must be between",
		},
		{
			name: "negative DB",
			config: &valkey.Config{
				Host: "localhost",
				Port: 6379,
				DB:   -1,
			},
			expectError: true,
			errorMsg:    "database number must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValkeyConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := &valkey.Config{
		Host: "localhost",
		Port: 6379,
	}

	cfg = cfg.WithDefaults()

	require.Equal(t, 5*time.Second, cfg.DialTimeout)
	require.Equal(t, 3*time.Second, cfg.PingTimeout)
	require.Equal(t, 3*time.Second, cfg.ReadTimeout)
	require.Equal(t, 3*time.Second, cfg.WriteTimeout)
	require.Equal(t, 10, cfg.PoolSize)
	require.Equal(t, 5, cfg.MaxIdleConns)
	require.Equal(t, 1, cfg.MinIdleConns)
	require.Equal(t, 3, cfg.MaxRetries)
	require.Equal(t, 8*time.Millisecond, cfg.MinRetryBackoff)
	require.Equal(t, 512*time.Millisecond, cfg.MaxRetryBackoff)
}

func TestValkeyHealthCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start the test container
	container := testutil.SetupValkeyContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &valkey.Config{
		Host: container.Host,
		Port: port,
	}

	valkey, err := valkey.New(opts)
	require.NoError(t, err)

	// Test health check
	err = valkey.HealthCheck(ctx)
	require.NoError(t, err)

	// Test pool stats
	stats := valkey.PoolStats()
	require.NotNil(t, stats)
	require.Greater(t, stats.TotalConns, uint32(0))
}
