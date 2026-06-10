package middleware

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

var ErrInternalTokenMintFailed = echo.NewHTTPError(
	http.StatusBadGateway, "failed to mint internal authorization token",
)

type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

type InjectInternalTokenConfig struct {
	Skipper  middleware.Skipper
	Provider TokenProvider
	Header   string
}

func InjectInternalToken(provider TokenProvider, header string) echo.MiddlewareFunc {
	return InjectInternalTokenWithConfig(InjectInternalTokenConfig{
		Skipper:  middleware.DefaultSkipper,
		Provider: provider,
		Header:   header,
	})
}

func InjectInternalTokenWithConfig(config InjectInternalTokenConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.Header == "" {
		config.Header = HeaderXInternalAuthorization
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()
			req.Header.Del(config.Header)

			token, err := config.Provider.GetToken(req.Context())
			if err != nil {
				return ErrInternalTokenMintFailed
			}

			req.Header.Set(config.Header, "Bearer "+token)

			return next(ctx)
		}
	}
}
