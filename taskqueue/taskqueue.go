package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	defaultWorkerCount  = 10
	defaultBufferSize   = 100
	defaultExecTimeout  = 30 * time.Second
	defaultPollInterval = time.Second
)

var (
	ErrQueueAlreadyRunning = errors.New("taskqueue: queue is already running")
	ErrQueueNotRunning     = errors.New("taskqueue: queue is not running")
	ErrNilClient           = errors.New("taskqueue: redis client is nil")
	ErrNilExecutor         = errors.New("taskqueue: task executor is nil")
	ErrEmptyQueueKey       = errors.New("taskqueue: queue key is empty")
	ErrMaxAgeTooSmall      = errors.New("taskqueue: maxAge must be greater than execTimeout")
)

type Payload []byte

type Task struct {
	ID      string
	Payload Payload
}

type Executor interface {
	Execute(ctx context.Context, taskID string, payload Payload) error
}

type taskItem struct {
	id      string
	payload Payload
}

type Queue struct {
	client        redis.UniversalClient
	queueKey      string
	processingKey string
	payloadKey    string
	executor      Executor
	workerCount   int
	bufferSize    int
	execTimeout   time.Duration
	pollInterval  time.Duration
	taskChan      chan taskItem
	wg            sync.WaitGroup
	cancel        context.CancelFunc
	mu            sync.Mutex
	running       atomic.Bool
}

type Option func(*Queue)

func New(client redis.UniversalClient, queueKey string, executor Executor, opts ...Option) (*Queue, error) {
	if client == nil {
		return nil, ErrNilClient
	}

	if queueKey == "" {
		return nil, ErrEmptyQueueKey
	}

	if executor == nil {
		return nil, ErrNilExecutor
	}

	queue := &Queue{ //nolint:exhaustruct
		client:        client,
		queueKey:      queueKey,
		processingKey: queueKey + ":processing",
		payloadKey:    queueKey + ":payloads",
		executor:      executor,
		workerCount:   defaultWorkerCount,
		bufferSize:    defaultBufferSize,
		execTimeout:   defaultExecTimeout,
		pollInterval:  defaultPollInterval,
		taskChan:      make(chan taskItem, defaultBufferSize),
		wg:            sync.WaitGroup{},
		cancel:        nil,
		mu:            sync.Mutex{},
	}

	for _, opt := range opts {
		opt(queue)
	}

	return queue, nil
}

func WithWorkerCount(count int) Option {
	return func(q *Queue) {
		if count > 0 {
			q.workerCount = count
		}
	}
}

