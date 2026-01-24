//nolint:exhaustruct
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/require"
)

func setupRetryTestPostgres(t *testing.T) (*postgres.Postgres, context.Context) {
	t.Helper()

	ctx := t.Context()

	container := testutil.SetupPostgresContainer(t)

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		container.User,
		container.Password,
		net.JoinHostPort(container.Host, container.Port.Port()),
		container.Database,
	)

	opts := &postgres.Config{
		URL:                   dbURL,
		MaxConnection:         5,
		MinConnection:         1,
		MaxConnectionIdleTime: 60 * time.Second,
		HealthCheckPeriod:     10 * time.Second,
		LogLevel:              tracelog.LogLevelTrace,
	}

	pg, err := postgres.New(opts)
	require.NoError(t, err)

	t.Cleanup(func() {
		pg.Close()
	})

	return pg, ctx
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()

	config := postgres.DefaultRetryConfig()

	require.Equal(t, 3, config.MaxRetries)
	require.Equal(t, 100*time.Millisecond, config.InitialDelay)
	require.Equal(t, 5*time.Second, config.MaxDelay)
	require.InEpsilon(t, float64(2), config.Multiplier, 0.01)
	require.Len(t, config.RetryableErrs, 5)
	require.Contains(t, config.RetryableErrs, "40001") // serialization_failure
	require.Contains(t, config.RetryableErrs, "40P01") // deadlock_detected
	require.Contains(t, config.RetryableErrs, "53300") // too_many_connections
	require.Contains(t, config.RetryableErrs, "08006") // connection_failure
	require.Contains(t, config.RetryableErrs, "08003") // connection_does_not_exist
}

func TestIsRetryableError_WithPgError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		errorCode      string
		retryableCodes []string
		expected       bool
	}{
		{
			name:           "serialization_failure is retryable",
			errorCode:      "40001",
			retryableCodes: []string{"40001", "40P01"},
			expected:       true,
		},
		{
			name:           "deadlock_detected is retryable",
			errorCode:      "40P01",
			retryableCodes: []string{"40001", "40P01"},
			expected:       true,
		},
		{
			name:           "non-retryable error code",
			errorCode:      "23505", // unique_violation
			retryableCodes: []string{"40001", "40P01"},
			expected:       false,
		},
		{
			name:           "empty retryable codes list",
			errorCode:      "40001",
			retryableCodes: []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{Code: tt.errorCode}
			result := postgres.IsRetryableError(pgErr, tt.retryableCodes)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError_WithNonPgError(t *testing.T) {
	t.Parallel()

	regularErr := errors.New("regular error") //nolint:err113
	result := postgres.IsRetryableError(regularErr, []string{"40001", "40P01"})
	require.False(t, result)
}

func TestIsRetryableError_WithWrappedPgError(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{Code: "40001"}
	wrappedErr := fmt.Errorf("wrapped: %w", pgErr)

	result := postgres.IsRetryableError(wrappedErr, []string{"40001", "40P01"})
	require.True(t, result)
}

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		atomic.AddInt32(&attempts, 1)

		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return &pgconn.PgError{Code: "40001"}
		}

		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		atomic.AddInt32(&attempts, 1)

		return &pgconn.PgError{Code: "40001"}
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "max retries (2) exceeded")
	require.Equal(t, int32(3), atomic.LoadInt32(&attempts)) // initial + 2 retries
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		atomic.AddInt32(&attempts, 1)

		return &pgconn.PgError{Code: "23505"} // unique_violation - not retryable
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "non-retryable error")
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	config := postgres.RetryConfig{
		MaxRetries:    5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		atomic.AddInt32(&attempts, 1)

		return &pgconn.PgError{Code: "40001"}
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "retry cancelled")
	require.ErrorIs(t, err, context.Canceled)
}

