package middleware_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/andyle182810/gframework/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

func ExampleRequestID() {
	e := echo.New()

	e.Use(middleware.RequestID(echomiddleware.DefaultSkipper))

	e.GET("/api/users", func(c *echo.Context) error {
		requestID := middleware.GetRequestID(c)

		return c.JSON(http.StatusOK, map[string]string{
			"request_id": requestID,
			"message":    "User list",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set(middleware.HeaderXRequestID, uuid.New().String())

	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output: Status: 200
}

func ExampleRequestIDWithConfig_autoGenerate() {
	e := echo.New()

	config := middleware.RequestIDConfig{
		Skipper:      echomiddleware.DefaultSkipper,
		AutoGenerate: true,
		Generator:    uuid.NewString,
		Validator:    uuid.Validate,
	}
	e.Use(middleware.RequestIDWithConfig(config))

	e.GET("/api/users", func(c *echo.Context) error {
		requestID := middleware.GetRequestID(c)

		return c.JSON(http.StatusOK, map[string]string{
			"request_id": requestID,
			"message":    "User list",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	fmt.Println("Has Request ID in response:", rec.Header().Get(middleware.HeaderXRequestID) != "")
	// Output:
	// Status: 200
	// Has Request ID in response: true
}

func ExampleRequestIDWithConfig_custom() {
	e := echo.New()

	config := middleware.RequestIDConfig{
		Skipper:      echomiddleware.DefaultSkipper,
		AutoGenerate: true,
		Generator: func() string {
			return fmt.Sprintf("REQ-%d-%s", 1234567890, uuid.New().String()[:8])
		},
		Validator: func(id string) error {
			if len(id) == 0 {
				return errors.New("request ID cannot be empty") //nolint:err113
			}

			return nil
		},
	}
	e.Use(middleware.RequestIDWithConfig(config))

	e.GET("/api/users", func(c *echo.Context) error {
		requestID := middleware.GetRequestID(c)

		return c.JSON(http.StatusOK, map[string]string{
			"request_id": requestID,
			"message":    "User list",
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set(middleware.HeaderXRequestID, "CUSTOM-123-ABC")

	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output: Status: 200
}
