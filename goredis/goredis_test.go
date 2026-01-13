//nolint:exhaustruct,paralleltest,tparallel,usetesting,testifylint // Test configuration
package goredis_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/andyle182810/gframework/goredis"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

func TestRedisConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start the test container
	container := testutil.SetupRedisContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &goredis.Config{
		Host:         container.Host,
		Port:         port,
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		MaxIdleConns: 5,
		MinIdleConns: 1,
		PingTimeout:  2 * time.Second,
	}

	redis, err := goredis.New(opts)
	require.NoError(t, err)

	_, err = redis.Ping(ctx).Result()
	require.NoError(t, err, "failed to ping Redis")
}

func TestRedisConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *goredis.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &goredis.Config{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			expectError: false,
		},
		{
			name: "missing host",
			config: &goredis.Config{
				Port: 6379,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "host is required",
		},
		{
			name: "invalid port - too low",
			config: &goredis.Config{
				Host: "localhost",
				Port: 0,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "port must be between",
		},
		{
			name: "invalid port - too high",
			config: &goredis.Config{
				Host: "localhost",
				Port: 70000,
				DB:   0,
			},
			expectError: true,
			errorMsg:    "port must be between",
		},
		{
			name: "negative DB",
			config: &goredis.Config{
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

func TestRedisConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := &goredis.Config{
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

func TestRedisHealthCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start the test container
	container := testutil.SetupRedisContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &goredis.Config{
		Host: container.Host,
		Port: port,
	}

	redis, err := goredis.New(opts)
	require.NoError(t, err)

	// Test health check
	err = redis.HealthCheck(ctx)
	require.NoError(t, err)

	// Test pool stats
	stats := redis.PoolStats()
	require.NotNil(t, stats)
	require.Greater(t, stats.TotalConns, uint32(0))
}
