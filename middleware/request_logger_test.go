package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func BenchmarkRequestLogger(b *testing.B) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	middleware := middleware.RequestLogger(log.Logger)

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		// Simulate the next handler
		handler := middleware(func(c echo.Context) error {
			time.Sleep(5 * time.Millisecond)

			return c.String(http.StatusOK, "OK")
		})

		// Run the handler
		if err := handler(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
