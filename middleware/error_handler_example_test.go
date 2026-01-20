package middleware_test

import (
	"errors"
	"net/http"
	"os"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

func Example_errorHandlerBasic() {
	e := echo.New()

	e.HTTPErrorHandler = middleware.ErrorHandler(e.HTTPErrorHandler)

	e.GET("/users/:id", func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	})

	_ = e.Start(":8080")
	// Output:
}

func Example_errorHandlerWithLogging() {
	e := echo.New()
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		Logger:    &logger,
		LogErrors: true,
	}
	e.HTTPErrorHandler = middleware.ErrorHandler(e.HTTPErrorHandler, config)

	e.GET("/users/:id", func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	})

	_ = e.Start(":8080")
	// Output:
}

func Example_errorHandlerWithInternalErrors() {
	e := echo.New()
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		Logger:                &logger,
		LogErrors:             true,
		IncludeInternalErrors: true, // WARNING: Only use in development!
	}
	e.HTTPErrorHandler = middleware.ErrorHandler(e.HTTPErrorHandler, config)

	e.GET("/users/:id", func(_ *echo.Context) error {
		dbErr := errors.New("database connection timeout") //nolint:err113
		baseErr := echo.NewHTTPError(http.StatusServiceUnavailable, "Service temporarily unavailable")

		return baseErr.Wrap(dbErr)
	})

	_ = e.Start(":8080")
	// Output:
}

func Example_errorHandlerCustomResponse() {
	e := echo.New()

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		CustomErrorResponse: func(ctx *echo.Context, err error, code int) map[string]any {
			return map[string]any{
				"success": false,
				"error": map[string]any{
					"code":    code,
					"message": err.Error(),
				},
				"path":      ctx.Request().URL.Path,
				"timestamp": "2024-01-01T00:00:00Z",
			}
		},
	}
	e.HTTPErrorHandler = middleware.ErrorHandler(e.HTTPErrorHandler, config)

	e.GET("/users/:id", func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	})

	_ = e.Start(":8080")
	// Output:
}

func Example_errorHandlerProduction() {
	e := echo.New()
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &middleware.ErrorHandlerConfig{ //nolint:exhaustruct
		Logger:                &logger,
		LogErrors:             true,
		IncludeInternalErrors: false,
	}
	e.HTTPErrorHandler = middleware.ErrorHandler(e.HTTPErrorHandler, config)

	e.GET("/users/:id", func(_ *echo.Context) error {
		dbErr := errors.New("database connection timeout") //nolint:err113
		baseErr := echo.NewHTTPError(http.StatusServiceUnavailable, "Service temporarily unavailable")

		return baseErr.Wrap(dbErr)
	})

	_ = e.Start(":8080")
	// Output:
}
