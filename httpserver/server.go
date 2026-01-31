package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/validator"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog/log"
)

const (
	kilobyte         = 1 << 10
	megabyte         = 1 << 20
	gigabyte         = 1 << 30
	defaultBodyLimit = 10 * megabyte
)

type Config struct {
	Host         string
	Port         int
	EnableCors   bool
	AllowOrigins []string
	BodyLimit    string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	GracePeriod  time.Duration
}

type Server struct {
	address      string
	gracePeriod  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	Echo         *echo.Echo
	Root         *echo.Group
	httpServer   *http.Server
}

func New(cfg *Config) *Server {
	e := echo.New()
	e.Validator = validator.DefaultRestValidator()
	e.HTTPErrorHandler = middleware.ErrorHandler(echo.DefaultHTTPErrorHandler(false))

	e.Pre(middleware.RequestLogger(log.Logger, SafeLogFieldsExtractor))
	e.Pre(echomiddleware.BodyLimit(parseBodyLimit(cfg.BodyLimit)))

	if cfg.EnableCors {
		e.Use(echomiddleware.CORS(cfg.AllowOrigins...))
	}

	root := e.Group("")
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{ //nolint:exhaustruct
		gracePeriod:  cfg.GracePeriod,
		address:      address,
		Echo:         e,
		Root:         root,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
	}
}

func parseBodyLimit(limit string) int64 {
	if limit == "" {
		return defaultBodyLimit
	}

	multiplier := int64(1)
	unit := limit[len(limit)-1:]

	switch unit {
	case "K", "k":
		multiplier = kilobyte
		limit = limit[:len(limit)-1]
	case "M", "m":
		multiplier = megabyte
		limit = limit[:len(limit)-1]
	case "G", "g":
		multiplier = gigabyte
		limit = limit[:len(limit)-1]
	}

	size, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return defaultBodyLimit
	}

	return size * multiplier
}

func (s *Server) Start(_ context.Context) error {
	s.httpServer = &http.Server{ //nolint:exhaustruct
		Addr:         s.address,
		Handler:      s.Echo,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	log.Info().
		Str("address", s.address).
		Msg("The HTTP server is being started")

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("HTTP server failed to start")
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	log.Info().
		Msg("The graceful shutdown of HTTP server is being initiated")

	if s.httpServer == nil {
		return errors.New("HTTP server is not running") //nolint:err113
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.gracePeriod)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to gracefully stop HTTP server")

		return fmt.Errorf("failed to stop HTTP server: %w", err)
	}

	log.Info().
		Msg("The HTTP server shutdown has been completed successfully")

	return nil
}

func (s *Server) Name() string {
	return "http"
}

func SafeLogFieldsExtractor(ctx *echo.Context) map[string]any {
	fields := make(map[string]any)

	if req := ctx.Get(middleware.ContextKeyBody); req != nil {
		// Only log that a body exists and its type, not the actual content
		fields["has_body"] = true
		fields["body_type"] = fmt.Sprintf("%T", req)
	} else {
		fields["has_body"] = false
	}

	return fields
}

func RequestIDSkipper(skip bool) echomiddleware.Skipper {
	return func(_ *echo.Context) bool {
		return skip
	}
}
