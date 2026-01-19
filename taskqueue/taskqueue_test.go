//nolint:exhaustruct,paralleltest,tparallel,usetesting
package taskqueue_test

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/taskqueue"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type mockExecutor struct {
	fn func(ctx context.Context, taskID string) error
}

func (m *mockExecutor) Execute(ctx context.Context, taskID string) error {
	return m.fn(ctx, taskID)
}

func setupTestQueue(ctx context.Context, t *testing.T) *valkey.Valkey {
	t.Helper()

	container := testutil.SetupValkeyContainer(ctx, t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	valkeyClient, err := valkey.New(&valkey.Config{
		Host: container.Host,
		Port: port,
	})
	require.NoError(t, err)

	return valkeyClient
}

func TestNew(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			return nil
		},
	}

	tests := []struct {
		name        string
		client      func() *valkey.Valkey
		queueKey    string
		executor    taskqueue.Executor
		expectError error
	}{
		{
			name:        "valid configuration",
			client:      func() *valkey.Valkey { return valkeyClient },
			queueKey:    "test:queue",
			executor:    executor,
			expectError: nil,
		},
		{
			name:        "nil client",
			client:      func() *valkey.Valkey { return nil },
			queueKey:    "test:queue",
			executor:    executor,
			expectError: taskqueue.ErrNilClient,
		},
		{
			name:        "empty queue key",
			client:      func() *valkey.Valkey { return valkeyClient },
			queueKey:    "",
			executor:    executor,
			expectError: taskqueue.ErrEmptyQueueKey,
		},
		{
			name:        "nil executor",
			client:      func() *valkey.Valkey { return valkeyClient },
			queueKey:    "test:queue",
			executor:    nil,
			expectError: taskqueue.ErrNilExecutor,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			redisClient := testCase.client()

			var queue *taskqueue.Queue

			var err error

			if redisClient != nil {
				queue, err = taskqueue.New(redisClient.Client, testCase.queueKey, testCase.executor)
			} else {
				queue, err = taskqueue.New(nil, testCase.queueKey, testCase.executor)
			}

			if testCase.expectError != nil {
				require.ErrorIs(t, err, testCase.expectError)
				require.Nil(t, queue)
			} else {
				require.NoError(t, err)
				require.NotNil(t, queue)
			}
		})
	}
}

func TestQueueStartStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			return nil
		},
	}

	queue, err := taskqueue.New(valkeyClient, "test:start-stop", executor)
	require.NoError(t, err)

	err = queue.Start(ctx)
	require.NoError(t, err)

	err = queue.Start(ctx)
	require.ErrorIs(t, err, taskqueue.ErrQueueAlreadyRunning)

	err = queue.Stop()
	require.NoError(t, err)

	err = queue.Stop()
	require.NoError(t, err)
}

func TestQueuePushAndProcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	var processedTasks sync.Map

	var processedCount atomic.Int32

	executor := &mockExecutor{
		fn: func(_ context.Context, taskID string) error {
			processedTasks.Store(taskID, true)
			processedCount.Add(1)

			return nil
		},
	}

	queue, err := taskqueue.New(
		valkeyClient,
		"test:push-process",
		executor,
		taskqueue.WithWorkerCount(3),
		taskqueue.WithExecTimeout(5*time.Second),
	)
	require.NoError(t, err)

	err = queue.Push(ctx, "task1", "task2", "task3")
	require.NoError(t, err)

	length, err := queue.QueueLength(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), length)

	err = queue.Start(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return processedCount.Load() == 3
	}, 10*time.Second, 100*time.Millisecond)

	err = queue.Stop()
	require.NoError(t, err)

	_, ok := processedTasks.Load("task1")
	require.True(t, ok)
	_, ok = processedTasks.Load("task2")
	require.True(t, ok)
	_, ok = processedTasks.Load("task3")
	require.True(t, ok)

	length, err = queue.QueueLength(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), length)
}

func TestQueueExecTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	var timedOut atomic.Bool

	executor := &mockExecutor{
		fn: func(ctx context.Context, _ string) error {
			select {
			case <-ctx.Done():
				timedOut.Store(true)

				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		},
	}

	queue, err := taskqueue.New(
		valkeyClient,
		"test:timeout",
		executor,
		taskqueue.WithWorkerCount(1),
		taskqueue.WithExecTimeout(500*time.Millisecond),
	)
	require.NoError(t, err)

	err = queue.Push(ctx, "slow-task")
	require.NoError(t, err)

	err = queue.Start(ctx)
	require.NoError(t, err)

	require.Eventually(t, timedOut.Load, 5*time.Second, 100*time.Millisecond)

	err = queue.Stop()
	require.NoError(t, err)
}

