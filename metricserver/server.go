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

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Info().
			Str("address", s.address).
			Msg("The metrics server is being started.")

		if err := s.echo.Start(s.address); !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server failed: %w", err)
		} else {
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info().
			Msg("The metrics server Run() context has been cancelled.")

		return ctx.Err()
	}
}

func (s *Server) Stop() error {
	shutdownCtx, cancel := context.WithTimeout(context.TODO(), s.gracePeriod)
	defer cancel()

	log.Info().
		Msg("The graceful shutdown of metrics server is being initiated.")

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		log.Error().
			Err(err).
			Msg("The metrics server failed to shut down gracefully.")

		return err
	}

	log.Info().
		Msg("The metrics server shutdown has been completed successfully.")

	return nil
}

func (s *Server) Name() string {
	return "metric"
}
