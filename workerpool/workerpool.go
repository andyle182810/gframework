package workerpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var ErrAlreadyRunning = errors.New("worker pool is already running")

type Executor interface {
	Execute(ctx context.Context) error
}

type WorkerPool struct {
	name         string
	executor     Executor
	workerCount  int
	tickInterval time.Duration
	execTimeout  time.Duration
	jobChan      chan struct{}
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
}

type Option func(*WorkerPool)

func New(executor Executor, opts ...Option) *WorkerPool {
	pool := &WorkerPool{
		name:         "worker-pool",
		executor:     executor,
		workerCount:  1,
		tickInterval: time.Second,
		execTimeout:  0,
		jobChan:      nil,
		cancel:       nil,
		wg:           sync.WaitGroup{},
		mu:           sync.Mutex{},
		running:      false,
	}

	for _, opt := range opts {
		opt(pool)
	}

	return pool
}

func WithWorkerCount(count int) Option {
	return func(pool *WorkerPool) {
		if count > 0 {
			pool.workerCount = count
		}
	}
}

func WithTickInterval(duration time.Duration) Option {
	return func(pool *WorkerPool) {
		if duration > 0 {
			pool.tickInterval = duration
		}
	}
}

func WithExecutionTimeout(timeout time.Duration) Option {
	return func(pool *WorkerPool) {
		if timeout > 0 {
			pool.execTimeout = timeout
		}
	}
}

func WithName(name string) Option {
	return func(pool *WorkerPool) {
		if name != "" {
			pool.name = name
		}
	}
}

func (pool *WorkerPool) Name() string {
	return pool.name
}

func (pool *WorkerPool) Start(ctx context.Context) error {
	pool.mu.Lock()
	if pool.running {
		pool.mu.Unlock()

		return ErrAlreadyRunning
	}

	pool.running = true
	pool.jobChan = make(chan struct{})

	workerCtx, cancel := context.WithCancel(ctx)
	pool.cancel = cancel
	pool.mu.Unlock()

	log.Info().
		Int("worker_count", pool.workerCount).
		Dur("tick_interval", pool.tickInterval).
		Dur("exec_timeout", pool.execTimeout).
		Msg("Worker pool is starting.")

	for workerID := range pool.workerCount {
		pool.wg.Add(1)

		go pool.worker(workerCtx, workerID)
	}

	pool.wg.Add(1)

	go pool.dispatcher(workerCtx)

	return nil
}

func (pool *WorkerPool) Stop() error {
	pool.mu.Lock()
	if !pool.running {
		pool.mu.Unlock()

		return nil
	}

	pool.running = false
	pool.mu.Unlock()

	log.Info().Msg("Worker pool is stopping.")

	if pool.cancel != nil {
		pool.cancel()
	}

	pool.wg.Wait()

	log.Info().Msg("Worker pool has stopped.")

	return nil
}

func (pool *WorkerPool) dispatcher(ctx context.Context) {
	defer pool.wg.Done()

	ticker := time.NewTicker(pool.tickInterval)
	defer ticker.Stop()

	log.Info().Msg("Dispatcher has started.")

	for {
		select {
		case <-ctx.Done():
			close(pool.jobChan)

			log.Info().Msg("Dispatcher is shutting down.")

			return
		case <-ticker.C:
			select {
			case pool.jobChan <- struct{}{}:
			case <-ctx.Done():
				close(pool.jobChan)

				log.Info().Msg("Dispatcher is shutting down.")

				return
			}
		}
	}
}

func (pool *WorkerPool) worker(ctx context.Context, id int) {
	defer pool.wg.Done()

	log.Info().
		Int("worker_id", id).
		Msg("Worker has started.")

	for {
		select {
		case <-ctx.Done():
			log.Info().
				Int("worker_id", id).
				Msg("Worker is shutting down.")

			return
		case _, ok := <-pool.jobChan:
			if !ok {
				log.Info().
					Int("worker_id", id).
					Msg("Worker is shutting down.")

				return
			}

			pool.executeWithTimeout(ctx, id)
		}
	}
}

func (pool *WorkerPool) executeWithTimeout(ctx context.Context, workerID int) {
	var execCtx context.Context

	var cancel context.CancelFunc

	if pool.execTimeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, pool.execTimeout)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}

	defer cancel()

	log.Debug().
		Int("worker_id", workerID).
		Dur("timeout", pool.execTimeout).
		Msg("Starting execution for worker.")

	err := pool.executor.Execute(execCtx)
	if err != nil {
		log.Error().
			Err(err).
			Int("worker_id", workerID).
			Msg("Executor failed.")
	}
}
