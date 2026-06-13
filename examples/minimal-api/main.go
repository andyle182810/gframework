// The smallest possible gframework service: an HTTP server with a health
// endpoint, wired through the runner for graceful startup and shutdown.
//
// Run it:
//
//	go run .
//	curl http://localhost:8080/health
//
// Stop it with Ctrl+C — the runner drains in-flight requests before exiting.
package main

import (
	"net/http"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/runner"
	"github.com/labstack/echo/v5"
)

func main() {
	server := httpserver.New(&httpserver.Config{ //nolint:exhaustruct
		Host:        "0.0.0.0",
		Port:        8080,
		BodyLimit:   "10M",
		GracePeriod: 10 * time.Second,
		// Timeouts left at zero use the secure defaults:
		// ReadHeaderTimeout 10s, ReadTimeout 30s, WriteTimeout 30s, IdleTimeout 120s.
	})

	server.Root.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	server.Root.GET("/hello/:name", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "hello, " + c.Param("name"),
		})
	})

	// Run blocks until SIGINT/SIGTERM or a service failure. If the port is
	// already in use, Run logs the bind error and exits non-zero immediately.
	runner.New(
		runner.WithCoreService(server),
	).Run()
}
