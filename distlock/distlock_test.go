package distlock_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/distlock"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/stretchr/testify/require"
)

var errHandler = errors.New("handler error")

func setupTestLocker(t *testing.T) *distlock.Locker {
	t.Helper()

	container := testutil.SetupValkeyContainer(t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	//nolint:exhaustruct
	valkeyClient, err := valkey.New(&valkey.Config{
		Host: container.Host,
		Port: port,
	})
	require.NoError(t, err)

	return distlock.New(valkeyClient.Client)
}

func TestNew(t *testing.T) {
	t.Parallel()

	locker := setupTestLocker(t)
	require.NotNil(t, locker)
}

func TestWithLock_Success(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	var executed bool

	err := locker.WithLock(ctx, "test:lock:success", 5*time.Second, func() error {
		executed = true

		return nil
	})

	require.NoError(t, err)
	require.True(t, executed)
}

func TestWithLock_HandlerError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	err := locker.WithLock(ctx, "test:lock:handler-error", 5*time.Second, func() error {
		return errHandler
	})

	require.ErrorIs(t, err, errHandler)
}

func TestWithLock_LockNotObtained(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	lockKey := "test:lock:not-obtained"

	var firstLockReleased atomic.Bool

	var wg sync.WaitGroup

	wg.Go(func() {
		_ = locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
			for !firstLockReleased.Load() {
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		})
	})

	time.Sleep(100 * time.Millisecond)

	err := locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
		t.Error("handler should not be called when lock is not obtained")

		return nil
	})

	require.ErrorIs(t, err, distlock.ErrLockNotObtained)

	firstLockReleased.Store(true)
	wg.Wait()
}

func TestWithLock_ReleasesLockAfterHandler(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	lockKey := "test:lock:release"

	err := locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
		return nil
	})
	require.NoError(t, err)

	var secondExecuted bool

	err = locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
		secondExecuted = true

		return nil
	})

	require.NoError(t, err)
	require.True(t, secondExecuted)
}

func TestTryWithLock_Success(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	var executed bool

	err := locker.TryWithLock(ctx, "test:trylock:success", 5*time.Second, func() error {
		executed = true

		return nil
	})

	require.NoError(t, err)
	require.True(t, executed)
}

func TestTryWithLock_HandlerError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	err := locker.TryWithLock(ctx, "test:trylock:handler-error", 5*time.Second, func() error {
		return errHandler
	})

	require.ErrorIs(t, err, errHandler)
}

func TestTryWithLock_ReturnsNilWhenLockNotObtained(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	lockKey := "test:trylock:not-obtained"

	var firstLockReleased atomic.Bool

	var wg sync.WaitGroup

	wg.Go(func() {
		_ = locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
			for !firstLockReleased.Load() {
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		})
	})

	time.Sleep(100 * time.Millisecond)

	var handlerCalled bool

	err := locker.TryWithLock(ctx, lockKey, 5*time.Second, func() error {
		handlerCalled = true

		return nil
	})

	require.NoError(t, err)
	require.False(t, handlerCalled, "handler should not be called when lock is not obtained")

	firstLockReleased.Store(true)
	wg.Wait()
}

func TestWithLock_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	locker := setupTestLocker(t)

	lockKey := "test:lock:concurrent"

	var counter atomic.Int32

	var maxConcurrent atomic.Int32

	var currentConcurrent atomic.Int32

	workerCount := 5
	iterationsPerWorker := 3

	var wg sync.WaitGroup

	for range workerCount {
		wg.Go(func() {
			for range iterationsPerWorker {
				for {
					err := locker.WithLock(ctx, lockKey, 5*time.Second, func() error {
						current := currentConcurrent.Add(1)

						for {
							peak := maxConcurrent.Load()
							if current <= peak || maxConcurrent.CompareAndSwap(peak, current) {
								break
							}
						}

						time.Sleep(10 * time.Millisecond)

						counter.Add(1)
						currentConcurrent.Add(-1)

						return nil
					})

					if !errors.Is(err, distlock.ErrLockNotObtained) {
						break
					}

					time.Sleep(5 * time.Millisecond)
				}
			}
		})
	}

	wg.Wait()

	require.Equal(t, int32(workerCount*iterationsPerWorker), counter.Load()) //nolint:gosec

	require.Equal(t, int32(1), maxConcurrent.Load())
}

func TestWithLock_ContextCancellation(t *testing.T) {
	t.Parallel()

	locker := setupTestLocker(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := locker.WithLock(ctx, "test:lock:cancelled", 5*time.Second, func() error {
		t.Error("handler should not be called with cancelled context")

		return nil
	})

	require.Error(t, err)
}
