package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/transformer"
	"github.com/andyle182810/gframework/validator"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type scrubbableBody struct {
	Email string `json:"email" scrub:"emails"`
}

func TestScrubbedBodyLogFieldExtractor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf)
	tfm := transformer.DefaultRestTransformer()

	e := echo.New()
	e.Validator = validator.DefaultRestValidator()
	e.Pre(middleware.RequestLogger(logger, safeLogFieldsExtractor, scrubbedBodyLogFieldExtractor(tfm)))
	e.Use(injectTransformer(tfm))

	e.Group("").POST("/users", Wrapper(func(_ *echo.Context, _ *scrubbableBody) (any, *echo.HTTPError) {
		return map[string]string{"status": "ok"}, nil
	}))

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/users", strings.NewReader(`{"email":"joe@example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	logged := buf.String()

	require.Contains(t, logged, "Request completed", "the single request log line must be emitted")
	require.Contains(t, logged, `"body":`, "scrubbed body must be folded into the request log line")
	require.Contains(t, logged, "<<scrubbed::email::sha1::")
	require.NotContains(t, logged, "joe@example.com", "raw PII must never reach the logs")
}
