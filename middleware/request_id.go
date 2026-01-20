package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type RequestIDConfig struct {
	Skipper      middleware.Skipper
	Generator    func() string
	AutoGenerate bool
	Validator    func(string) error
}

func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Skipper:      middleware.DefaultSkipper,
		Generator:    uuid.NewString,
		AutoGenerate: false,
		Validator:    uuid.Validate,
	}
}

func RequestID(skipper middleware.Skipper) echo.MiddlewareFunc {
	config := DefaultRequestIDConfig()
	config.Skipper = skipper

	return RequestIDWithConfig(config)
}

func RequestIDWithConfig(config RequestIDConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.Generator == nil {
		config.Generator = uuid.NewString
	}

	if config.Validator == nil {
		config.Validator = uuid.Validate
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()
			rid := strings.TrimSpace(req.Header.Get(HeaderXRequestID))

			if rid == "" {
				if config.AutoGenerate {
					rid = config.Generator()
					req.Header.Set(HeaderXRequestID, rid)
					ctx.Response().Header().Set(HeaderXRequestID, rid)
				} else {
					return echo.NewHTTPError(
						http.StatusBadRequest,
						fmt.Sprint("missing required header: ", HeaderXRequestID),
					)
				}
			} else {
				if err := config.Validator(rid); err != nil {
					return echo.NewHTTPError(
						http.StatusBadRequest,
						fmt.Sprintf("invalid %s: must be a valid UUID", HeaderXRequestID),
					)
				}

				ctx.Response().Header().Set(HeaderXRequestID, rid)
			}

			ctx.Set(ContextKeyRequestID, rid)

			return next(ctx)
		}
	}
}
