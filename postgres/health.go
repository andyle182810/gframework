//nolint:exhaustruct
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHealthCheckTimeout = 5 * time.Second
)

var (
	ErrHealthCheckTimeout   = errors.New("postgres: health check timeout")
	ErrNoActiveConnections  = errors.New("postgres: no active connections in pool")
	ErrPoolStatsUnavailable = errors.New("postgres: pool stats unavailable")
)

type HealthCheckOptions struct {
	Timeout              time.Duration
	RequireActiveConns   bool
	MinIdleConns         int32
	CheckQueryExecution  bool
	CustomHealthCheckSQL string
}

type HealthCheckOption func(*HealthCheckOptions)

func WithHealthCheckTimeout(timeout time.Duration) HealthCheckOption {
	return func(opts *HealthCheckOptions) {
		opts.Timeout = timeout
	}
}

func WithRequireActiveConns() HealthCheckOption {
	return func(opts *HealthCheckOptions) {
		opts.RequireActiveConns = true
	}
}

func WithMinIdleConns(minConns int32) HealthCheckOption {
	return func(opts *HealthCheckOptions) {
		opts.MinIdleConns = minConns
	}
}

func WithCustomHealthCheckSQL(sql string) HealthCheckOption {
	return func(opts *HealthCheckOptions) {
		opts.CheckQueryExecution = true
		opts.CustomHealthCheckSQL = sql
	}
}

func (p *Postgres) HealthCheck(ctx context.Context, opts ...HealthCheckOption) error {
	if p.DBPool == nil {
		return ErrConnectionPoolNil
	}

	options := &HealthCheckOptions{
		Timeout:             defaultHealthCheckTimeout,
		RequireActiveConns:  false,
		MinIdleConns:        0,
		CheckQueryExecution: false,
	}

	for _, opt := range opts {
		opt(options)
	}

	healthCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	if err := p.DBPool.Ping(healthCtx); err != nil {
		return fmt.Errorf("postgres health check ping failed: %w", err)
	}

	if options.CheckQueryExecution {
		querySQL := options.CustomHealthCheckSQL
		if querySQL == "" {
			querySQL = "SELECT 1"
		}

		var result int
		if err := p.DBPool.QueryRow(healthCtx, querySQL).Scan(&result); err != nil {
			return fmt.Errorf("postgres health check query failed: %w", err)
		}
	}

	return nil
}

type PoolStats struct {
	AcquireCount            int64
	AcquireDuration         time.Duration
	AcquiredConns           int32
	CanceledAcquireCount    int64
	ConstructingConns       int32
	EmptyAcquireCount       int64
	IdleConns               int32
	MaxConns                int32
	TotalConns              int32
	NewConnsCount           int64
	MaxLifetimeDestroyCount int64
	MaxIdleDestroyCount     int64
}

func (p *Postgres) GetPoolStats() (*PoolStats, error) {
	pool, ok := p.DBPool.(*pgxpool.Pool)
	if !ok {
		return nil, ErrPoolStatsUnavailable
	}

	stats := pool.Stat()

	return &PoolStats{
		AcquireCount:            stats.AcquireCount(),
		AcquireDuration:         stats.AcquireDuration(),
		AcquiredConns:           stats.AcquiredConns(),
		CanceledAcquireCount:    stats.CanceledAcquireCount(),
		ConstructingConns:       stats.ConstructingConns(),
		EmptyAcquireCount:       stats.EmptyAcquireCount(),
		IdleConns:               stats.IdleConns(),
		MaxConns:                stats.MaxConns(),
		TotalConns:              stats.TotalConns(),
		NewConnsCount:           stats.NewConnsCount(),
		MaxLifetimeDestroyCount: stats.MaxLifetimeDestroyCount(),
		MaxIdleDestroyCount:     stats.MaxIdleDestroyCount(),
	}, nil
}

func (p *Postgres) IsHealthy(ctx context.Context) bool {
	return p.HealthCheck(ctx) == nil
}
