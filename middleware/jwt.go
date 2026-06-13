package middleware

import (
	"net/http"
	"slices"
	"time"

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

// DefaultJWTLeeway is the clock-skew tolerance applied to time-based claims (exp, nbf, iat).
const DefaultJWTLeeway = 30 * time.Second

// DefaultValidMethods returns the JWT signing algorithms accepted by default.
// Only asymmetric algorithms are included: accepting HS* alongside a JWKS keyfunc
// would open the door to algorithm-confusion attacks where a public key is used
// as an HMAC secret.
func DefaultValidMethods() []string {
	return []string{
		"RS256", "RS384", "RS512",
		"ES256", "ES384", "ES512",
		"PS256", "PS384", "PS512",
		"EdDSA",
	}
}

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

	// ValidMethods lists the accepted JWT signing algorithms. Empty means
	// DefaultValidMethods(). Never include HS* methods when keys come from a JWKS.
	ValidMethods []string

	// Issuer, when non-empty, requires the token "iss" claim to match exactly.
	// Set this to the realm issuer URL (e.g. https://auth.example.com/realms/my-realm)
	// so tokens minted by other issuers are rejected.
	Issuer string

	// Audiences, when non-empty, requires the token "aud" claim to contain at
	// least one of the listed values. Set this to reject tokens issued for
	// other services in the same realm.
	Audiences []string

	// Leeway is the clock-skew tolerance for time-based claims.
	// Zero means DefaultJWTLeeway; set a negative-free small value (e.g. time.Nanosecond)
	// to effectively disable leeway.
	Leeway time.Duration
}

func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Skipper:       middleware.DefaultSkipper,
		Logger:        &log.Logger,
		Keyfunc:       nil,
		NewClaimsFunc: defaultNewClaimsFunc,
		ContextKey:    "user",
		TokenLookup:   "",
		ValidMethods:  DefaultValidMethods(),
		Issuer:        "",
		Audiences:     nil,
		Leeway:        DefaultJWTLeeway,
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

	if len(config.ValidMethods) == 0 {
		config.ValidMethods = DefaultValidMethods()
	}

	if config.Leeway == 0 {
		config.Leeway = DefaultJWTLeeway
	}

	jwtMiddleware := echojwt.WithConfig(buildJWTConfig(config))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		handler := jwtMiddleware(next)

		return func(ctx *echo.Context) error {
			if config.Skipper(ctx) {
				return next(ctx)
			}

			return handler(ctx)
		}
	}
}

func buildJWTConfig(config JWTConfig) echojwt.Config {
	return echojwt.Config{
		Skipper:                nil,
		BeforeFunc:             nil,
		ContextKey:             config.ContextKey,
		SigningKey:             nil,
		SigningKeys:            nil,
		SigningMethod:          "",
		TokenLookup:            config.TokenLookup,
		TokenLookupFuncs:       nil,
		ParseTokenFunc:         createParseTokenFunc(config),
		KeyFunc:                nil,
		NewClaimsFunc:          config.NewClaimsFunc,
		SuccessHandler:         createSuccessHandler(config.Logger, config.ContextKey),
		ErrorHandler:           createErrorHandler(config.Logger),
		ContinueOnIgnoredError: false,
	}
}

// createParseTokenFunc builds the token parser used in place of echo-jwt's default.
// echo-jwt skips its own algorithm check when a custom key function is supplied,
// so algorithm pinning, issuer, audience, and leeway validation happen here.
func createParseTokenFunc(config JWTConfig) func(*echo.Context, string) (any, error) {
	parserOpts := buildParserOptions(config.ValidMethods, config.Issuer, config.Audiences, config.Leeway)

	return func(ctx *echo.Context, auth string) (any, error) {
		token, err := jwt.ParseWithClaims(auth, config.NewClaimsFunc(ctx), config.Keyfunc.Keyfunc, parserOpts...)
		if err != nil {
			return nil, err
		}

		if !token.Valid {
			return nil, jwt.ErrTokenUnverifiable
		}

		return token, nil
	}
}

func buildParserOptions(validMethods []string, issuer string, audiences []string, leeway time.Duration) []jwt.ParserOption {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods(validMethods),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(leeway),
	}

	if issuer != "" {
		opts = append(opts, jwt.WithIssuer(issuer))
	}

	if len(audiences) > 0 {
		opts = append(opts, jwt.WithAudience(audiences...))
	}

	return opts
}

func createSuccessHandler(logger *zerolog.Logger, contextKey string) func(*echo.Context) error {
	return func(echoCtx *echo.Context) error {
		token, ok := echoCtx.Get(contextKey).(*jwt.Token)
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