func WithBufferSize(size int) Option {
	return func(q *Queue) {
		if size > 0 {
			q.bufferSize = size
			q.taskChan = make(chan taskItem, size)
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(q *Queue) {
		if timeout > 0 {
			q.execTimeout = timeout
		}
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(q *Queue) {
		if interval > 0 {
			q.pollInterval = interval
		}
	}
}

func (q *Queue) Push(ctx context.Context, tasks ...Task) error {
	if len(tasks) == 0 {
		return nil
	}

	_, err := q.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		queueArgs := make([]any, len(tasks))
		for i, task := range tasks {
			queueArgs[i] = task.ID
		}

		pipe.LPush(ctx, q.queueKey, queueArgs...)

		for _, task := range tasks {
			if len(task.Payload) > 0 {
				pipe.HSet(ctx, q.payloadKey, task.ID, []byte(task.Payload))
			}
		}

		return nil
	})

	return err
}

func (q *Queue) Start(ctx context.Context) error {
	if !q.running.CompareAndSwap(false, true) {
		return ErrQueueAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	q.cancel = cancel

	for i := range q.workerCount {
		q.wg.Add(1)

		go q.worker(ctx, i)
	}

	q.wg.Add(1)

	go q.fetcher(ctx)

	log.Info().
		Int("workers", q.workerCount).
		Str("queue", q.queueKey).
		Dur("exec_timeout", q.execTimeout).
		Msg("Task queue started")

	return nil
}

func (q *Queue) Stop() error {
	if !q.running.CompareAndSwap(true, false) {
		return nil
	}

	log.Info().Str("queue", q.queueKey).Msg("Task queue stopping")

	if q.cancel != nil {
		q.cancel()
	}

	q.wg.Wait()

	log.Info().Str("queue", q.queueKey).Msg("Task queue stopped")

	return nil
}

func (q *Queue) fetcher(ctx context.Context) {
	defer q.wg.Done()
	defer close(q.taskChan)

	log.Debug().Str("queue", q.queueKey).Msg("Fetcher started")

	for {
		select {
		case <-ctx.Done():
			log.Debug().Str("queue", q.queueKey).Msg("Fetcher stopping")

			return
		default:
			task, err := q.fetchTask(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				if !errors.Is(err, redis.Nil) {
					log.Error().Err(err).Msg("Failed to fetch task")
					time.Sleep(q.pollInterval)
				}

				continue
			}

			select {
			case q.taskChan <- task:
			case <-ctx.Done():
				// Use a fresh context since the parent is cancelled but we need to return the task
				returnCtx := context.WithoutCancel(ctx)
				q.returnTask(returnCtx, task.id)

				return
			}
		}
	}
}

func (q *Queue) fetchTask(ctx context.Context) (taskItem, error) {
	result, err := q.client.BRPop(ctx, q.pollInterval, q.queueKey).Result()
	if err != nil {
		return taskItem{}, err
	}

	taskID := result[1] // result[0] is the key name

	// Fetch payload from hash (if exists)
	payloadBytes, err := q.client.HGet(ctx, q.payloadKey, taskID).Bytes()
	if err != nil && !errors.Is(err, redis.Nil) {
		q.returnTask(ctx, taskID)

		return taskItem{}, fmt.Errorf("failed to get payload: %w", err)
	}

	now := float64(time.Now().Unix())

	err = q.client.ZAdd(ctx, q.processingKey, redis.Z{
		Score:  now,
		Member: taskID,
	}).Err()
	if err != nil {
		q.returnTask(ctx, taskID)

		return taskItem{}, fmt.Errorf("failed to add task to processing set: %w", err)
	}

	return taskItem{
		id:      taskID,
		payload: payloadBytes,
	}, nil
}

func (q *Queue) returnTask(ctx context.Context, taskID string) {
	if err := q.client.LPush(ctx, q.queueKey, taskID).Err(); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to return task to queue")
	}
}

func (q *Queue) worker(ctx context.Context, id int) {
	defer q.wg.Done()

	log.Debug().Int("worker_id", id).Msg("Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker stopping")

			return
		case task, ok := <-q.taskChan:
			if !ok {
				log.Debug().Int("worker_id", id).Msg("Worker stopping - channel closed")

				return
			}

			q.processTask(ctx, id, task)
		}
	}
}

func (q *Queue) processTask(ctx context.Context, workerID int, task taskItem) {
	log.Debug().
		Int("worker_id", workerID).
		Str("task_id", task.id).
		Msg("Processing task")

	execCtx, cancel := context.WithTimeout(ctx, q.execTimeout)
	defer cancel()

	err := q.executor.Execute(execCtx, task.id, task.payload)

	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			log.Error().
				Int("worker_id", workerID).
				Str("task_id", task.id).
				Dur("timeout", q.execTimeout).
				Msg("Task timed out")
		} else {
			log.Error().
				Err(err).
				Int("worker_id", workerID).
				Str("task_id", task.id).
				Msg("Task failed")
		}
	} else {
		log.Debug().
			Int("worker_id", workerID).
			Str("task_id", task.id).
			Msg("Task completed successfully")
	}

	// Clean up: remove from processing set and delete payload
	if err := q.client.ZRem(ctx, q.processingKey, task.id).Err(); err != nil {
		log.Error().Err(err).Str("task_id", task.id).Msg("Failed to remove task from processing set")
	}

	if len(task.payload) > 0 {
		if err := q.client.HDel(ctx, q.payloadKey, task.id).Err(); err != nil {
			log.Error().Err(err).Str("task_id", task.id).Msg("Failed to delete payload")
		}
	}
}

func (q *Queue) QueueLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.queueKey).Result()
}

func (q *Queue) ProcessingCount(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, q.processingKey).Result()
}

func (q *Queue) RecoverStale(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= q.execTimeout {
		return 0, ErrMaxAgeTooSmall
	}

	cutoff := float64(time.Now().Add(-maxAge).Unix())

	staleTasks, err := q.client.ZRangeByScore(ctx, q.processingKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%f", cutoff),
		Offset: 0,
		Count:  0,
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get stale tasks: %w", err)
	}

	recovered := 0

	for _, taskID := range staleTasks {
		_, err := q.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.ZRem(ctx, q.processingKey, taskID)
			pipe.LPush(ctx, q.queueKey, taskID)

			return nil
		})
		if err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("Failed to recover stale task")

			continue
		}

		log.Warn().Str("task_id", taskID).Msg("Recovered stale task")

		recovered++
	}

	return recovered, nil
}

func (q *Queue) Name() string {
	return "taskqueue:" + q.queueKey
}
