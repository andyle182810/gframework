// Package distlock provides Redis-backed distributed locking with two usage modes: fail-hard and fail-silent.
//
// The locker wraps the bsm/redislock library and ensures locks are always released via defer,
// even if the handler panics. Lock release failures are logged as warnings but do not cause errors.
//
// Basic usage (fail-hard):
//
//	locker := distlock.New(redisClient)
//	err := locker.WithLock(ctx, "resource:123", 30*time.Second, func() error {
//	    // Exclusive access to resource:123 for 30 seconds
//	    return updateResource()
//	})
//	if err != nil && errors.Is(err, distlock.ErrLockNotObtained) {
//	    return fmt.Errorf("failed to acquire lock")
//	}
//
// For background jobs that should silently skip if the lock is unavailable:
//
//	err := locker.TryWithLock(ctx, "job:cleanup", 5*time.Minute, cleanupHandler)
//	// ErrLockNotObtained is swallowed and logged at debug level
//
// Lock TTL is enforced by Redis; the handler should complete well before the TTL expires.
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
				Str("source", "gframework").
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
			Str("source", "gframework").
			Str("key", key).
			Msg("Could not obtain lock, another instance is running")

		return nil
	}

	return err
}
