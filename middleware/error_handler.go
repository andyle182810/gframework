package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

// ErrorHandlerConfig holds configuration for the error handler middleware.
type ErrorHandlerConfig struct {
	// Logger for logging errors (optional)
	Logger *zerolog.Logger
	// LogErrors enables automatic error logging
	LogErrors bool
	// IncludeInternalErrors includes internal error details in response (useful for dev, dangerous for prod)
	IncludeInternalErrors bool
	// CustomErrorResponse allows customizing the error response structure
	CustomErrorResponse func(*echo.Context, error, int) map[string]any
}

// ErrorHandler creates an error handler middleware with optional configuration.
func ErrorHandler(next echo.HTTPErrorHandler, config ...*ErrorHandlerConfig) echo.HTTPErrorHandler {
	cfg := getErrorHandlerConfig(config)

	return func(ectx *echo.Context, err error) {
		// Skip if response is already committed
		res, unwrapErr := echo.UnwrapResponse(ectx.Response())
		if unwrapErr == nil && res.Committed {
			return
		}

		// Handle echo.HTTPError
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			handleHTTPError(ectx, httpErr, cfg)

			return
		}

		// Log non-HTTP errors if configured
		if cfg.LogErrors && cfg.Logger != nil {
			logError(ectx, err, cfg.Logger)
		}

		// Delegate to next handler if available
		if next != nil {
			next(ectx, err)
		}
	}
}

func getErrorHandlerConfig(config []*ErrorHandlerConfig) *ErrorHandlerConfig {
	if len(config) > 0 && config[0] != nil {
		return config[0]
	}

	return &ErrorHandlerConfig{} //nolint:exhaustruct
}

func handleHTTPError(ectx *echo.Context, httpErr *echo.HTTPError, cfg *ErrorHandlerConfig) {
	// Log HTTP errors if configured
	if cfg.LogErrors && cfg.Logger != nil {
		logHTTPError(ectx, httpErr, cfg.Logger)
	}

	// Use custom response if configured
	if cfg.CustomErrorResponse != nil {
		response := cfg.CustomErrorResponse(ectx, httpErr, httpErr.Code)
		_ = ectx.JSON(httpErr.Code, response)

		return
	}

	// Build standard error response
	response := buildErrorResponse(httpErr, cfg)
	_ = ectx.JSON(httpErr.Code, response)
}

func buildErrorResponse(httpErr *echo.HTTPError, cfg *ErrorHandlerConfig) map[string]any {
	response := map[string]any{
		"message": httpErr.Message,
	}

	// Include internal error details if configured (dev mode)
	if cfg.IncludeInternalErrors {
		if internal := httpErr.Unwrap(); internal != nil {
			response["internal"] = internal.Error()
		}
	}

	return response
}

func logHTTPError(ectx *echo.Context, httpErr *echo.HTTPError, logger *zerolog.Logger) {
	logFields := map[string]any{
		"status_code": httpErr.Code,
		"message":     httpErr.Message,
		"path":        ectx.Request().URL.Path,
		"method":      ectx.Request().Method,
	}

	if id, ok := ectx.Get(ContextKeyRequestID).(string); ok && id != "" {
		logFields["request_id"] = id
	}

	loggerWithFields := logger.With().Fields(logFields).Logger()

	if internal := httpErr.Unwrap(); internal != nil {
		loggerWithFields = loggerWithFields.With().Err(internal).Logger()
	}

	// Log based on status code
	switch {
	case httpErr.Code >= http.StatusInternalServerError:
		loggerWithFields.Error().Msg("HTTP error: server error")
	case httpErr.Code >= http.StatusBadRequest:
		loggerWithFields.Warn().Msg("HTTP error: client error")
	default:
		loggerWithFields.Info().Msg("HTTP error")
	}
}

func logError(ectx *echo.Context, err error, logger *zerolog.Logger) {
	logFields := map[string]any{
		"path":   ectx.Request().URL.Path,
		"method": ectx.Request().Method,
	}

	if id, ok := ectx.Get(ContextKeyRequestID).(string); ok && id != "" {
		logFields["request_id"] = id
	}

	logger.Error().
		Err(err).
		Fields(logFields).
		Msg("Unhandled error")
}
