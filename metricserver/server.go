// Package metricserver provides a dedicated HTTP server for operational endpoints:
// a /status health-check and a /metrics Prometheus scrape endpoint.
//
// The server implements the runner.Service interface (Start, Stop, Name) and is designed to run
// independently from the main application server. This separation allows metrics and health to be
// monitored on a different port/interface from the API.
//
// Basic usage:
//
//	msrv := metricserver.New(&metricserver.Config{
//	    Host: "0.0.0.0",
//	    Port: 9090,
//	})
//	err := msrv.Start(ctx)
//	defer msrv.Stop()
//
//	// Endpoints:
//	// GET /status     returns {"status":"ok"}
//	// GET /metrics    returns Prometheus-formatted metrics
//
// The metrics endpoint integrates with Prometheus client via echoprometheus middleware.
package metricserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

const (
	metricsPath = "/metrics"
	statusPath  = "/status"

	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultGracePeriod       = 15 * time.Second
)

type Config struct {
	Host              string
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	GracePeriod       time.Duration
}

type Server struct {
	gracePeriod       time.Duration
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	address           string
	echo              *echo.Echo
	httpServer        *http.Server
}

func New(cfg *Config) *Server {
	ech := echo.New()

	ech.GET(statusPath, func(ctx *echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})

	metricsHandler := promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{DisableCompression: true}),
	)
	ech.GET(metricsPath, echo.WrapHandler(metricsHandler))

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{ //nolint:exhaustruct
		gracePeriod:       defaultDuration(cfg.GracePeriod, defaultGracePeriod),
		readHeaderTimeout: defaultDuration(cfg.ReadHeaderTimeout, defaultReadHeaderTimeout),
		readTimeout:       defaultDuration(cfg.ReadTimeout, defaultReadTimeout),
		writeTimeout:      defaultDuration(cfg.WriteTimeout, defaultWriteTimeout),
		idleTimeout:       defaultDuration(cfg.IdleTimeout, defaultIdleTimeout),
		address:           address,
		echo:              ech,
	}
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}

	return value
}

// Start binds the listener synchronously so bind failures are returned to the
// caller, then serves requests in a background goroutine.
func (s *Server) Start(ctx context.Context) error {
	s.httpServer = &http.Server{ //nolint:exhaustruct
		Addr:              s.address,
		Handler:           s.echo,
		ReadHeaderTimeout: s.readHeaderTimeout,
		ReadTimeout:       s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       s.idleTimeout,
	}

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to bind metrics server to %s: %w", s.address, err)
	}

	log.Info().
		Str("source", "gframework").
		Str("address", s.address).
		Msg("The metrics server is being started")

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Str("source", "gframework").Err(err).Msg("Metrics server terminated unexpectedly")
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	log.Info().
		Str("source", "gframework").
		Msg("The graceful shutdown of metrics server is being initiated")

	if s.httpServer == nil {
		return errors.New("metrics server is not running") //nolint:err113
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.gracePeriod)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Error().Str("source", "gframework").Err(err).Msg("Failed to gracefully stop metrics server")

		return fmt.Errorf("failed to stop metrics server: %w", err)
	}

	log.Info().
		Str("source", "gframework").
		Msg("The metrics server shutdown has been completed successfully")

	return nil
}

func (s *Server) Name() string {
	return "metric"
}
