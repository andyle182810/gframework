package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	// Application
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// HTTP Server
	HTTPServerHost         string        `env:"HTTP_SERVER_HOST"          envDefault:"0.0.0.0"`
	HTTPServerPort         int           `env:"HTTP_SERVER_PORT"          envDefault:"8080"`
	HTTPEnableCORS         bool          `env:"HTTP_ENABLE_CORS"          envDefault:"true"`
	HTTPBodyLimit          string        `env:"HTTP_BODY_LIMIT"           envDefault:"10M"`
	HTTPServerReadTimeout  time.Duration `env:"HTTP_SERVER_READ_TIMEOUT"  envDefault:"30s"`
	HTTPServerWriteTimeout time.Duration `env:"HTTP_SERVER_WRITE_TIMEOUT" envDefault:"30s"`
	HTTPSkipRequestID      bool          `env:"HTTP_SKIP_REQUEST_ID"      envDefault:"false"`

	// Metric Server
	MetricServerHost         string        `env:"METRIC_SERVER_HOST"          envDefault:"0.0.0.0"`
	MetricServerPort         int           `env:"METRIC_SERVER_PORT"          envDefault:"9090"`
	MetricServerReadTimeout  time.Duration `env:"METRIC_SERVER_READ_TIMEOUT"  envDefault:"10s"`
	MetricServerWriteTimeout time.Duration `env:"METRIC_SERVER_WRITE_TIMEOUT" envDefault:"10s"`

	// Graceful Shutdown
	GracefulShutdownPeriod time.Duration `env:"GRACEFUL_SHUTDOWN_PERIOD" envDefault:"10s"`

	// PostgreSQL
	PostgresHost     string `env:"POSTGRES_HOST"     envDefault:"localhost"`
	PostgresPort     int    `env:"POSTGRES_PORT"     envDefault:"5441"`
	PostgresUser     string `env:"POSTGRES_USER"     envDefault:"postgres"`
	PostgresPassword string `env:"POSTGRES_PASSWORD" envDefault:"password"`
	PostgresDatabase string `env:"POSTGRES_DB"       envDefault:"admindb"`
	PostgresSSLMode  string `env:"POSTGRES_SSL_MODE" envDefault:"disable"`

	PostgresMaxConnection            int           `env:"POSTGRES_MAX_CONNECTION"              envDefault:"25"`
	PostgresMinConnection            int           `env:"POSTGRES_MIN_CONNECTION"              envDefault:"5"`
	PostgresMaxConnectionIdleTime    time.Duration `env:"POSTGRES_MAX_CONNECTION_IDLE_TIME"    envDefault:"30m"`
	PostgresMaxConnectionLifetime    time.Duration `env:"POSTGRES_MAX_CONNECTION_LIFETIME"     envDefault:"1h"`
	PostgresHealthCheckPeriod        time.Duration `env:"POSTGRES_HEALTH_CHECK_PERIOD"         envDefault:"1m"`
	PostgresConnectTimeout           time.Duration `env:"POSTGRES_CONNECT_TIMEOUT"             envDefault:"10s"`
	PostgresStatementTimeout         time.Duration `env:"POSTGRES_STATEMENT_TIMEOUT"           envDefault:"30s"`
	PostgresLockTimeout              time.Duration `env:"POSTGRES_LOCK_TIMEOUT"                envDefault:"10s"`
	PostgresIdleInTransactionTimeout time.Duration `env:"POSTGRES_IDLE_IN_TRANSACTION_TIMEOUT" envDefault:"30s"`
	PostgresLogLevel                 string        `env:"POSTGRES_LOG_LEVEL"                   envDefault:"info"`

	// Valkey (Redis)
	ValkeyHost         string        `env:"VALKEY_HOST"           envDefault:"localhost"`
	ValkeyPort         int           `env:"VALKEY_PORT"           envDefault:"6379"`
	ValkeyPassword     string        `env:"VALKEY_PASSWORD"       envDefault:"password"`
	ValkeyDB           int           `env:"VALKEY_DB"             envDefault:"0"`
	ValkeyDialTimeout  time.Duration `env:"VALKEY_DIAL_TIMEOUT"   envDefault:"5s"`
	ValkeyReadTimeout  time.Duration `env:"VALKEY_READ_TIMEOUT"   envDefault:"3s"`
	ValkeyWriteTimeout time.Duration `env:"VALKEY_WRITE_TIMEOUT"  envDefault:"3s"`
	ValkeyPoolSize     int           `env:"VALKEY_POOL_SIZE"      envDefault:"10"`
	ValkeyMinIdleConns int           `env:"VALKEY_MIN_IDLE_CONNS" envDefault:"2"`
	ValkeyMaxIdleConns int           `env:"VALKEY_MAX_IDLE_CONNS" envDefault:"5"`
	ValkeyPingTimeout  time.Duration `env:"VALKEY_PING_TIMEOUT"   envDefault:"3s"`

	// Migration settings
	MigrationEnabled bool   `env:"MIGRATION_ENABLED" envDefault:"false"`
	MigrationSource  string `env:"MIGRATION_SOURCE"  envDefault:"file://db/migrations"`
}

func New() (*Config, error) {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresDatabase,
		c.PostgresSSLMode,
	)
}
