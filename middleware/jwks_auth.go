package middleware

import (
	"net/http"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/andyle182810/gframework/notifylog"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

var (
	ErrTokenRequired   = echo.NewHTTPError(http.StatusUnauthorized, "Authorization header is required")
	ErrJWKSFetchFailed = echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch JWKS")
	ErrInvalidToken    = echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
)

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
	log     notifylog.NotifyLog
	keyfunc keyfunc.Keyfunc
}

func NewJwksAuth(
	log notifylog.NotifyLog,
	keyfunc keyfunc.Keyfunc,
) *JwksAuth {
	return &JwksAuth{
		log:     log,
		keyfunc: keyfunc,
	}
}

func (j *JwksAuth) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			requestID := ctx.Request().Header.Get(HeaderXRequestID)

			config := echojwt.Config{
				KeyFunc: func(t *jwt.Token) (any, error) {
					return j.keyfunc.Keyfunc(t)
				},
				NewClaimsFunc: func(c echo.Context) jwt.Claims {
					return &ExtendedClaims{}
				},
				SuccessHandler: func(c echo.Context) {
					token := c.Get("user").(*jwt.Token)

					c.Set(ContextKeyToken, token.Raw)

					if claims, ok := token.Claims.(*ExtendedClaims); ok {
						c.Set(ContextKeyClaims, claims)

						j.log.Debug().
							Str("request_id", requestID).
							Msg("JWT token and claims set successfully")
					} else {
						j.log.Error().
							Str("request_id", requestID).
							Msg("Failed to assert token claims as ExtendedClaims")
					}

					j.log.Debug().
						Str("request_id", requestID).
						Msg("JWT token verified successfully")
				},
				ErrorHandler: func(c echo.Context, err error) error {
					j.log.Error().
						Err(err).
						Str("request_id", requestID).
						Msg("JWT verification failed")

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
