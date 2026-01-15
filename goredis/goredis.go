package goredis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	defaultDialTimeout     = 5 * time.Second
	defaultPingTimeout     = 3 * time.Second
	defaultReadTimeout     = 3 * time.Second
	defaultWriteTimeout    = 3 * time.Second
	defaultMinRetryBackoff = 8 * time.Millisecond
	defaultMaxRetryBackoff = 512 * time.Millisecond
	initialPingTimeout     = 5 * time.Second
	minPort                = 1
	maxPort                = 65535
	defaultPoolSize        = 10
	defaultMaxIdleConns    = 5
	defaultMinIdleConns    = 1
	defaultMaxRetries      = 3
)

var (
	ErrRedisPoolNil             = errors.New("redis: client connection is nil")
	ErrInvalidHost              = errors.New("redis: host is required")
	ErrInvalidPort              = errors.New("redis: port must be between 1 and 65535")
	ErrInvalidDB                = errors.New("redis: database number must be non-negative")
	ErrInvalidPoolSize          = errors.New("redis: pool size must be positive")
	ErrConfigNil                = errors.New("redis: configuration must not be nil")
	ErrCAParseFailure           = errors.New("failed to parse CA certificate")
	ErrHealthCheckNoActiveConns = errors.New("redis health check failed: no active connections in pool")
)

type Config struct {
	Host            string
	Port            int
	Password        string
	DB              int
	DialTimeout     time.Duration
	MaxIdleConns    int
	MinIdleConns    int
	PingTimeout     time.Duration
	PoolSize        int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	TLSEnabled      bool
	TLSSkipVerify   bool
	TLSCertFile     string
	TLSKeyFile      string
	TLSCAFile       string
}

type Redis struct {
	*redis.Client
}

func (cfg *Config) Validate() error {
	if cfg.Host == "" {
		return ErrInvalidHost
	}

	if cfg.Port <= minPort-1 || cfg.Port > maxPort {
		return fmt.Errorf("%w: %d", ErrInvalidPort, cfg.Port)
	}

	if cfg.DB < 0 {
		return fmt.Errorf("%w: %d", ErrInvalidDB, cfg.DB)
	}

	if cfg.PoolSize < 0 {
		return ErrInvalidPoolSize
	}

	return nil
}

//nolint:cyclop
func (cfg *Config) WithDefaults() *Config {
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaultDialTimeout
	}

	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = defaultPingTimeout
	}

	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaultReadTimeout
	}

	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}

	if cfg.PoolSize == 0 {
		cfg.PoolSize = defaultPoolSize
	}

	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = defaultMaxIdleConns
	}

	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = defaultMinIdleConns
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}

	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = defaultMinRetryBackoff
	}

	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = defaultMaxRetryBackoff
	}

	return cfg
}

func New(cfg *Config) (*Redis, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opt, err := buildRedisOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build Redis options: %w", err)
	}

	client := redis.NewClient(opt)

	return &Redis{Client: client}, nil
}

//nolint:exhaustruct
func buildRedisOptions(cfg *Config) (*redis.Options, error) {
	opt := &redis.Options{
		Addr:            net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Password:        cfg.Password,
		DB:              cfg.DB,
		DialTimeout:     cfg.DialTimeout,
		MaxIdleConns:    cfg.MaxIdleConns,
		MinIdleConns:    cfg.MinIdleConns,
		PoolSize:        cfg.PoolSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	}

	if cfg.TLSEnabled {
		tlsConfig, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}

		opt.TLSConfig = tlsConfig
	}

	return opt, nil
}

//nolint:gosec,exhaustruct
func buildTLSConfig(cfg *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, ErrCAParseFailure
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

func (rds *Redis) Start(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, initialPingTimeout)
	defer cancel()

	if _, err := rds.Client.Ping(pingCtx).Result(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	log.Info().
		Str("service_name", rds.Name()).
		Msg("The Redis client is operational and waiting for shutdown signal.")

	<-ctx.Done()
	log.Info().
		Str("service_name", rds.Name()).
		Msg("The Redis Run() context has been cancelled.")

	return nil
}

func (rds *Redis) Stop() error {
	if rds.Client == nil {
		return ErrRedisPoolNil
	}

	log.Info().
		Str("service_name", rds.Name()).
		Msg("The Redis client pool is being closed.")

	if err := rds.Client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}

	log.Info().
		Str("service_name", rds.Name()).
		Msg("The Redis client pool has been closed successfully.")

	return nil
}

func (rds *Redis) Name() string {
	return "redis"
}

func (rds *Redis) PoolStats() *redis.PoolStats {
	if rds.Client == nil {
		return nil
	}

	return rds.Client.PoolStats()
}

func (rds *Redis) HealthCheck(ctx context.Context) error {
	if rds.Client == nil {
		return ErrRedisPoolNil
	}

	if _, err := rds.Client.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	stats := rds.PoolStats()
	if stats != nil && stats.TotalConns == 0 {
		return ErrHealthCheckNoActiveConns
	}

	return nil
}
