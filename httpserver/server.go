// Package httpserver provides an opinionated HTTP server wrapper around Echo v5 with integrated middleware.
//
// The server automatically configures request logging, body size limiting, CORS (optional), validation,
// and error handling. It supports graceful shutdown and is production-ready.
//
// Basic usage:
//
//	server := httpserver.New(&httpserver.Config{
//	    Host:      "0.0.0.0",
//	    Port:      8080,
//	    BodyLimit: "10M",
//	})
//
//	server.Echo.GET("/api/users", listUsersHandler)
//	if err := server.Start(ctx); err != nil {
//	    return err
//	}
//
// The underlying Echo instance is accessible via server.Echo for route registration and custom middleware.
// The server implements graceful shutdown with a configurable GracePeriod (default 15s).
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
	"github.com/andyle182810/gframework/transformer"
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

	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultGracePeriod       = 15 * time.Second
)

type Config struct {
	Host              string
	Port              int
	EnableCors        bool
	AllowOrigins      []string
	BodyLimit         string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	GracePeriod       time.Duration
	Transformer       *transformer.Transformer
	DisableTransform  bool
	DisableMetrics    bool
	LogRequestBody    bool
}

type Server struct {
	address           string
	gracePeriod       time.Duration
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	Echo              *echo.Echo
	Root              *echo.Group
	httpServer        *http.Server
	listener          net.Listener
}

func New(cfg *Config) *Server {
	e := echo.New()
	e.Validator = validator.DefaultRestValidator()
	e.HTTPErrorHandler = middleware.ErrorHandler(echo.DefaultHTTPErrorHandler(false))

	tfm := cfg.Transformer
	if tfm == nil {
		tfm = transformer.DefaultRestTransformer()
	}

	logExtractors := []middleware.LogFieldExtractor{safeLogFieldsExtractor}

	if cfg.LogRequestBody {
		logExtractors = append(logExtractors, scrubbedBodyLogFieldExtractor(tfm))
	}

	e.Pre(middleware.RequestLogger(log.Logger, logExtractors...))
	e.Pre(echomiddleware.BodyLimit(parseBodyLimit(cfg.BodyLimit)))

	if !cfg.DisableMetrics {
		e.Use(middleware.HTTPMetrics())
	}

	if !cfg.DisableTransform {
		e.Use(injectTransformer(tfm))
	}

	if cfg.EnableCors {
		if len(cfg.AllowOrigins) == 0 {
			log.Warn().
				Str("source", "gframework").
				Msg("CORS is enabled without AllowOrigins; all origins are allowed — set AllowOrigins in production")
		}

		e.Use(echomiddleware.CORS(cfg.AllowOrigins...))
	}

	root := e.Group("")
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{ //nolint:exhaustruct
		gracePeriod:       defaultDuration(cfg.GracePeriod, DefaultGracePeriod),
		address:           address,
		Echo:              e,
		Root:              root,
		readHeaderTimeout: defaultDuration(cfg.ReadHeaderTimeout, DefaultReadHeaderTimeout),
		readTimeout:       defaultDuration(cfg.ReadTimeout, DefaultReadTimeout),
		writeTimeout:      defaultDuration(cfg.WriteTimeout, DefaultWriteTimeout),
		idleTimeout:       defaultDuration(cfg.IdleTimeout, DefaultIdleTimeout),
	}
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}

	return value
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

func (s *Server) Start(ctx context.Context) error {
	s.httpServer = &http.Server{ //nolint:exhaustruct
		Addr:              s.address,
		Handler:           s.Echo,
		ReadHeaderTimeout: s.readHeaderTimeout,
		ReadTimeout:       s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       s.idleTimeout,
	}

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server to %s: %w", s.address, err)
	}

	s.listener = listener

	log.Info().
		Str("source", "gframework").
		Str("address", listener.Addr().String()).
		Msg("The HTTP server is being started")

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Str("source", "gframework").Err(err).Msg("HTTP server terminated unexpectedly")
		}
	}()

	return nil
}

func (s *Server) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}

	return s.address
}

func (s *Server) Stop() error {
	log.Info().
		Str("source", "gframework").
		Msg("The graceful shutdown of HTTP server is being initiated")

	if s.httpServer == nil {
		return errors.New("HTTP server is not running") //nolint:err113
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.gracePeriod)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Error().Str("source", "gframework").Err(err).Msg("Failed to gracefully stop HTTP server")

		return fmt.Errorf("failed to stop HTTP server: %w", err)
	}

	log.Info().
		Str("source", "gframework").
		Msg("The HTTP server shutdown has been completed successfully")

	return nil
}

func (s *Server) Name() string {
	return "http"
}

func safeLogFieldsExtractor(ctx *echo.Context) map[string]any {
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
