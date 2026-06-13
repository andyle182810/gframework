package distlock_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andyle182810/gframework/distlock"
	"github.com/redis/go-redis/v9"
)

// ExampleLocker_WithLock shows fail-hard locking: the caller decides what to do
// when the lock is held elsewhere. (Not run: requires a Redis server.)
//
//nolint:testableexamples // requires live infrastructure; compile-checked only
func ExampleLocker_WithLock() {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"}) //nolint:exhaustruct
	locker := distlock.New(client)

	err := locker.WithLock(context.Background(), "resource:123", 30*time.Second, func() error {
		// Exclusive access to resource:123 for up to 30 seconds.
		return nil
	})
	if errors.Is(err, distlock.ErrLockNotObtained) {
		fmt.Println("another instance holds the lock")
	}
}

// ExampleLocker_TryWithLock shows fail-silent locking for background jobs:
// when the lock is unavailable the handler is skipped without an error.
// (Not run: requires a Redis server.)
//
//nolint:testableexamples // requires live infrastructure; compile-checked only
func ExampleLocker_TryWithLock() {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"}) //nolint:exhaustruct
	locker := distlock.New(client)

	_ = locker.TryWithLock(context.Background(), "jobs:cleanup", 5*time.Minute, func() error {
		// Runs only on the instance that obtained the lock.
		return nil
	})
}