func TestWithRetry_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var timestamps []time.Time

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) <= 3 {
			return &pgconn.PgError{Code: "40001"}
		}

		return nil
	})

	require.NoError(t, err)
	require.Len(t, timestamps, 4)

	// Check that delays are increasing (with some tolerance)
	delay1 := timestamps[1].Sub(timestamps[0])
	delay2 := timestamps[2].Sub(timestamps[1])
	delay3 := timestamps[3].Sub(timestamps[2])

	require.GreaterOrEqual(t, delay1.Milliseconds(), int64(40))  // ~50ms
	require.GreaterOrEqual(t, delay2.Milliseconds(), int64(80))  // ~100ms
	require.GreaterOrEqual(t, delay3.Milliseconds(), int64(160)) // ~200ms
}

func TestWithRetry_MaxDelayRespected(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      150 * time.Millisecond, // Cap at 150ms
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var timestamps []time.Time

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) <= 4 {
			return &pgconn.PgError{Code: "40001"}
		}

		return nil
	})

	require.NoError(t, err)
	require.Len(t, timestamps, 5)

	// Later delays should be capped at MaxDelay
	delay3 := timestamps[3].Sub(timestamps[2])
	delay4 := timestamps[4].Sub(timestamps[3])

	// Both should be around MaxDelay (150ms) with some tolerance
	require.LessOrEqual(t, delay3.Milliseconds(), int64(200))
	require.LessOrEqual(t, delay4.Milliseconds(), int64(200))
}

func TestWithRetryTx_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupRetryTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS retry_tx_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	err = pg.WithRetryTx(ctx, config, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO retry_tx_test (value) VALUES ($1)", "test_value")

		return execErr
	})
	require.NoError(t, err)

	var count int
	err = pg.QueryRow(ctx, "SELECT COUNT(*) FROM retry_tx_test").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestWithRetryTx_RollbackOnNonRetryableError(t *testing.T) {
	t.Parallel()

	pg, ctx := setupRetryTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS retry_tx_rollback_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL UNIQUE
		)
	`)
	require.NoError(t, err)

	// Insert a value to cause unique violation
	_, err = pg.Exec(ctx, "INSERT INTO retry_tx_rollback_test (value) VALUES ($1)", "existing")
	require.NoError(t, err)

	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"}, // unique_violation (23505) is NOT in this list
	}

	var attempts int32

	err = pg.WithRetryTx(ctx, config, func(ctx context.Context, tx pgx.Tx) error {
		atomic.AddInt32(&attempts, 1)

		_, execErr := tx.Exec(ctx, "INSERT INTO retry_tx_rollback_test (value) VALUES ($1)", "existing")

		return execErr
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "non-retryable error")
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestWithRetryTxDefault_Success(t *testing.T) {
	t.Parallel()

	pg, ctx := setupRetryTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS retry_tx_default_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	err = pg.WithRetryTxDefault(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO retry_tx_default_test (value) VALUES ($1)", "default_test")

		return execErr
	})
	require.NoError(t, err)

	var value string
	err = pg.QueryRow(ctx, "SELECT value FROM retry_tx_default_test WHERE id = 1").Scan(&value)
	require.NoError(t, err)
	require.Equal(t, "default_test", value)
}

func TestWithRetryTxDefault_UsesDefaultConfig(t *testing.T) {
	t.Parallel()

	pg, ctx := setupRetryTestPostgres(t)

	_, err := pg.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS retry_default_config_test (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	var attempts int32

	// This will use default config which has serialization_failure (40001) as retryable
	err = pg.WithRetryTxDefault(ctx, func(ctx context.Context, tx pgx.Tx) error {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			_, execErr := tx.Exec(ctx, "INSERT INTO retry_default_config_test (value) VALUES ($1)", "first")
			if execErr != nil {
				return execErr
			}
		}

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestWithRetry_RegularErrorNotRetryable(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	regularErr := errors.New("some regular error") //nolint:err113

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		atomic.AddInt32(&attempts, 1)

		return regularErr
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "non-retryable error")
	require.ErrorIs(t, err, regularErr)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestWithRetry_ZeroMaxRetries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := postgres.RetryConfig{
		MaxRetries:    0,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		Multiplier:    2,
		RetryableErrs: []string{"40001"},
	}

	var attempts int32

	err := postgres.WithRetry(ctx, config, func(_ context.Context) error {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			return &pgconn.PgError{Code: "40001"}
		}

		return nil
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "max retries (0) exceeded")
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}
