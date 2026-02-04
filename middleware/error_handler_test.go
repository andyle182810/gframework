package middleware_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var ErrGeneric = errors.New("generic error")

func TestErrorHandler_HTTPError(t *testing.T) {
	t.Parallel()

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodPost,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: false,
	})

	var nextCalled bool

	next := func(_ *echo.Context, _ error) {
		nextCalled = true
	}

	errorHandler := middleware.ErrorHandler(next)

	httpErr := &echo.HTTPError{
		Code:    http.StatusBadRequest,
		Message: "Bad Request",
	}
	errorHandler(ctx, httpErr)

	expectedResponse := `{"message":"Bad Request"}` + "\n"

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, expectedResponse, rec.Body.String()) // Compare JSON
	require.False(t, nextCalled)                           // Next handler
}

func TestErrorHandler_GenericError(t *testing.T) {
	t.Parallel()

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodPost,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: false,
	})

	var nextCalled bool

	next := func(_ *echo.Context, _ error) {
		nextCalled = true
	}

	errorHandler := middleware.ErrorHandler(next)

	errorHandler(ctx, ErrGeneric)

	require.True(t, nextCalled)
}

func BenchmarkErrorHandler_HTTPError(b *testing.B) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rec)

	errorHandler := middleware.ErrorHandler(nil)

	httpErr := &echo.HTTPError{
		Code:    400,
		Message: "Bad Request",
	}

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		rec.Body.Reset()
		errorHandler(ctx, httpErr)
	}
}

func BenchmarkErrorHandler_GenericError(b *testing.B) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rec)

	next := func(_ *echo.Context, _ error) {}
	errorHandler := middleware.ErrorHandler(next)

	b.ResetTimer()

	for range make([]struct{}, b.N) {
		errorHandler(ctx, ErrGeneric)
	}
}

func TestErrorHandler_WithLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodGet,
		Path:   "/test",
	})

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		Logger:    &logger,
		LogErrors: true,
	}
	errorHandler := middleware.ErrorHandler(nil, config)

	httpErr := &echo.HTTPError{
		Code:    http.StatusInternalServerError,
		Message: "Internal Server Error",
	}
	errorHandler(ctx, httpErr)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	require.Contains(t, buf.String(), "Request failed with server error")
	require.Contains(t, buf.String(), "Internal Server Error")
}

func TestErrorHandler_WithWrappedError(t *testing.T) {
	t.Parallel()

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodPost,
		Path:   "/test",
	})

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		IncludeInternalErrors: true,
	}
	errorHandler := middleware.ErrorHandler(nil, config)

	internalErr := errors.New("database connection failed") //nolint:err113
	baseErr := echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable")

	wrappedErr := baseErr.Wrap(internalErr)

	var httpErr *echo.HTTPError

	ok := errors.As(wrappedErr, &httpErr)
	require.True(t, ok)

	errorHandler(ctx, httpErr)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "Service Unavailable")
	require.Contains(t, rec.Body.String(), "database connection failed")
}

func TestErrorHandler_CustomErrorResponse(t *testing.T) {
	t.Parallel()

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodGet,
		Path:   "/test",
	})

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		CustomErrorResponse: func(_ *echo.Context, err error, code int) map[string]any {
			return map[string]any{
				"error":  err.Error(),
				"status": code,
				"custom": "field",
			}
		},
	}
	errorHandler := middleware.ErrorHandler(nil, config)

	httpErr := &echo.HTTPError{
		Code:    http.StatusBadRequest,
		Message: "Bad Request",
	}
	errorHandler(ctx, httpErr)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "\"custom\":\"field\"")
	require.Contains(t, rec.Body.String(), "\"status\":400")
}

func TestErrorHandler_NonHTTPError_WithLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodPost,
		Path:   "/test",
	})

	var nextCalled bool

	next := func(_ *echo.Context, _ error) {
		nextCalled = true
	}

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		Logger:    &logger,
		LogErrors: true,
	}
	errorHandler := middleware.ErrorHandler(next, config)

	errorHandler(ctx, ErrGeneric)

	require.True(t, nextCalled)

	require.Contains(t, buf.String(), "Unhandled error")
	require.Contains(t, buf.String(), "generic error")
}
