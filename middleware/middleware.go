// Package middleware provides reusable Echo v5 middleware and context value accessors for common patterns.
//
// It includes JWT validation with JWKS support, request ID generation/propagation, request logging,
// error handling, and typed context accessor functions. Each middleware is optional and can be
// composed based on application requirements.
//
// Key components:
//
//   - JWT validation via middleware.JWT() with support for Keycloak-style extended claims
//   - Request ID tracking via middleware.RequestID()
//   - Request logging via middleware.RequestLogger()
//   - Centralized error handling via middleware.ErrorHandler()
//   - Context getter functions: GetRequestID(), GetToken(), GetExtendedClaimsFromContext(), etc.
//
// Example:
//
//	e := echo.New()
//	e.Use(middleware.RequestID())
//	e.Use(middleware.RequestLogger(logger))
//	e.Use(middleware.JWT(jwtConfig))
//	e.GET("/api/protected", protectedHandler)
//
// All context values use typed keys (ContextKeyTenantID, ContextKeyRequestID, etc.) to prevent collisions.
package middleware

import (
	"errors"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	ContextKeyTenantID      string = "tenantID"
	ContextKeyRateLimitReqs string = "rateLimitRequests"
	ContextKeyRateLimitWin  string = "rateLimitWindow"
	ContextKeyAPIKey        string = "apiKey"
	ContextKeyRequestID     string = "requestID"
	ContextKeyBody          string = "body"
	ContextKeyToken         string = "token"
	ContextKeyClaims        string = "claims"
	ContextKeyHandler       string = "handler"
)

const (
	HeaderXAPIKey    = "X-Api-Key" //nolint:gosec
	HeaderXRequestID = "X-Request-ID"
	HeaderXSignature = "X-Signature"
	HeaderXTimestamp = "X-Timestamp"
)

const (
	HeaderRateLimitLimit     = "X-Ratelimit-Limit"
	HeaderRateLimitRemaining = "X-Ratelimit-Remaining"
	HeaderRateLimitReset     = "X-Ratelimit-Reset"
)

var (
	ErrClaimsNotFound            = errors.New("jwt claims: not found in context")
	ErrClaimsTypeAssertionFailed = errors.New("jwt claims: type assertion failed")
	ErrMissingJWTClaims          = errors.New("jwt claims: missing")
	ErrJWTSubjectEmpty           = errors.New("jwt claims: subject is empty")
)

func GetTenantID(c *echo.Context) string {
	if tenantID, ok := c.Get(ContextKeyTenantID).(string); ok {
		return tenantID
	}

	return ""
}

func GetRateLimitRequests(c *echo.Context) int {
	if requests, ok := c.Get(ContextKeyRateLimitReqs).(int); ok {
		return requests
	}

	return 0
}

func GetRateLimitWindow(c *echo.Context) int {
	if window, ok := c.Get(ContextKeyRateLimitWin).(int); ok {
		return window
	}

	return 0
}

func GetAPIKey(c *echo.Context) string {
	if apiKey, ok := c.Get(ContextKeyAPIKey).(string); ok {
		return apiKey
	}

	return ""
}

func GetRequestID(c *echo.Context) string {
	if requestID, ok := c.Get(ContextKeyRequestID).(string); ok {
		return requestID
	}

	return uuid.NewString()
}

func GetToken(c *echo.Context) string {
	if token, ok := c.Get(ContextKeyToken).(string); ok {
		return token
	}

	return ""
}

func GetExtendedClaimsFromContext(c *echo.Context) (*ExtendedClaims, error) {
	claimsValue := c.Get(ContextKeyClaims)

	if claimsValue == nil {
		return nil, ErrClaimsNotFound
	}

	claims, ok := claimsValue.(*ExtendedClaims)
	if !ok {
		return nil, ErrClaimsTypeAssertionFailed
	}

	return claims, nil
}

func CurrentUserID(c *echo.Context) (string, error) {
	claims, err := GetExtendedClaimsFromContext(c)
	if err != nil {
		return "", ErrMissingJWTClaims
	}

	if claims.Subject == "" {
		return "", ErrJWTSubjectEmpty
	}

	return claims.Subject, nil
}

func GetHandler(c *echo.Context) string {
	if handler, ok := c.Get(ContextKeyHandler).(string); ok {
		return handler
	}

	return ""
}
