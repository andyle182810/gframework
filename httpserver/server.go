package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/validator"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Host         string
	Port         int
	EnableCors   bool
	BodyLimit    string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	GracePeriod  time.Duration
}

type Server struct {
	address     string
	gracePeriod time.Duration
	Echo        *echo.Echo
	Root        *echo.Group
}

func New(cfg *Config) *Server {
	echo := echo.New()
	echo.Server.ReadTimeout = cfg.ReadTimeout
	echo.Server.WriteTimeout = cfg.WriteTimeout
	echo.HidePort = true
	echo.HideBanner = true
	echo.Validator = validator.DefaultRestValidator()
	echo.HTTPErrorHandler = middleware.ErrorHandler(echo.DefaultHTTPErrorHandler)

	echo.Pre(middleware.RequestLogger(log.Logger, RestLogFieldsExtractor))
	echo.Pre(echomiddleware.BodyLimit(cfg.BodyLimit))

	if cfg.EnableCors {
		echo.Use(echomiddleware.CORS())
	}

	root := echo.Group("")
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{
		gracePeriod: cfg.GracePeriod,
		address:     address,
		Echo:        echo,
		Root:        root,
	}
}

func (s *Server) Run() {
	log.Info().Str("address", s.address).Msg("Starting HTTP server")

	if err := s.Echo.Start(s.address); !errors.Is(err, http.ErrServerClosed) {
		log.Panic().Err(err).Msg("HTTP server encountered a fatal error")
	}
}

func (s *Server) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.gracePeriod)
	defer cancel()

	log.Info().Msg("Initiating graceful shutdown of HTTP server")

	if err := s.Echo.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to gracefully shut down HTTP server")

		return err
	}

	return nil
}

func (s *Server) Name() string {
	return "http"
}

func RestLogFieldsExtractor(ctx echo.Context) map[string]any {
	if req := ctx.Get(middleware.ContextKeyBody); req != nil {
		var requestPayload string

		if b, err := json.Marshal(req); err != nil {
			requestPayload = fmt.Sprintf("failed to parse request object as string: %+v", err)
		} else {
			requestPayload = string(b)
		}

		return map[string]any{"request_payload": requestPayload}
	}

	return nil
}

func RequestIDSkipper(skip bool) echomiddleware.Skipper {
	return func(_ echo.Context) bool {
		return skip
	}
}
