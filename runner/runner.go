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
	gracefulShutdownTimeout = 10 * time.Second
	startupCheckDelay       = 2 * time.Second
)

var (
	ErrServicePanic  = errors.New("service panicked")
	ErrServiceFailed = errors.New("service failed to start")
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
		shutdownTimeout:        gracefulShutdownTimeout,
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
			Msg("The core service has been registered successfully.")
	}
}

func WithInfrastructureService(svc Service) Option {
	return func(r *Runner) {
		r.infrastructureServices = append(r.infrastructureServices, svc)
		log.Info().
			Str("service_type", "infrastructure").
			Str("service_name", svc.Name()).
			Msg("The infrastructure service has been registered successfully.")
	}
}

func WithGracefulShutdownTimeout(d time.Duration) Option {
	return func(r *Runner) {
		r.shutdownTimeout = d
	}
}

func (r *Runner) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().
		Msg("The infrastructure services are being started.")

	if err := r.startServices(ctx, r.infrastructureServices); err != nil {
		log.Error().
			Err(err).
			Msg("The infrastructure services failed to start.")
		stop() // Ensure cleanup before exit
		os.Exit(1)
	}

	log.Info().
		Msg("The core services are being started.")

	if err := r.startServices(ctx, r.coreServices); err != nil {
		log.Error().
			Err(err).
			Msg("The core services failed to start.")
		stop()
		os.Exit(1)
	}

	log.Info().
		Int("pid", os.Getpid()).
		Msg("All services have been started successfully and are waiting for shutdown signal.")
	<-ctx.Done()
	log.Warn().
		Msg("The shutdown signal has been received.")

	// Stop core services first (stop accepting new work)
	r.concurrentStop(r.coreServices)
	r.concurrentStop(r.infrastructureServices)

	log.Info().
		Msg("The graceful shutdown has been completed successfully.")
}

func (r *Runner) startServices(ctx context.Context, services []Service) error {
	errCh := make(chan error, len(services))

	for _, svc := range services {
		service := svc
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					errCh <- fmt.Errorf("%w: service %s: %v", ErrServicePanic, service.Name(), rec)
				}
			}()

			log.Info().
				Str("service_name", service.Name()).
				Msg("The service is being started.")

			if err := service.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("%w: %s: %w", ErrServiceFailed, service.Name(), err)
			}
		}()
	}

	select {
	case err := <-errCh:
		return err
	case <-time.After(startupCheckDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) concurrentStop(services []Service) {
	var waitGroup sync.WaitGroup

	for _, svc := range services {
		waitGroup.Add(1)

		go func(service Service) {
			defer waitGroup.Done()

			log.Info().
				Str("service_name", service.Name()).
				Msg("An attempt is being made to stop the service.")

			if err := service.Stop(); err != nil {
				log.Error().
					Err(err).
					Str("service_name", service.Name()).
					Msg("The service failed to stop.")
			} else {
				log.Info().
					Str("service_name", service.Name()).
					Msg("The service has been stopped successfully.")
			}
		}(svc)
	}

	waitGroup.Wait()
}
