package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/andyle182810/gframework/i18n"
	"github.com/andyle182810/gframework/validator"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

// AcceptLanguageHeader names the locale the caller wants error messages in.
const AcceptLanguageHeader = "Accept-Language"

type ErrorHandlerConfig struct {
	Logger    *zerolog.Logger
	LogErrors bool
	// IncludeInternalErrors adds the wrapped internal error string to HTTP
	// responses under the "internal" key. It exposes implementation details
	// (driver errors, hostnames, queries) to clients — only enable it in
	// development environments, never in production.
	IncludeInternalErrors bool
	CustomErrorResponse   func(*echo.Context, error, int) map[string]any
	// Messages translates message codes raised by handlers. Errors carrying
	// anything else are returned with their message untouched.
	Messages i18n.Catalog
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

	response := buildErrorResponse(ectx, httpErr, cfg)
	_ = ectx.JSON(httpErr.Code, response)
}

func buildErrorResponse(ectx *echo.Context, httpErr *echo.HTTPError, cfg *ErrorHandlerConfig) map[string]any {
	code, message := resolveMessage(ectx, httpErr, cfg.Messages)

	response := map[string]any{
		"code":    code,
		"message": message,
	}

	if cfg.IncludeInternalErrors {
		if internal := httpErr.Unwrap(); internal != nil {
			response["internal"] = internal.Error()
		}
	}

	return response
}

// resolveMessage splits an error message into the code a client branches on and
// the text a person reads. A message registered in the catalog is a code: it is
// reported as-is and the text comes from the catalog in the caller's locale.
// Anything else is literal prose, kept verbatim under the generic code for the
// status, so an unmigrated handler still answers with a usable code.
//
// Request validation is the exception: it fails before any handler runs, so its
// message is rebuilt here from the field failures the validator recorded rather
// than read off the error.
func resolveMessage(ectx *echo.Context, httpErr *echo.HTTPError, catalog i18n.Catalog) (string, string) {
	locale := i18n.Negotiate(ectx.Request().Header.Get(AcceptLanguageHeader))

	var validationErrs validator.ValidationErrors
	if errors.As(httpErr, &validationErrs) && len(validationErrs) > 0 {
		return i18n.CodeValidation, localizeValidation(validationErrs, catalog, locale)
	}

	text, registered := catalog.Lookup(httpErr.Message, locale)
	if !registered {
		return i18n.CodeForStatus(httpErr.Code), httpErr.Message
	}

	return httpErr.Message, text
}

// localizeValidation renders every field failure and joins them the way
// validator.ValidationErrors does, so the message reads the same in either
// language and the English original still reaches the log untouched.
func localizeValidation(
	errs validator.ValidationErrors, catalog i18n.Catalog, locale i18n.Locale,
) string {
	messages := make([]string, 0, len(errs))

	for _, err := range errs {
		messages = append(messages, i18n.ValidationMessage(catalog, locale, err.Field, err.Tag, err.Param))
	}

	return strings.Join(messages, "; ")
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
