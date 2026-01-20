package middleware

import (
	"net/http"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

var (
	ErrTokenRequired   = echo.NewHTTPError(http.StatusUnauthorized, "Authorization header is required")
	ErrJWKSFetchFailed = echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch JWKS")
	ErrInvalidToken    = echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
)

//nolint:tagliatelle
type ExtendedClaims struct {
	TenantID   string `json:"tenant_id"`
	CustomerID string `json:"customer_id"`
	jwt.RegisteredClaims
}

func (c *ExtendedClaims) GetTenantID() string {
	return c.TenantID
}

func (c *ExtendedClaims) GetCustomerID() string {
	return c.CustomerID
}

type JwksAuth struct {
	keyfunc keyfunc.Keyfunc
}

func NewJwksAuth(
	keyfunc keyfunc.Keyfunc,
) *JwksAuth {
	return &JwksAuth{
		keyfunc: keyfunc,
	}
}

func (j *JwksAuth) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			requestID := ctx.Request().Header.Get(HeaderXRequestID)
			log := zerolog.Ctx(ctx.Request().Context()).With().
				Str("middleware", "jwks_auth").
				Str("request_id", requestID).
				Logger()

			config := echojwt.Config{
				Skipper:          nil,
				BeforeFunc:       nil,
				ContextKey:       "user",
				SigningKey:       nil,
				SigningKeys:      nil,
				SigningMethod:    "",
				TokenLookup:      "",
				TokenLookupFuncs: nil,
				ParseTokenFunc:   nil,
				KeyFunc: func(token *jwt.Token) (any, error) {
					return j.keyfunc.Keyfunc(token)
				},
				NewClaimsFunc: func(_ *echo.Context) jwt.Claims {
					//nolint:exhaustruct // Claims fields are populated by JWT parser
					return &ExtendedClaims{}
				},
				SuccessHandler: func(echoCtx *echo.Context) error {
					token, ok := echoCtx.Get("user").(*jwt.Token)
					if !ok {
						log.Error().
							Msg("The JWT token failed to be retrieved from the context")

						return nil
					}

					echoCtx.Set(ContextKeyToken, token.Raw)

					if claims, ok := token.Claims.(*ExtendedClaims); ok {
						echoCtx.Set(ContextKeyClaims, claims)
						log.Debug().
							Msg("The JWT token and claims have been set successfully")
					} else {
						log.Error().
							Msg("The token claims failed to be asserted as ExtendedClaims")
					}

					log.Debug().
						Msg("The JWT token has been verified successfully")

					return nil
				},
				ErrorHandler: func(_ *echo.Context, err error) error {
					log.Error().
						Err(err).
						Msg("The JWT verification has failed")

					if err.Error() == "missing value in request header" {
						return ErrTokenRequired
					}

					return ErrInvalidToken
				},
				ContinueOnIgnoredError: false,
			}

			jwtMiddleware := echojwt.WithConfig(config)

			handler := jwtMiddleware(next)

			return handler(ctx)
		}
	}
}
