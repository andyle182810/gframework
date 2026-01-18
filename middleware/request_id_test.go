package middleware_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
)

func echoSuccessHandler(ctx echo.Context) error {
	return ctx.String(http.StatusOK, "success")
}

type RequestIDTestCase struct {
	name           string
	requestID      string
	expectedStatus int
	skipper        func(echo.Context) bool
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
			skipper:        func(_ echo.Context) bool { return true },
		},
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	testCases := generateRequestIDTestCases()
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Set up test context
			ctx, _, req := testutil.SetupEchoContext(t, &testutil.Options{
				Method:        http.MethodPost,
				Path:          "/test",
				Body:          nil,
				Headers:       nil,
				QueryParams:   nil,
				PathParams:    nil,
				ContentType:   "",
				SkipRequestID: true,
			})

			// Set header if provided
			if test.requestID != "" {
				req.Header.Set(echo.HeaderXRequestID, test.requestID)
			}

			// Apply middleware
			mw := middleware.RequestID(test.skipper)
			err := mw(echoSuccessHandler)(ctx)

			// Validate response
			if test.expectedStatus == http.StatusOK {
				require.NoError(t, err)

				if !test.skipper(ctx) {
					requestID, ok := ctx.Get(middleware.ContextKeyRequestID).(string)
					require.True(t, ok)
					require.Equal(t, test.requestID, requestID)
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
