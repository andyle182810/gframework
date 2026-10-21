// Package postgres provides a PostgreSQL connection pool factory with built-in support for decimal types,
// query tracing via zerolog, and configurable session-level timeouts.
//
// The pool is created from a pgxpool.Pool and supports automatic health checks, connection limits,
// and idle timeout management. Shopspring decimal types are automatically registered for JSON marshaling.
//
// Basic usage:
//
//	pool, err := postgres.New(&postgres.Config{
//	    URL:           "postgres://user:pass@localhost/dbname",
//	    MaxConnection: 25,
//	    LogLevel:      "info",
//	})
//	if err != nil {
//	    return err
//	}
//
//	// Use pool for queries
//	var user User
//	err = pool.QueryRow(ctx, "SELECT ... WHERE id = $1", userID).Scan(&user)
//
// Session-level timeouts (statement_timeout, lock_timeout, idle_in_transaction_session_timeout) can be
// configured to prevent long-running queries from blocking other connections.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	frameworkmetrics "github.com/andyle182810/gframework/metrics"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	pgxzerolog "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog/log"
)

const (
	initialPingTimeout = 5 * time.Second
)

var (
	ErrConnectionPoolNil = errors.New("postgres: connection pool is nil")
	ErrConfigNil         = errors.New("postgres: configuration must not be nil")
)

type Config struct {
	URL                      string
	MaxConnection            int32
	MinConnection            int32
	MaxConnectionIdleTime    time.Duration
	MaxConnectionLifetime    time.Duration
	HealthCheckPeriod        time.Duration
	ConnectTimeout           time.Duration
	LogLevel                 tracelog.LogLevel
	StatementTimeout         time.Duration
	LockTimeout              time.Duration
	IdleInTransactionTimeout time.Duration
	SlowQueryThreshold       time.Duration
	DisableMetrics           bool
}

type Postgres struct {
	DBPool
	metricsPool *pgxpool.Pool
}

func New(cfg *Config) (*Postgres, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	pgConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}

	tracerLogger := log.Logger.With().Str("component", "pgx_tracer").Logger()
	logger := pgxzerolog.NewLogger(tracerLogger, pgxzerolog.WithoutPGXModule())

	tracer := &tracelog.TraceLog{
		Logger:   logger,
		LogLevel: cfg.LogLevel,
		Config:   nil,
	}

	if cfg.DisableMetrics || !frameworkmetrics.Enabled() {
		pgConfig.ConnConfig.Tracer = tracer
	} else {
		pgConfig.ConnConfig.Tracer = multitracer.New(tracer, newMetricsTracer(cfg.SlowQueryThreshold))
	}

	applyPoolTuning(pgConfig, cfg)

	applySessionTimeouts(pgConfig, cfg)

	pgConfig.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())

		return nil
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
	if err != nil {
		return nil, err
	}

	if !cfg.DisableMetrics {
		registerPoolMetrics(pool)
	}

	return &Postgres{
		DBPool:      pool,
		metricsPool: pool,
	}, nil
}

func applyPoolTuning(pgConfig *pgxpool.Config, cfg *Config) {
	if cfg.MaxConnection > 0 {
		pgConfig.MaxConns = cfg.MaxConnection
	}

	if cfg.MinConnection > 0 {
		pgConfig.MinConns = cfg.MinConnection
	}

	if cfg.MaxConnectionIdleTime > 0 {
		pgConfig.MaxConnIdleTime = cfg.MaxConnectionIdleTime
	}

	if cfg.MaxConnectionLifetime > 0 {
		pgConfig.MaxConnLifetime = cfg.MaxConnectionLifetime
	}

	if cfg.HealthCheckPeriod > 0 {
		pgConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	if cfg.ConnectTimeout > 0 {
		pgConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}
}

func applySessionTimeouts(pgConfig *pgxpool.Config, cfg *Config) {
	if cfg.StatementTimeout > 0 {
		pgConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	}

	if cfg.LockTimeout > 0 {
		pgConfig.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(cfg.LockTimeout.Milliseconds(), 10)
	}

	if cfg.IdleInTransactionTimeout > 0 {
		idleTimeout := strconv.FormatInt(cfg.IdleInTransactionTimeout.Milliseconds(), 10)
		pgConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = idleTimeout
	}
}

func (p *Postgres) Start(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, initialPingTimeout)
	defer cancel()

	if err := p.Ping(pingCtx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}

	return nil
}

func (p *Postgres) Stop() error {
	if p.DBPool == nil {
		return ErrConnectionPoolNil
	}

	log.Info().
		Str("source", "gframework").
		Str("service_name", p.Name()).
		Msg("The PostgreSQL connection pool is being closed")

	unregisterPoolMetrics(p.metricsPool)
	p.Close()

	log.Info().
		Str("source", "gframework").
		Str("service_name", p.Name()).
		Msg("The PostgreSQL connection pool has been closed successfully")

	return nil
}

func (p *Postgres) Name() string {
	return "postgres"
}
