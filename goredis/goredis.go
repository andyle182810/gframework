package goredis

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var ErrRedisPoolNil = fmt.Errorf("redis: client connection is nil")

type Config struct {
	Host         string
	Port         int
	Password     string
	DB           int
	DialTimeout  time.Duration
	MaxIdleConns int
	MinIdleConns int
	PingTimeout  time.Duration
	PoolSize     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type Redis struct {
	*redis.Client
}

func New(cfg *Config) (*Redis, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis: configuration must not be nil")
	}

	opt := buildRedisOptions(cfg)
	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return &Redis{Client: client}, nil
}

func buildRedisOptions(cfg *Config) *redis.Options {
	opt := &redis.Options{
		Addr:         net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		MaxIdleConns: cfg.MaxIdleConns,
		MinIdleConns: cfg.MinIdleConns,
		PoolSize:     cfg.PoolSize,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	return opt
}

func (r *Redis) Run() {
	log.Info().Str("service_name", r.Name()).Msg("Redis client operational. Waiting for shutdown signal")
}

func (r *Redis) Stop(ctx context.Context) error {
	if r.Client == nil {
		return ErrRedisPoolNil
	}

	log.Info().Str("service_name", r.Name()).Msg("Closing Redis client pool...")

	if err := r.Client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}

	log.Info().Str("service_name", r.Name()).Msg("Redis client pool closed")

	return nil
}

func (r *Redis) Name() string {
	return "redis"
}
