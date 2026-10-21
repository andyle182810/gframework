package middleware_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/i18n"
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var errDatabaseConnectionFailed = errors.New("database connection failed")

func echoTestOptions(method string) *testutil.Options {
	return &testutil.Options{
		Method:        method,
		Path:          testPath,
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: false,
	}
}

func errorHandlerConfig(
	logger *zerolog.Logger,
	logErrors bool,
	includeInternalErrors bool,
	customResponse func(*echo.Context, error, int) map[string]any,
) *middleware.ErrorHandlerConfig {
	return &middleware.ErrorHandlerConfig{
		Logger:                logger,
		LogErrors:             logErrors,
		IncludeInternalErrors: includeInternalErrors,
		CustomErrorResponse:   customResponse,
	}
}

const (
	testPath          = "/test"
	badRequestMessage = "Bad Request"
)

var ErrGeneric = errors.New("generic error")

func TestErrorHandler_HTTPError(t *testing.T) {
	t.Parallel()

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodPost,
		Path:          testPath,
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
		Message: badRequestMessage,
	}
	errorHandler(ctx, httpErr)

	expectedResponse := `{"code":"BAD_REQUEST","message":"Bad Request"}` + "\n"

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, expectedResponse, rec.Body.String()) // Compare JSON
	require.False(t, nextCalled)                           // Next handler
}

func TestErrorHandler_GenericError(t *testing.T) {
	t.Parallel()

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodPost,
		Path:          testPath,
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

func TestErrorHandler_MessageCodes(t *testing.T) {
	t.Parallel()

	const code = "PHONE_NUMBER_IN_USE"

	catalog := i18n.Catalog{
		code: {
			i18n.English:    "Phone number is already used",
			i18n.Vietnamese: "Số điện thoại đã được sử dụng",
		},
	}

	tests := []struct {
		name           string
		acceptLanguage string
		message        string
		want           string
	}{
		{
			name:           "code renders in the requested locale",
			acceptLanguage: "vi",
			message:        code,
			want:           `{"code":"PHONE_NUMBER_IN_USE","message":"Số điện thoại đã được sử dụng"}`,
		},
		{
			name:           "no header falls back to english",
			acceptLanguage: "",
			message:        code,
			want:           `{"code":"PHONE_NUMBER_IN_USE","message":"Phone number is already used"}`,
		},
		{
			name:           "unsupported locale falls back to english",
			acceptLanguage: "ja-JP",
			message:        code,
			want:           `{"code":"PHONE_NUMBER_IN_USE","message":"Phone number is already used"}`,
		},
		{
			// An unmigrated handler keeps its prose and still answers with a
			// code the client can branch on.
			name:           "prose keeps its text under the generic code",
			acceptLanguage: "vi",
			message:        "Employee not found",
			want:           `{"code":"CONFLICT","message":"Employee not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
				Method:  http.MethodPost,
				Path:    testPath,
				Headers: map[string]string{middleware.AcceptLanguageHeader: tt.acceptLanguage},
			})

			config := &middleware.ErrorHandlerConfig{Messages: catalog} //nolint:exhaustruct
			errorHandler := middleware.ErrorHandler(nil, config)

			errorHandler(ctx, &echo.HTTPError{Code: http.StatusConflict, Message: tt.message})

			require.Equal(t, http.StatusConflict, rec.Code)
			require.JSONEq(t, tt.want, rec.Body.String())
		})
	}
}

func BenchmarkErrorHandler_HTTPError(b *testing.B) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rec)

	errorHandler := middleware.ErrorHandler(nil)

	httpErr := &echo.HTTPError{
		Code:    400,
		Message: badRequestMessage,
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
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil)
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

	ctx, rec, _ := testutil.SetupEchoContext(t, echoTestOptions(http.MethodGet))

	config := errorHandlerConfig(&logger, true, false, nil)
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

	ctx, rec, _ := testutil.SetupEchoContext(t, echoTestOptions(http.MethodPost))

	config := errorHandlerConfig(nil, false, true, nil)
	errorHandler := middleware.ErrorHandler(nil, config)

	internalErr := errDatabaseConnectionFailed
	baseErr := echo.NewHTTPError(http.StatusServiceUnavailable, "Service Unavailable")

	wrappedErr := baseErr.Wrap(internalErr)

	var httpErr *echo.HTTPError

	ok := errors.As(wrappedErr, &httpErr)
	require.True(t, ok)

	errorHandler(ctx, httpErr)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	// The handler's own prose is withheld at 5xx; the caller gets the generic
	// text for the status, and the cause only because IncludeInternalErrors is on.
	require.Contains(t, rec.Body.String(), "The service is temporarily unavailable")
	require.NotContains(t, rec.Body.String(), "Service Unavailable")
	require.Contains(t, rec.Body.String(), "database connection failed")
}

func TestErrorHandler_CustomErrorResponse(t *testing.T) {
	t.Parallel()

	ctx, rec, _ := testutil.SetupEchoContext(t, echoTestOptions(http.MethodGet))

	config := errorHandlerConfig(
		nil,
		false,
		false,
		func(_ *echo.Context, err error, code int) map[string]any {
			return map[string]any{
				"error":  err.Error(),
				"status": code,
				"custom": "field",
			}
		},
	)
	errorHandler := middleware.ErrorHandler(nil, config)

	httpErr := &echo.HTTPError{
		Code:    http.StatusBadRequest,
		Message: badRequestMessage,
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

	ctx, _, _ := testutil.SetupEchoContext(t, echoTestOptions(http.MethodPost))

	var nextCalled bool

	next := func(_ *echo.Context, _ error) {
		nextCalled = true
	}

	config := errorHandlerConfig(&logger, true, false, nil)
	errorHandler := middleware.ErrorHandler(next, config)

	errorHandler(ctx, ErrGeneric)

	require.True(t, nextCalled)

	require.Contains(t, buf.String(), "Unhandled error")
	require.Contains(t, buf.String(), "generic error")
}
