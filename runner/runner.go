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

const defaultShutdownTimeout = 30 * time.Second

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
}

type Option func(*Runner)

func New(opts ...Option) *Runner {
	runner := &Runner{
		coreServices:           make([]Service, 0),
		infrastructureServices: make([]Service, 0),
		shutdownTimeout:        defaultShutdownTimeout,
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
			Str("service_type", "core").
			Str("service_name", svc.Name()).
			Msg("Core service registered")
	}
}

func WithInfrastructureService(svc Service) Option {
	return func(r *Runner) {
		r.infrastructureServices = append(r.infrastructureServices, svc)
		log.Info().
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

func (r *Runner) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Starting infrastructure services")

	if err := r.startServices(ctx, r.infrastructureServices); err != nil {
		log.Error().Err(err).Msg("Infrastructure services failed to start")
		r.shutdownWithTimeout(r.infrastructureServices)
		os.Exit(1)
	}

	log.Info().Msg("Starting core services")

	if err := r.startServices(ctx, r.coreServices); err != nil {
		log.Error().Err(err).Msg("Core services failed to start")
		r.shutdownWithTimeout(r.coreServices)
		r.shutdownWithTimeout(r.infrastructureServices)
		os.Exit(1)
	}

	log.Info().
		Int("pid", os.Getpid()).
		Int("core_services", len(r.coreServices)).
		Int("infra_services", len(r.infrastructureServices)).
		Msg("All services started, waiting for shutdown signal")

	<-ctx.Done()
	log.Warn().Msg("Shutdown signal received")

	r.shutdownWithTimeout(r.coreServices)
	r.shutdownWithTimeout(r.infrastructureServices)

	log.Info().Msg("Graceful shutdown completed")
}

func (r *Runner) startServices(ctx context.Context, services []Service) error {
	if len(services) == 0 {
		return nil
	}

	errCh := make(chan error, len(services))

	for _, svc := range services {
		go func(service Service) {
			defer func() {
				if rec := recover(); rec != nil {
					errCh <- fmt.Errorf("%w: %s: %v", ErrServicePanic, service.Name(), rec)
				}
			}()

			log.Info().Str("service_name", service.Name()).Msg("Starting service")

			if err := service.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("%w: %s: %w", ErrServiceFailed, service.Name(), err)
			}
		}(svc)
	}

	// Check for immediate startup failures.
	// Note: This only catches services that fail synchronously.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	default:
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

			log.Info().Str("service_name", service.Name()).Msg("Stopping service")

			if err := service.Stop(); err != nil {
				log.Error().
					Err(err).
					Str("service_name", service.Name()).
					Msg("Service failed to stop")
			} else {
				log.Info().
					Str("service_name", service.Name()).
					Msg("Service stopped")
			}
		}(svc)
	}

	wg.Wait()
}
