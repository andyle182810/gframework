package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type ErrorHandlerConfig struct {
	Logger                *zerolog.Logger
	LogErrors             bool
	IncludeInternalErrors bool
	CustomErrorResponse   func(*echo.Context, error, int) map[string]any
}

func ErrorHandler(next echo.HTTPErrorHandler, config ...*ErrorHandlerConfig) echo.HTTPErrorHandler {
	cfg := getErrorHandlerConfig(config)

	return func(ectx *echo.Context, err error) {
		res, unwrapErr := echo.UnwrapResponse(ectx.Response())
		if unwrapErr == nil && res.Committed {
			return
		}

		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			handleHTTPError(ectx, httpErr, cfg)

			return
		}

		if cfg.LogErrors && cfg.Logger != nil {
			logError(ectx, err, cfg.Logger)
		}

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
	if cfg.LogErrors && cfg.Logger != nil {
		logHTTPError(ectx, httpErr, cfg.Logger)
	}

	if cfg.CustomErrorResponse != nil {
		response := cfg.CustomErrorResponse(ectx, httpErr, httpErr.Code)
		_ = ectx.JSON(httpErr.Code, response)

		return
	}

	response := buildErrorResponse(httpErr, cfg)
	_ = ectx.JSON(httpErr.Code, response)
}

func buildErrorResponse(httpErr *echo.HTTPError, cfg *ErrorHandlerConfig) map[string]any {
	response := map[string]any{
		"message": httpErr.Message,
	}

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

	if handler, ok := ectx.Get(ContextKeyHandler).(string); ok && handler != "" {
		logFields["handler"] = handler
	}

	loggerWithFields := logger.With().Fields(logFields).Logger()

	if internal := httpErr.Unwrap(); internal != nil {
		loggerWithFields = loggerWithFields.With().Err(internal).Logger()
	}

	switch {
	case httpErr.Code >= http.StatusInternalServerError:
		loggerWithFields.Error().Msg("Request failed with server error")
	case httpErr.Code >= http.StatusBadRequest:
		loggerWithFields.Warn().Msg("Request failed with client error")
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

	if handler, ok := ectx.Get(ContextKeyHandler).(string); ok && handler != "" {
		logFields["handler"] = handler
	}

	logger.Error().
		Err(err).
		Fields(logFields).
		Msg("Unhandled error")
}
