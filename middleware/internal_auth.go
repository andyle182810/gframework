package middleware

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

var (
	ErrInternalTokenRequired   = echo.NewHTTPError(http.StatusUnauthorized, "internal authorization is required")
	ErrInternalTokenInvalid    = echo.NewHTTPError(http.StatusForbidden, "internal authorization is invalid")
	ErrInternalClientForbidden = echo.NewHTTPError(http.StatusForbidden, "internal caller is not allowed")
)

type InternalAuthConfig struct {
	Skipper        middleware.Skipper
	Keyfunc        keyfunc.Keyfunc
	Header         string
	AllowedClients []string

	// ValidMethods lists the accepted JWT signing algorithms. Empty means
	// DefaultValidMethods(). Never include HS* methods when keys come from a JWKS.
	ValidMethods []string

	// Issuer, when non-empty, requires the token "iss" claim to match exactly.
	Issuer string

	// Leeway is the clock-skew tolerance for time-based claims.
	// Zero means DefaultJWTLeeway.
	Leeway time.Duration
}

func DefaultInternalAuthConfig() InternalAuthConfig {
	return InternalAuthConfig{
		Skipper:        middleware.DefaultSkipper,
		Keyfunc:        nil,
		Header:         HeaderXInternalAuthorization,
		AllowedClients: nil,
		ValidMethods:   DefaultValidMethods(),
		Issuer:         "",
		Leeway:         DefaultJWTLeeway,
	}
}

func InternalAuth(kf keyfunc.Keyfunc, allowedClients []string) echo.MiddlewareFunc {
	config := DefaultInternalAuthConfig()
	config.Keyfunc = kf
	config.AllowedClients = allowedClients

	return InternalAuthWithConfig(config)
}

func InternalAuthWithConfig(config InternalAuthConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.Header == "" {
		config.Header = HeaderXInternalAuthorization
	}

	if len(config.ValidMethods) == 0 {
		config.ValidMethods = DefaultValidMethods()
	}

	if config.Leeway == 0 {
		config.Leeway = DefaultJWTLeeway
	}

	parserOpts := buildParserOptions(config.ValidMethods, config.Issuer, nil, config.Leeway)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if config.Skipper(ctx) {
				return next(ctx)
			}

			raw := stripBearer(ctx.Request().Header.Get(config.Header))
			if raw == "" {
				return ErrInternalTokenRequired
			}

			claims := &ExtendedClaims{} //nolint:exhaustruct

			token, err := jwt.ParseWithClaims(raw, claims, config.Keyfunc.Keyfunc, parserOpts...)
			if err != nil || !token.Valid {
				return ErrInternalTokenInvalid
			}

			if !slices.Contains(config.AllowedClients, claims.Azp) {
				return ErrInternalClientForbidden
			}

			return next(ctx)
		}
	}
}

func stripBearer(header string) string {
	const prefix = "Bearer "

	if len(header) >= len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}

	return strings.TrimSpace(header)
}