func TestQueueConcurrentProcessing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	var peakConcurrent atomic.Int32

	var currentConcurrent atomic.Int32

	var totalProcessed atomic.Int32

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			current := currentConcurrent.Add(1)

			// Track max concurrent workers
			for {
				peak := peakConcurrent.Load()
				if current <= peak || peakConcurrent.CompareAndSwap(peak, current) {
					break
				}
			}

			// Simulate work
			time.Sleep(100 * time.Millisecond)

			currentConcurrent.Add(-1)
			totalProcessed.Add(1)

			return nil
		},
	}

	workerCount := 5
	queue, err := taskqueue.New(
		valkeyClient,
		"test:concurrent",
		executor,
		taskqueue.WithWorkerCount(workerCount),
		taskqueue.WithExecTimeout(5*time.Second),
	)
	require.NoError(t, err)

	taskCount := 20
	for i := range taskCount {
		err = queue.Push(ctx, "task-"+strconv.Itoa(i))
		require.NoError(t, err)
	}

	err = queue.Start(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return totalProcessed.Load() == int32(taskCount) //nolint:gosec
	}, 30*time.Second, 100*time.Millisecond)

	err = queue.Stop()
	require.NoError(t, err)

	require.Greater(t, peakConcurrent.Load(), int32(1), "Expected concurrent execution")
	require.LessOrEqual(t, peakConcurrent.Load(), int32(workerCount), "Should not exceed worker count")
}

func TestQueueRecoverStale(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	queueKey := "test:recover"
	processingKey := queueKey + ":processing"

	// Add task with old timestamp (simulating a crashed worker)
	oldTimestamp := float64(time.Now().Add(-2 * time.Minute).Unix())
	err := valkeyClient.ZAdd(ctx, processingKey, redis.Z{
		Score:  oldTimestamp,
		Member: "stale-task",
	}).Err()
	require.NoError(t, err)

	// Add task with recent timestamp (should not be recovered)
	recentTimestamp := float64(time.Now().Unix())
	err = valkeyClient.ZAdd(ctx, processingKey, redis.Z{
		Score:  recentTimestamp,
		Member: "recent-task",
	}).Err()
	require.NoError(t, err)

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			return nil
		},
	}

	queue, err := taskqueue.New(valkeyClient, queueKey, executor)
	require.NoError(t, err)

	recovered, err := queue.RecoverStale(ctx, time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)

	length, err := queue.QueueLength(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), length)

	processingCount, err := queue.ProcessingCount(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), processingCount)
}

func TestQueueUniqueTaskProcessing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	var taskProcessCount sync.Map

	executor := &mockExecutor{
		fn: func(_ context.Context, taskID string) error {
			// Count how many times each task is processed
			val, _ := taskProcessCount.LoadOrStore(taskID, new(atomic.Int32))

			counter, ok := val.(*atomic.Int32)
			if ok {
				counter.Add(1)
			}

			time.Sleep(50 * time.Millisecond)

			return nil
		},
	}

	queue, err := taskqueue.New(
		valkeyClient,
		"test:unique",
		executor,
		taskqueue.WithWorkerCount(5),
	)
	require.NoError(t, err)

	for i := range 10 {
		err = queue.Push(ctx, "task-"+strconv.Itoa(i))
		require.NoError(t, err)
	}

	err = queue.Start(ctx)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	err = queue.Stop()
	require.NoError(t, err)

	taskProcessCount.Range(func(key, value any) bool {
		counter, ok := value.(*atomic.Int32)
		if ok {
			count := counter.Load()
			require.Equal(t, int32(1), count, "Task %s should be processed exactly once", key)
		}

		return true
	})
}

func TestQueueName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			return nil
		},
	}

	queue, err := taskqueue.New(valkeyClient, "my-tasks", executor)
	require.NoError(t, err)

	require.Equal(t, "taskqueue:my-tasks", queue.Name())
}

func TestQueueOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupTestQueue(ctx, t)

	var processedCount atomic.Int32

	executor := &mockExecutor{
		fn: func(_ context.Context, _ string) error {
			processedCount.Add(1)

			return nil
		},
	}

	queue, err := taskqueue.New(
		valkeyClient,
		"test:options",
		executor,
		taskqueue.WithWorkerCount(2),
		taskqueue.WithBufferSize(50),
		taskqueue.WithExecTimeout(10*time.Second),
		taskqueue.WithPollInterval(500*time.Millisecond),
	)
	require.NoError(t, err)
	require.NotNil(t, queue)

	err = queue.Push(ctx, "test-task")
	require.NoError(t, err)

	err = queue.Start(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return processedCount.Load() == 1
	}, 5*time.Second, 100*time.Millisecond)

	err = queue.Stop()
	require.NoError(t, err)
}
