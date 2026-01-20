package middleware_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/require"
)

func echoSuccessHandler(ctx *echo.Context) error {
	return ctx.String(http.StatusOK, "success")
}

type RequestIDTestCase struct {
	name           string
	requestID      string
	expectedStatus int
	skipper        func(*echo.Context) bool
}

func generateRequestIDTestCases() []RequestIDTestCase {
	return []RequestIDTestCase{
		{
			name:           "Valid Request ID",
			requestID:      uuid.New().String(),
			expectedStatus: http.StatusOK,
			skipper:        echomiddleware.DefaultSkipper,
		},
		{
			name:           "Missing Request ID",
			requestID:      "",
			expectedStatus: http.StatusBadRequest,
			skipper:        echomiddleware.DefaultSkipper,
		},
		{
			name:           "Invalid Request ID",
			requestID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			skipper:        echomiddleware.DefaultSkipper,
		},
		{
			name:           "Skipped Middleware",
			requestID:      "",
			expectedStatus: http.StatusOK,
			skipper:        func(_ *echo.Context) bool { return true },
		},
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	testCases := generateRequestIDTestCases()
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec, req := testutil.SetupEchoContext(t, &testutil.Options{
				Method:        http.MethodPost,
				Path:          "/test",
				Body:          nil,
				Headers:       nil,
				QueryParams:   nil,
				PathParams:    nil,
				ContentType:   "",
				SkipRequestID: true,
			})

			if test.requestID != "" {
				req.Header.Set(echo.HeaderXRequestID, test.requestID)
			}

			mw := middleware.RequestID(test.skipper)
			err := mw(echoSuccessHandler)(ctx)

			if test.expectedStatus == http.StatusOK {
				require.NoError(t, err)

				if !test.skipper(ctx) {
					requestID, ok := ctx.Get(middleware.ContextKeyRequestID).(string)
					require.True(t, ok)
					require.Equal(t, test.requestID, requestID)

					require.Equal(t, test.requestID, rec.Header().Get(middleware.HeaderXRequestID))
				}
			} else {
				var httpErr *echo.HTTPError
				ok := errors.As(err, &httpErr)
				require.True(t, ok)
				require.Equal(t, test.expectedStatus, httpErr.Code)
			}
		})
	}
}

func TestRequestIDWithConfigAutoGenerate(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		name           string
		config         middleware.RequestIDConfig
		requestID      string
		expectedStatus int
		checkGenerated bool
	}{
		{
			name: "AutoGenerate enabled - missing request ID",
			config: middleware.RequestIDConfig{
				Skipper:      echomiddleware.DefaultSkipper,
				AutoGenerate: true,
				Generator:    uuid.NewString,
				Validator:    uuid.Validate,
			},
			requestID:      "",
			expectedStatus: http.StatusOK,
			checkGenerated: true,
		},
		{
			name: "AutoGenerate enabled - valid request ID provided",
			config: middleware.RequestIDConfig{
				Skipper:      echomiddleware.DefaultSkipper,
				AutoGenerate: true,
				Generator:    uuid.NewString,
				Validator:    uuid.Validate,
			},
			requestID:      uuid.New().String(),
			expectedStatus: http.StatusOK,
			checkGenerated: false,
		},
		{
			name: "AutoGenerate enabled - invalid request ID",
			config: middleware.RequestIDConfig{
				Skipper:      echomiddleware.DefaultSkipper,
				AutoGenerate: true,
				Generator:    uuid.NewString,
				Validator:    uuid.Validate,
			},
			requestID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			checkGenerated: false,
		},
		{
			name: "Custom generator",
			config: middleware.RequestIDConfig{
				Skipper:      echomiddleware.DefaultSkipper,
				AutoGenerate: true,
				Generator:    func() string { return "custom-id-12345" },
				Validator:    func(_ string) error { return nil },
			},
			requestID:      "",
			expectedStatus: http.StatusOK,
			checkGenerated: true,
		},
		{
			name: "Custom validator - accept any string",
			config: middleware.RequestIDConfig{
				Skipper:      echomiddleware.DefaultSkipper,
				AutoGenerate: false,
				Generator:    uuid.NewString,
				Validator:    func(_ string) error { return nil },
			},
			requestID:      "any-string-works",
			expectedStatus: http.StatusOK,
			checkGenerated: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec, req := testutil.SetupEchoContext(t, &testutil.Options{
				Method:        http.MethodPost,
				Path:          "/test",
				Body:          nil,
				Headers:       nil,
				QueryParams:   nil,
				PathParams:    nil,
				ContentType:   "",
				SkipRequestID: true,
			})

			if test.requestID != "" {
				req.Header.Set(middleware.HeaderXRequestID, test.requestID)
			}

			mw := middleware.RequestIDWithConfig(test.config)
			err := mw(echoSuccessHandler)(ctx)

			if test.expectedStatus == http.StatusOK {
				require.NoError(t, err)

				requestID, ok := ctx.Get(middleware.ContextKeyRequestID).(string)
				require.True(t, ok)
				require.NotEmpty(t, requestID)

				if test.checkGenerated {
					require.NotEqual(t, test.requestID, requestID)

					require.Equal(t, requestID, rec.Header().Get(middleware.HeaderXRequestID))

					require.Equal(t, requestID, req.Header.Get(middleware.HeaderXRequestID))
				} else if test.requestID != "" {
					require.Equal(t, test.requestID, requestID)
					require.Equal(t, test.requestID, rec.Header().Get(middleware.HeaderXRequestID))
				}
			} else {
				var httpErr *echo.HTTPError
				ok := errors.As(err, &httpErr)
				require.True(t, ok)
				require.Equal(t, test.expectedStatus, httpErr.Code)
			}
		})
	}
}

func TestDefaultRequestIDConfig(t *testing.T) {
	t.Parallel()

	config := middleware.DefaultRequestIDConfig()

	require.NotNil(t, config.Skipper)
	require.NotNil(t, config.Generator)
	require.NotNil(t, config.Validator)
	require.False(t, config.AutoGenerate)

	generatedID := config.Generator()
	require.NoError(t, uuid.Validate(generatedID))

	require.NoError(t, config.Validator(uuid.New().String()))
	require.Error(t, config.Validator("invalid-uuid"))
}
