package metricserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
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
	gracePeriod time.Duration
	address     string
	echo        *echo.Echo
}

func New(cfg *Config) *Server {
	ech := echo.New()
	ech.Server.ReadTimeout = cfg.ReadTimeout
	ech.Server.WriteTimeout = cfg.WriteTimeout
	ech.HideBanner = true
	ech.HidePort = true

	ech.GET(statusPath, func(ctx echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})

	ech.GET(metricsPath, echoprometheus.NewHandler())

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{
		gracePeriod: cfg.GracePeriod,
		address:     address,
		echo:        ech,
	}
}

func (s *Server) Run() {
	log.Info().Str("address", s.address).Msg("Starting metrics server")

	if err := s.echo.Start(s.address); !errors.Is(err, http.ErrServerClosed) {
		log.Panic().Err(err).Msg("Metrics server encountered a fatal error")
	}
}

func (s *Server) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.gracePeriod)
	defer cancel()

	log.Info().Msg("Initiating graceful shutdown of metrics server")

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Failed to gracefully shut down metrics server")

		return err
	}

	log.Info().Msg("Metrics server shutdown complete")

	return nil
}

func (s *Server) Name() string {
	return "metric"
}
