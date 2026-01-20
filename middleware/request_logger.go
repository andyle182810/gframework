package middleware

import (
	"maps"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type LogFieldExtractor func(*echo.Context) map[string]any

func RequestLogger(log zerolog.Logger, extraLogFieldExtractor ...LogFieldExtractor) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			start := time.Now()

			err := next(ctx)
			if err != nil {
				return err
			}

			res, extractErr := echo.UnwrapResponse(ctx.Response())
			if extractErr != nil {
				return extractErr
			}

			fields := extractLogFields(ctx, start, res)

			if id, ok := ctx.Get(ContextKeyRequestID).(string); ok && id != "" {
				fields["request_id"] = id
			}

			addExtraLogFields(fields, ctx, extraLogFieldExtractor)

			status := 0
			if res != nil {
				status = res.Status
			}

			logRequest(log, fields, err, status)

			return nil
		}
	}
}

func extractLogFields(ctx *echo.Context, start time.Time, res *echo.Response) map[string]any {
	req := ctx.Request()

	return map[string]any{
		"remote_ip":   ctx.RealIP(),
		"latency":     time.Since(start).String(),
		"host":        req.Host,
		"request":     req.Method + " " + req.URL.String(),
		"request_uri": req.RequestURI,
		"status":      res.Status,
		"size":        res.Size,
		"user_agent":  req.UserAgent(),
	}
}

func addExtraLogFields(fields map[string]any, ctx *echo.Context, extractors []LogFieldExtractor) {
	for _, extractor := range extractors {
		maps.Copy(fields, extractor(ctx))
	}
}

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
