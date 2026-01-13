package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

var ErrGeneric = errors.New("generic error")

func TestErrorHandler_HTTPError(t *testing.T) {
	t.Parallel()

	// Set up test context
	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:      http.MethodPost,
		Path:        "/test",
		Body:        nil,
		Headers:     nil,
		QueryParams: nil,
		PathParams:  nil,
		ContentType: "",
	})

	// Mock next error handler
	var nextCalled bool

	next := func(_ error, _ echo.Context) {
		nextCalled = true
	}

	// Middleware with the mock next handler
	errorHandler := middleware.ErrorHandler(next)

	// Test case: Handle *echo.HTTPError
	httpErr := &echo.HTTPError{
		Code:     http.StatusBadRequest,
		Message:  "Bad Request",
		Internal: nil,
	}
	errorHandler(httpErr, ctx)

	// Expected JSON response
	expectedResponse := `{"message":"Bad Request"}` + "\n"

	// Verify response
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, expectedResponse, rec.Body.String()) // Compare JSON
	require.False(t, nextCalled)                           // Next handler should not be called for HTTPError
}

func TestErrorHandler_GenericError(t *testing.T) {
	t.Parallel()

	// Set up test context
	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:      http.MethodPost,
		Path:        "/test",
		Body:        nil,
		Headers:     nil,
		QueryParams: nil,
		PathParams:  nil,
		ContentType: "",
	})

	// Track if next handler was called
	var nextCalled bool

	next := func(_ error, _ echo.Context) {
		nextCalled = true
	}

	// Middleware with the next handler
	errorHandler := middleware.ErrorHandler(next)

	// Test case: Handle a generic error (not *echo.HTTPError)
	errorHandler(ErrGeneric, ctx)

	// Assertions
	require.True(t, nextCalled) // Ensure next handler was called
}

// BenchmarkErrorHandler_HTTPError benchmarks handling of *echo.HTTPError.
func BenchmarkErrorHandler_HTTPError(b *testing.B) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rec)

	errorHandler := middleware.ErrorHandler(nil)

	httpErr := &echo.HTTPError{
		Code:     400,
		Message:  "Bad Request",
		Internal: nil,
	}

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		rec.Body.Reset() // Reset response recorder for each iteration
		errorHandler(httpErr, ctx)
	}
}

// BenchmarkErrorHandler_GenericError benchmarks handling of generic errors.
func BenchmarkErrorHandler_GenericError(b *testing.B) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rec)

	next := func(_ error, _ echo.Context) {}
	errorHandler := middleware.ErrorHandler(next)

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		errorHandler(ErrGeneric, ctx)
	}
}
