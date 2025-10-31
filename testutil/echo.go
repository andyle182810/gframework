package testutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/validator"
	"github.com/labstack/echo/v4"
)

type Options struct {
	Method string
	Path   string
	Body   []byte
}

//nolint:ireturn
func SetupEchoContext(
	t *testing.T,
	opts *Options,
) (echo.Context, *httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	// Initialize Echo with a default validator
	iecho := echo.New()
	iecho.Validator = validator.DefaultRestValidator()

	// Create an HTTP request
	req := httptest.NewRequest(opts.Method, opts.Path, bytes.NewBuffer(opts.Body))
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	rec := httptest.NewRecorder()
	ctx := iecho.NewContext(req, rec)

	return ctx, rec, req
}
