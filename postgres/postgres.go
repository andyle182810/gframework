package postgres

import (
	"context"
	"fmt"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	pgxzerolog "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog/log"
)

var ErrConnectionPoolNil = fmt.Errorf("postgres: connection pool is nil")

type Config struct {
	URL                   string
	MaxConnection         int32
	MinConnection         int32
	MaxConnectionIdleTime time.Duration
	LogLevel              tracelog.LogLevel
}

type Postgres struct {
	DBPool
	name string
}

func New(cfg *Config) (*Postgres, error) {
	if cfg == nil {
		return nil, fmt.Errorf("postgres: configuration must not be nil")
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
	pgConfig.ConnConfig.Tracer = tracer

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
		name:   "postgres_infrastructure",
	}, nil
}

func (p *Postgres) Run() {
	log.Info().Str("service_name", p.Name()).Msg("PostgreSQL pool operational. Waiting for shutdown signal")
}

func (p *Postgres) Stop(ctx context.Context) error {
	if p.DBPool == nil {
		return ErrConnectionPoolNil
	}

	log.Info().Str("service_name", p.Name()).Msg("Closing PostgreSQL connection pool...")
	p.DBPool.Close()

	log.Info().Str("service_name", p.Name()).Msg("PostgreSQL connection pool closed")

	return nil
}

func (p *Postgres) Name() string {
	return "postgres"
}
