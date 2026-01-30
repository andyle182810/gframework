package distlock

import (
	"context"
	"errors"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var ErrLockNotObtained = redislock.ErrNotObtained

type Locker struct {
	client *redislock.Client
}

func New(redisClient redis.UniversalClient) *Locker {
	return &Locker{
		client: redislock.New(redisClient),
	}
}

func (l *Locker) WithLock(ctx context.Context, key string, ttl time.Duration, handler func() error) error {
	lock, err := l.client.Obtain(ctx, key, ttl, nil)
	if err != nil {
		if errors.Is(err, redislock.ErrNotObtained) {
			return ErrLockNotObtained
		}

		return err
	}

	defer func() {
		if releaseErr := lock.Release(ctx); releaseErr != nil {
			log.Warn().
				Err(releaseErr).
				Str("key", key).
				Msg("Failed to release distributed lock")
		}
	}()

	return handler()
}

func (l *Locker) TryWithLock(ctx context.Context, key string, ttl time.Duration, handler func() error) error {
	err := l.WithLock(ctx, key, ttl, handler)
	if errors.Is(err, ErrLockNotObtained) {
		log.Debug().
			Str("key", key).
			Msg("Could not obtain lock, another instance is running")

		return nil
	}

	return err
}
