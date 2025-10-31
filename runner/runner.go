package runner

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	DefaultGracefulShutdownTimeout = 10 * time.Second
)

type Service interface {
	Run()
	Stop(ctx context.Context) error
	Name() string
}

type Runner struct {
	coreServices           []Service
	infrastructureServices []Service
	shutdownTimeout        time.Duration
}

type Option func(*Runner)

func New(opts ...Option) *Runner {
	r := &Runner{
		coreServices:           make([]Service, 0),
		infrastructureServices: make([]Service, 0),
		shutdownTimeout:        DefaultGracefulShutdownTimeout,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func WithCoreService(svc Service) Option {
	return func(r *Runner) {
		r.coreServices = append(r.coreServices, svc)
		log.Info().Str("service_type", "core").Str("service_name", svc.Name()).Msg("Service registered")
	}
}

func WithInfrastructureService(svc Service) Option {
	return func(r *Runner) {
		r.infrastructureServices = append(r.infrastructureServices, svc)
		log.Info().Str("service_type", "infrastructure").Str("service_name", svc.Name()).Msg("Service registered")
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

	allServices := append(r.coreServices, r.infrastructureServices...)

	for _, svc := range allServices {
		s := svc
		go func() {
			log.Info().Msgf("Starting service %s", s.Name())
			go s.Run()
		}()
	}

	log.Info().Int("PID", os.Getpid()).Msg("Runner waiting for shutdown signal...")
	<-ctx.Done()
	log.Warn().Msg("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), r.shutdownTimeout)
	defer cancel()

	r.concurrentStop(shutdownCtx, r.infrastructureServices)
	r.concurrentStop(shutdownCtx, r.coreServices)

	log.Info().Msg("Graceful shutdown complete")
}

func (r *Runner) concurrentStop(ctx context.Context, services []Service) {
	for _, svc := range services {
		log.Info().Str("service_name", svc.Name()).Msg("Attempting to stop service")

		if err := svc.Stop(ctx); err != nil {
			log.Error().Err(err).Str("service_name", svc.Name()).Msg("Service stop failed")
		} else {
			log.Info().Str("service_name", svc.Name()).Msg("Service stopped successfully")
		}
	}
}
