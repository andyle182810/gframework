package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type LogFieldExtractor func(echo.Context) map[string]any

func RequestLogger(log zerolog.Logger, extraLogFieldExtractor ...LogFieldExtractor) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ectx echo.Context) error {
			start := time.Now()

			err := next(ectx)
			if err != nil {
				ectx.Error(err)
			}

			fields := extractLogFields(ectx, start)

			// Add request ID if available
			if id, ok := ectx.Get(ContextKeyRequestID).(string); ok && id != "" {
				fields["request_id"] = id
			}

			// Extract extra log fields
			addExtraLogFields(fields, ectx, extraLogFieldExtractor)

			logRequest(log, fields, err, ectx.Response().Status)

			return nil
		}
	}
}

// extractLogFields extracts basic request/response log fields.
func extractLogFields(ectx echo.Context, start time.Time) map[string]any {
	req := ectx.Request()
	res := ectx.Response()

	return map[string]any{
		"remote_ip":   ectx.RealIP(),
		"latency":     time.Since(start).String(),
		"host":        req.Host,
		"request":     req.Method + " " + req.URL.String(),
		"request_uri": req.RequestURI,
		"status":      res.Status,
		"size":        res.Size,
		"user_agent":  req.UserAgent(),
	}
}

// addExtraLogFields adds additional fields from LogFieldExtractors.
func addExtraLogFields(fields map[string]interface{}, ectx echo.Context, extractors []LogFieldExtractor) {
	for _, extractor := range extractors {
		for k, v := range extractor(ectx) {
			fields[k] = v
		}
	}
}

// logRequest logs the request based on the status code.
func logRequest(log zerolog.Logger, fields map[string]interface{}, err error, status int) {
	logger := log.With().Fields(fields).Logger()
	if err != nil {
		logger = logger.With().Err(err).Logger()
	}

	switch {
	case status >= http.StatusInternalServerError:
		logger.Error().
			Msg("The request has resulted in a server error")
	case status >= http.StatusBadRequest:
		logger.Error().
			Msg("The request has resulted in a client error")
	case status >= http.StatusMultipleChoices:
		logger.Info().
			Msg("The request has resulted in a redirection")
	default:
		logger.Info().
			Msg("The request has completed successfully")
	}
}
