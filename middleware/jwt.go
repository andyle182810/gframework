package middleware

import (
	"net/http"
	"slices"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	ErrTokenRequired   = echo.NewHTTPError(http.StatusUnauthorized, "Authorization header is required")
	ErrJWKSFetchFailed = echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch JWKS")
	ErrInvalidToken    = echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
)

type RoleAccess struct {
	Roles []string `json:"roles"`
}

type ResourceAccess map[string]RoleAccess

type ExtendedClaims struct {
	// Token metadata
	Typ string `json:"typ"`
	Azp string `json:"azp"`
	Sid string `json:"sid"`
	Acr string `json:"acr"`

	// Authorization
	Scope          string         `json:"scope"`
	RealmAccess    RoleAccess     `json:"realm_access"`    //nolint:tagliatelle
	ResourceAccess ResourceAccess `json:"resource_access"` //nolint:tagliatelle
	AllowedOrigins []string       `json:"allowed-origins"` //nolint:tagliatelle

	// User profile
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"` //nolint:tagliatelle
	GivenName         string `json:"given_name"`         //nolint:tagliatelle
	FamilyName        string `json:"family_name"`        //nolint:tagliatelle
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"` //nolint:tagliatelle

	jwt.RegisteredClaims
}

func (c *ExtendedClaims) GetAzp() string {
	return c.Azp
}

func (c *ExtendedClaims) HasRealmRole(role string) bool {
	return slices.Contains(c.RealmAccess.Roles, role)
}

func (c *ExtendedClaims) GetRealmRoles() []string {
	return c.RealmAccess.Roles
}

func (c *ExtendedClaims) HasResourceRole(resource, role string) bool {
	access, ok := c.ResourceAccess[resource]
	if !ok {
		return false
	}

	return slices.Contains(access.Roles, role)
}

func (c *ExtendedClaims) GetResourceRoles(resource string) []string {
	access, ok := c.ResourceAccess[resource]
	if !ok {
		return nil
	}

	return access.Roles
}

type JWTConfig struct {
	Skipper       middleware.Skipper
	Logger        *zerolog.Logger
	Keyfunc       keyfunc.Keyfunc
	NewClaimsFunc func(*echo.Context) jwt.Claims
	ContextKey    string
	TokenLookup   string
}

func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Skipper:       middleware.DefaultSkipper,
		Logger:        &log.Logger,
		Keyfunc:       nil,
		NewClaimsFunc: defaultNewClaimsFunc,
		ContextKey:    "user",
		TokenLookup:   "",
	}
}

func defaultNewClaimsFunc(_ *echo.Context) jwt.Claims { //nolint:ireturn
	return &ExtendedClaims{} //nolint:exhaustruct
}

func JWT(kf keyfunc.Keyfunc) echo.MiddlewareFunc {
	config := DefaultJWTConfig()
	config.Keyfunc = kf

	return JWTWithConfig(config)
}

func JWTWithConfig(config JWTConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.NewClaimsFunc == nil {
		config.NewClaimsFunc = defaultNewClaimsFunc
	}

	if config.ContextKey == "" {
		config.ContextKey = "user"
	}

	jwtConfig := buildJWTConfig(config)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if config.Skipper(ctx) {
				return next(ctx)
			}

			jwtMiddleware := echojwt.WithConfig(jwtConfig)
			handler := jwtMiddleware(next)

			return handler(ctx)
		}
	}
}

func buildJWTConfig(config JWTConfig) echojwt.Config {
	return echojwt.Config{
		Skipper:          nil,
		BeforeFunc:       nil,
		ContextKey:       config.ContextKey,
		SigningKey:       nil,
		SigningKeys:      nil,
		SigningMethod:    "",
		TokenLookup:      config.TokenLookup,
		TokenLookupFuncs: nil,
		ParseTokenFunc:   nil,
		KeyFunc: func(token *jwt.Token) (any, error) {
			return config.Keyfunc.Keyfunc(token)
		},
		NewClaimsFunc:          config.NewClaimsFunc,
		SuccessHandler:         createSuccessHandler(config.Logger),
		ErrorHandler:           createErrorHandler(config.Logger),
		ContinueOnIgnoredError: false,
	}
}

func createSuccessHandler(logger *zerolog.Logger) func(*echo.Context) error {
	return func(echoCtx *echo.Context) error {
		token, ok := echoCtx.Get("user").(*jwt.Token)
		if !ok {
			jwtLogError(logger, "JWT token retrieval from context failed", nil)

			return nil
		}

		echoCtx.Set(ContextKeyToken, token.Raw)

		if claims, ok := token.Claims.(*ExtendedClaims); ok {
			echoCtx.Set(ContextKeyClaims, claims)
			jwtLogDebug(logger, "JWT token and claims set successfully")
		} else {
			jwtLogError(logger, "Token claims assertion as ExtendedClaims failed", nil)
		}

		jwtLogDebug(logger, "JWT token verified successfully")

		return nil
	}
}

func createErrorHandler(logger *zerolog.Logger) func(*echo.Context, error) error {
	return func(_ *echo.Context, err error) error {
		jwtLogError(logger, "JWT verification failed", err)

		if err.Error() == "missing value in request header" {
			return ErrTokenRequired
		}

		return ErrInvalidToken
	}
}

func jwtLogDebug(logger *zerolog.Logger, msg string) {
	if logger != nil {
		logger.Debug().Msg(msg)
	}
}

func jwtLogError(logger *zerolog.Logger, msg string, err error) {
	if logger != nil {
		event := logger.Error()
		if err != nil {
			event = event.Err(err)
		}

		event.Msg(msg)
	}
}
