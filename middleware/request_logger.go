package middleware

import (
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

//nolint:gochecknoglobals
var sensitiveQueryParams = []string{
	"token",
	"access_token",
	"id_token",
	"refresh_token",
	"code",
	"secret",
	"password",
	"api_key",
	"apikey",
	"key",
	"signature",
	"authorization",
}

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

			fields["source"] = "gframework"

			addExtraLogFields(fields, ctx, extraLogFieldExtractor)

			log.Info().Fields(fields).Msg("Request completed")

			return nil
		}
	}
}

func extractLogFields(ctx *echo.Context, start time.Time, res *echo.Response) map[string]any {
	req := ctx.Request()
	loggedURL := redactQueryParams(req.URL)

	return map[string]any{
		"remote_ip":   ctx.RealIP(),
		"latency":     time.Since(start).String(),
		"host":        req.Host,
		"request":     req.Method + " " + loggedURL,
		"request_uri": loggedURL,
		"status":      res.Status,
		"size":        res.Size,
		"user_agent":  req.UserAgent(),
	}
}

func redactQueryParams(requestURL *url.URL) string {
	if requestURL.RawQuery == "" {
		return requestURL.String()
	}

	query := requestURL.Query()
	redacted := false

	for name := range query {
		if slicesContainsFold(sensitiveQueryParams, name) {
			query.Set(name, "REDACTED")

			redacted = true
		}
	}

	if !redacted {
		return requestURL.String()
	}

	clone := *requestURL
	clone.RawQuery = query.Encode()

	return clone.String()
}

func slicesContainsFold(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if strings.EqualFold(candidate, needle) {
			return true
		}
	}

	return false
}

func addExtraLogFields(fields map[string]any, ctx *echo.Context, extractors []LogFieldExtractor) {
	for _, extractor := range extractors {
		maps.Copy(fields, extractor(ctx))
	}
}
