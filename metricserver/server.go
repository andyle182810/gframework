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

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog/log"
)

const (
	metricsPath = "/metrics"
	statusPath  = "/status"
)

type Config struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	GracePeriod  time.Duration
}

type Server struct {
	gracePeriod  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	address      string
	echo         *echo.Echo
	httpServer   *http.Server
}

func New(cfg *Config) *Server {
	ech := echo.New()

	ech.GET(statusPath, func(ctx *echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})

	ech.GET(metricsPath, echoprometheus.NewHandler())

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{ //nolint:exhaustruct
		gracePeriod:  cfg.GracePeriod,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		address:      address,
		echo:         ech,
	}
}

func (s *Server) Start(_ context.Context) error {
	s.httpServer = &http.Server{ //nolint:exhaustruct
		Addr:         s.address,
		Handler:      s.echo,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	log.Info().
		Str("source", "gframework").
		Str("address", s.address).
		Msg("The metrics server is being started")

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Str("source", "gframework").Err(err).Msg("Metrics server failed to start")
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
