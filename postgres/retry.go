//nolint:varnamelen,wsl
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultMaxRetries     = 3
	defaultRetryDelay     = 100 * time.Millisecond
	defaultMaxRetryDelay  = 5 * time.Second
	exponentialMultiplier = 2
)

type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	RetryableErrs []string
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   defaultMaxRetries,
		InitialDelay: defaultRetryDelay,
		MaxDelay:     defaultMaxRetryDelay,
		Multiplier:   exponentialMultiplier,
		RetryableErrs: []string{
			"40001", // serialization_failure
			"40P01", // deadlock_detected
			"53300", // too_many_connections
			"08006", // connection_failure
			"08003", // connection_does_not_exist
		},
	}
}

func IsRetryableError(err error, retryableCodes []string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return slices.Contains(retryableCodes, pgErr.Code)
}

type RetryableFunc func(ctx context.Context) error

func WithRetry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
				// Calculate next delay with exponential backoff
				delay = min(time.Duration(float64(delay)*config.Multiplier), config.MaxDelay)
			}
		}

		if err := fn(ctx); err != nil {
			lastErr = err

			if !IsRetryableError(err, config.RetryableErrs) {
				return fmt.Errorf("non-retryable error: %w", err)
			}

			if attempt == config.MaxRetries {
				return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, err)
			}

			continue
		}

		return nil
	}

	return fmt.Errorf("retry failed: %w", lastErr)
}

func (p *Postgres) WithRetryTx(ctx context.Context, config RetryConfig, fn TxFunc) error {
	return WithRetry(ctx, config, func(ctx context.Context) error {
		return p.WithTransaction(ctx, fn)
	})
}

func (p *Postgres) WithRetryTxDefault(ctx context.Context, fn TxFunc) error {
	return p.WithRetryTx(ctx, DefaultRetryConfig(), fn)
}
