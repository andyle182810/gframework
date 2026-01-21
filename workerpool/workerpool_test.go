package workerpool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/workerpool"
	"github.com/stretchr/testify/require"
)

var errExecutor = errors.New("executor error")

type mockExecutor struct {
	execCount    atomic.Int32
	execErr      error
	execDuration time.Duration
}

func (m *mockExecutor) Execute(ctx context.Context) error {
	m.execCount.Add(1)

	if m.execDuration > 0 {
		select {
		case <-time.After(m.execDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.execErr
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      nil,
		execDuration: 0,
	}
}

func TestNew_DefaultValues(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor)
	require.NotNil(t, pool)
}

func TestNew_WithCustomWorkerCount(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor, workerpool.WithWorkerCount(5))
	require.NotNil(t, pool)
}

func TestNew_WithCustomTickInterval(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor, workerpool.WithTickInterval(500*time.Millisecond))
	require.NotNil(t, pool)
}

func TestNew_WithExecutionTimeout(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor, workerpool.WithExecutionTimeout(5*time.Second))
	require.NotNil(t, pool)
}

func TestNew_InvalidWorkerCountIsIgnored(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor, workerpool.WithWorkerCount(0))
	require.NotNil(t, pool)
}

func TestNew_InvalidTickIntervalIsIgnored(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()

	pool := workerpool.New(executor, workerpool.WithTickInterval(0))
	require.NotNil(t, pool)
}

func TestWorkerPool_StartsAndStopsWorkers(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(3),
		workerpool.WithTickInterval(50*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(150 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count <= 0 {
		t.Errorf("expected executor to be called at least once, got %d", count)
	}
}

func TestWorkerPool_MultipleStartCallsAreIdempotent(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()
	pool := workerpool.New(executor, workerpool.WithTickInterval(50*time.Millisecond))

	ctx := t.Context()

	require.NoError(t, pool.Start(ctx))

	if err := pool.Start(ctx); !errors.Is(err, workerpool.ErrAlreadyRunning) {
		t.Errorf("expected ErrAlreadyRunning on second start, got %v", err)
	}

	if err := pool.Start(ctx); !errors.Is(err, workerpool.ErrAlreadyRunning) {
		t.Errorf("expected ErrAlreadyRunning on third start, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_ = pool.Stop()
}

func TestWorkerPool_MultipleStopCallsAreIdempotent(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()
	pool := workerpool.New(executor, workerpool.WithTickInterval(50*time.Millisecond))

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(100 * time.Millisecond)

	_ = pool.Stop()
	_ = pool.Stop()
	_ = pool.Stop()
}

func TestWorkerPool_StopWithoutStartIsSafe(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()
	pool := workerpool.New(executor)

	_ = pool.Stop()
}

func TestWorkerPool_ContinuesOnExecutorError(t *testing.T) {
	t.Parallel()

	executor := &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      errExecutor,
		execDuration: 0,
	}
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(1),
		workerpool.WithTickInterval(50*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(150 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count <= 1 {
		t.Errorf("expected executor to continue being called despite errors, got %d calls", count)
	}
}

func TestWorkerPool_RespectsParentContextCancellation(t *testing.T) {
	t.Parallel()

	executor := newMockExecutor()
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(2),
		workerpool.WithTickInterval(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())

	require.NoError(t, pool.Start(ctx))

	countBefore := executor.execCount.Load()

	time.Sleep(100 * time.Millisecond)

	cancel()

	time.Sleep(100 * time.Millisecond)

	countAfterCancel := executor.execCount.Load()

	time.Sleep(100 * time.Millisecond)

	countLater := executor.execCount.Load()

	require.Greater(t, countAfterCancel, countBefore, "expected executor to be called before cancel")

	if countAfterCancel != countLater {
		t.Errorf("expected executor not to be called after context cancellation, "+
			"afterCancel=%d, later=%d", countAfterCancel, countLater)
	}

	_ = pool.Stop()
}

func TestWorkerPool_BusyWorkersAllowOthersToPickUpJobs(t *testing.T) {
	t.Parallel()

	executor := &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      nil,
		execDuration: 80 * time.Millisecond,
	}
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(3),
		workerpool.WithTickInterval(30*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(200 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count <= 2 {
		t.Errorf("expected multiple jobs to be processed by different workers, got %d", count)
	}
}

func TestWorkerPool_AllWorkersBusyCausesTickToWait(t *testing.T) {
	t.Parallel()

	executor := &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      nil,
		execDuration: 200 * time.Millisecond,
	}
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(2),
		workerpool.WithTickInterval(20*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(150 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count != 2 {
		t.Errorf("expected only 2 jobs to be processed when all workers are busy, got %d", count)
	}
}

func TestWorkerPool_ExecutionTimeoutCancelsLongRunningTasks(t *testing.T) {
	t.Parallel()

	executor := &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      nil,
		execDuration: 500 * time.Millisecond,
	}
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(1),
		workerpool.WithTickInterval(50*time.Millisecond),
		workerpool.WithExecutionTimeout(100*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(300 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count <= 1 {
		t.Errorf("expected timeout to allow worker to process multiple jobs, got %d", count)
	}
}

func TestWorkerPool_NoTimeoutAllowsLongRunningTasksToComplete(t *testing.T) {
	t.Parallel()

	executor := &mockExecutor{
		execCount:    atomic.Int32{},
		execErr:      nil,
		execDuration: 100 * time.Millisecond,
	}
	pool := workerpool.New(executor,
		workerpool.WithWorkerCount(1),
		workerpool.WithTickInterval(30*time.Millisecond),
	)

	require.NoError(t, pool.Start(t.Context()))

	time.Sleep(250 * time.Millisecond)

	_ = pool.Stop()

	count := executor.execCount.Load()
	if count <= 0 {
		t.Errorf("expected executor to complete without timeout, got %d", count)
	}
}
