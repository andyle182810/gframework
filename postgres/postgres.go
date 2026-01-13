package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	pgxzerolog "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog/log"
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
}

type Postgres struct {
	DBPool
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

	pgConfig.MaxConns = cfg.MaxConnection
	pgConfig.MinConns = cfg.MinConnection
	pgConfig.MaxConnIdleTime = cfg.MaxConnectionIdleTime
	pgConfig.MaxConnLifetime = cfg.MaxConnectionLifetime
	pgConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	pgConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	pgConfig.ConnConfig.Tracer = tracer

	if cfg.StatementTimeout > 0 {
		pgConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	}

	if cfg.LockTimeout > 0 {
		pgConfig.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(cfg.LockTimeout.Milliseconds(), 10)
	}

	if cfg.IdleInTransactionTimeout > 0 {
		//nolint:lll
		pgConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(cfg.IdleInTransactionTimeout.Milliseconds(), 10)
	}

	pgConfig.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())

		return nil
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
	if err != nil {
		return nil, err
	}

	return &Postgres{
		DBPool: pool,
	}, nil
}

func (p *Postgres) Run(ctx context.Context) error {
	log.Info().
		Str("service_name", p.Name()).
		Msg("The PostgreSQL connection pool is operational and waiting for shutdown signal.")

	<-ctx.Done()
	log.Info().
		Str("service_name", p.Name()).
		Msg("The PostgreSQL Run() context has been cancelled.")

	return nil
}

func (p *Postgres) Stop(_ context.Context) error {
	if p.DBPool == nil {
		return ErrConnectionPoolNil
	}

	log.Info().
		Str("service_name", p.Name()).
		Msg("The PostgreSQL connection pool is being closed.")
	p.DBPool.Close()

	log.Info().
		Str("service_name", p.Name()).
		Msg("The PostgreSQL connection pool has been closed successfully.")

	return nil
}

func (p *Postgres) Name() string {
	return "postgres"
}
