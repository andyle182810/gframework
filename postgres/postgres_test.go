//nolint:exhaustruct,usetesting
//nolint:usetesting
package postgres_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/require"
)

func TestPostgresConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start the test container
	container := testutil.SetupPostgresContainer(ctx, t)

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		container.User,
		container.Password,
		net.JoinHostPort(container.Host, container.Port.Port()),
		container.Database,
	)

	// Define options
	opts := &postgres.Config{
		URL:                   dbURL,
		MaxConnection:         5,
		MinConnection:         1,
		MaxConnectionIdleTime: 60 * time.Second,
		LogLevel:              tracelog.LogLevelTrace,
	}

	// Create Postgres instance
	postgres, err := postgres.New(opts)
	require.NoError(t, err)

	// Check if connection is alive
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = postgres.Ping(ctx)
	require.NoError(t, err)

	// Close the DB pool
	postgres.Close()
}
