// Package runner provides an application lifecycle manager for coordinated startup and graceful shutdown.
//
// Services are registered in two tiers: infrastructure (database, cache, queue) and core (API handlers).
// Infrastructure services start first; if any fail, the application exits immediately. Core services start
// afterward. On shutdown, core services stop first, then infrastructure. The entire shutdown is bounded
// by a configurable timeout (default 30 seconds). Service panics are caught and logged.
//
// Basic usage:
//
//	r := runner.New(
//	    runner.WithInfrastructureService(dbService),
//	    runner.WithInfrastructureService(cacheService),
//	    runner.WithCoreService(apiServer),
//	    runner.WithShutdownTimeout(30*time.Second),
//	)
//	r.Run() // Blocks until SIGINT/SIGTERM; calls os.Exit(1) on error
//
// Use RunContext instead of Run to supply your own context and receive an
// error instead of exiting the process (e.g. for custom signal handling).
//
// Each service must implement the Service interface (Start, Stop, Name).
// Startup failures in any tier abort the application immediately.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultShutdownTimeout    = 30 * time.Second
	defaultStartupGracePeriod = 5 * time.Second
)

var (
	ErrServicePanic   = errors.New("runner: service panicked")
	ErrServiceFailed  = errors.New("runner: service failed to start")
	ErrShutdownTimout = errors.New("runner: shutdown timeout exceeded")
)

type Service interface {
	Start(ctx context.Context) error
	Stop() error
	Name() string
}

type Runner struct {
	coreServices           []Service
	infrastructureServices []Service
	shutdownTimeout        time.Duration
	startupGracePeriod     time.Duration
}

type Option func(*Runner)

func New(opts ...Option) *Runner {
	runner := &Runner{
		coreServices:           make([]Service, 0),
		infrastructureServices: make([]Service, 0),
		shutdownTimeout:        defaultShutdownTimeout,
		startupGracePeriod:     defaultStartupGracePeriod,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

func WithCoreService(svc Service) Option {
	return func(r *Runner) {
		r.coreServices = append(r.coreServices, svc)
		log.Info().
			Str("source", "gframework").
			Str("service_type", "core").
			Str("service_name", svc.Name()).
			Msg("Core service registered")
	}
}

func WithInfrastructureService(svc Service) Option {
	return func(r *Runner) {
		r.infrastructureServices = append(r.infrastructureServices, svc)
		log.Info().
			Str("source", "gframework").
			Str("service_type", "infrastructure").
			Str("service_name", svc.Name()).
			Msg("Infrastructure service registered")
	}
}

func WithShutdownTimeout(d time.Duration) Option {
	return func(r *Runner) {
		r.shutdownTimeout = d
	}
}

func WithStartupGracePeriod(d time.Duration) Option {
	return func(r *Runner) {
		r.startupGracePeriod = d
	}
}

func (r *Runner) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := r.RunContext(ctx); err != nil {
		os.Exit(1)
	}

	log.Info().Str("source", "gframework").Msg("Graceful shutdown completed")
}

// RunContext starts all registered services and blocks until ctx is cancelled
// or any service fails. Either event triggers a graceful shutdown (core
// services first, then infrastructure). It returns nil on a clean
// cancellation-triggered shutdown and the service failure otherwise.
//
// Most applications should use Run, which wires SIGINT/SIGTERM handling and
// exits the process on failure. RunContext is for callers that manage their
// own signals or exit codes, and for tests.
func (r *Runner) RunContext(ctx context.Context) error {
	infraErrCh := make(chan error, len(r.infrastructureServices))
	coreErrCh := make(chan error, len(r.coreServices))

	log.Info().Str("source", "gframework").Msg("Starting infrastructure services")

	if err := r.startServices(ctx, r.infrastructureServices, infraErrCh); err != nil {
		log.Error().Str("source", "gframework").Err(err).Msg("Infrastructure services failed to start")
		r.shutdownWithTimeout(r.infrastructureServices)

		return err
	}

	log.Info().Str("source", "gframework").Msg("Starting core services")

	if err := r.startServices(ctx, r.coreServices, coreErrCh); err != nil {
		log.Error().Str("source", "gframework").Err(err).Msg("Core services failed to start")
		r.shutdownWithTimeout(r.coreServices)
		r.shutdownWithTimeout(r.infrastructureServices)

		return err
	}

	log.Info().
		Str("source", "gframework").
		Int("pid", os.Getpid()).
		Int("core_services", len(r.coreServices)).
		Int("infra_services", len(r.infrastructureServices)).
		Msg("All services started, waiting for shutdown signal")

	var runErr error

	select {
	case <-ctx.Done():
		log.Warn().Str("source", "gframework").Msg("Shutdown signal received")
	case runErr = <-infraErrCh:
		log.Error().Str("source", "gframework").Err(runErr).Msg("Infrastructure service failed, shutting down")
	case runErr = <-coreErrCh:
		log.Error().Str("source", "gframework").Err(runErr).Msg("Core service failed, shutting down")
	}

	r.shutdownWithTimeout(r.coreServices)
	r.shutdownWithTimeout(r.infrastructureServices)

	return runErr
}

func (r *Runner) startServices(ctx context.Context, services []Service, errCh chan error) error {
	if len(services) == 0 {
		return nil
	}

	for _, svc := range services {
		go func(service Service) {
			defer func() {
				if rec := recover(); rec != nil {
					errCh <- fmt.Errorf("%w: %s: %v", ErrServicePanic, service.Name(), rec)
				}
			}()

			log.Info().Str("source", "gframework").Str("service_name", service.Name()).Msg("Starting service")

			if err := service.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("%w: %s: %w", ErrServiceFailed, service.Name(), err)
			}
		}(svc)
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.startupGracePeriod):
		return nil
	}
}

func (r *Runner) shutdownWithTimeout(services []Service) {
	if len(services) == 0 {
		return
	}

	done := make(chan struct{})

	go func() {
		r.concurrentStop(services)
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(r.shutdownTimeout):
		log.Error().
			Str("source", "gframework").
			Dur("timeout", r.shutdownTimeout).
			Msg("Shutdown timeout exceeded, some services may not have stopped cleanly")
	}
}

func (r *Runner) concurrentStop(services []Service) {
	var wg sync.WaitGroup

	for _, svc := range services {
		wg.Add(1)

		go func(service Service) {
			defer wg.Done()

			log.Info().Str("source", "gframework").Str("service_name", service.Name()).Msg("Stopping service")

			if err := service.Stop(); err != nil {
				log.Error().
					Str("source", "gframework").
					Err(err).
					Str("service_name", service.Name()).
					Msg("Service failed to stop")
			} else {
				log.Info().
					Str("source", "gframework").
					Str("service_name", service.Name()).
					Msg("Service stopped")
			}
		}(svc)
	}

	wg.Wait()
}
