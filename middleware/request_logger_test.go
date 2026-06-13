package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func BenchmarkRequestLogger(b *testing.B) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	middleware := middleware.RequestLogger(log.Logger)

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		handler := middleware(func(c *echo.Context) error {
			time.Sleep(5 * time.Millisecond)

			return c.String(http.StatusOK, "OK")
		})

		if err := handler(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRequestLogger_RedactsSensitiveQueryParams(t *testing.T) {
	t.Parallel()

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/callback?token=super-secret&page=1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	var buf bytes.Buffer

	logger := zerolog.New(&buf)

	handler := middleware.RequestLogger(logger)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	require.NoError(t, handler(ctx))

	logged := buf.String()

	require.NotContains(t, logged, "super-secret", "sensitive query values must not reach logs")
	require.Contains(t, logged, "token=REDACTED")
	require.Contains(t, logged, "page=1")
}
