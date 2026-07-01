package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

type conformReq struct {
	Name  string `json:"name"  mod:"trim"       validate:"required"`
	Email string `json:"email" mod:"trim,lcase" validate:"required,email"`
}

const paddedBody = `{"name":"  Joe  ","email":"  Joe@Example.COM  "}`

func newServer(t *testing.T, cfg *httpserver.Config) *httpserver.Server {
	t.Helper()

	cfg.Host = "127.0.0.1"
	cfg.Port = 0

	return httpserver.New(cfg)
}

func TestWrapper_ConformsBeforeValidation(t *testing.T) {
	t.Parallel()

	srv := newServer(t, &httpserver.Config{}) //nolint:exhaustruct

	var captured conformReq

	srv.Root.POST("/users", httpserver.Wrapper(func(_ *echo.Context, req *conformReq) (any, *echo.HTTPError) {
		captured = *req

		return map[string]string{"status": "ok"}, nil
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users", strings.NewReader(paddedBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.Echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, "Joe", captured.Name)
	require.Equal(t, "joe@example.com", captured.Email)
}

func TestWrapper_TransformDisabled_FailsOnDirtyInput(t *testing.T) {
	t.Parallel()

	srv := newServer(t, &httpserver.Config{DisableTransform: true}) //nolint:exhaustruct

	srv.Root.POST("/users", httpserver.Wrapper(func(_ *echo.Context, req *conformReq) (any, *echo.HTTPError) {
		return req, nil
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users", strings.NewReader(paddedBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.Echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
