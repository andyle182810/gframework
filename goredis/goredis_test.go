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
