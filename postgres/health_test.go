//nolint:exhaustruct,usetesting
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck_BasicSuccess(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx)
	require.NoError(t, err)
}

func TestHealthCheck_WithCustomTimeout(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx, postgres.WithHealthCheckTimeout(10*time.Second))
	require.NoError(t, err)
}

func TestHealthCheck_WithQueryExecution(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx, postgres.WithCustomHealthCheckSQL("SELECT 1"))
	require.NoError(t, err)
}

func TestHealthCheck_WithComplexQuery(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx, postgres.WithCustomHealthCheckSQL("SELECT 1 + 1"))
	require.NoError(t, err)
}

func TestHealthCheck_WithMultipleOptions(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx,
		postgres.WithHealthCheckTimeout(5*time.Second),
		postgres.WithCustomHealthCheckSQL("SELECT 1"),
	)
	require.NoError(t, err)
}

func TestHealthCheck_FailsWithInvalidQuery(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	err := pg.HealthCheck(ctx, postgres.WithCustomHealthCheckSQL("SELECT * FROM nonexistent_table"))
	require.Error(t, err)
}

func TestHealthCheck_FailsWithCancelledContext(t *testing.T) {
	t.Parallel()

	pg, _ := setupTestPostgres(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pg.HealthCheck(ctx)
	require.Error(t, err)
}

func TestIsHealthy_ReturnsTrueWhenHealthy(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	healthy := pg.IsHealthy(ctx)
	require.True(t, healthy)
}

func TestIsHealthy_ReturnsFalseWithCancelledContext(t *testing.T) {
	t.Parallel()

	pg, _ := setupTestPostgres(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	healthy := pg.IsHealthy(ctx)
	require.False(t, healthy)
}

func TestGetPoolStats_ReturnsStats(t *testing.T) {
	t.Parallel()

	pg, _ := setupTestPostgres(t)

	stats, err := pg.GetPoolStats()
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.GreaterOrEqual(t, stats.MaxConns, int32(1))
}

func TestGetPoolStats_UpdatesAfterQueries(t *testing.T) {
	t.Parallel()

	pg, ctx := setupTestPostgres(t)

	var result int
	err := pg.DBPool.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err)

	stats, err := pg.GetPoolStats()
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.AcquireCount, int64(1))
}

func TestWithHealthCheckTimeout_SetsTimeout(t *testing.T) {
	t.Parallel()

	opts := &postgres.HealthCheckOptions{}
	postgres.WithHealthCheckTimeout(3 * time.Second)(opts)

	require.Equal(t, 3*time.Second, opts.Timeout)
}

func TestWithRequireActiveConns_SetsFlag(t *testing.T) {
	t.Parallel()

	opts := &postgres.HealthCheckOptions{}
	postgres.WithRequireActiveConns()(opts)

	require.True(t, opts.RequireActiveConns)
}

func TestWithMinIdleConns_SetsMinimum(t *testing.T) {
	t.Parallel()

	opts := &postgres.HealthCheckOptions{}
	postgres.WithMinIdleConns(5)(opts)

	require.Equal(t, int32(5), opts.MinIdleConns)
}

func TestWithCustomHealthCheckSQL_SetsQueryAndEnablesCheck(t *testing.T) {
	t.Parallel()

	opts := &postgres.HealthCheckOptions{}
	postgres.WithCustomHealthCheckSQL("SELECT version()")(opts)

	require.True(t, opts.CheckQueryExecution)
	require.Equal(t, "SELECT version()", opts.CustomHealthCheckSQL)
}
