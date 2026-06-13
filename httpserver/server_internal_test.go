package httpserver

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

const loopbackHost = "127.0.0.1"

func TestNew_AppliesSecureTimeoutDefaults(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	require.Equal(t, DefaultReadHeaderTimeout, srv.httpServer.ReadHeaderTimeout)
	require.Equal(t, DefaultReadTimeout, srv.httpServer.ReadTimeout)
	require.Equal(t, DefaultWriteTimeout, srv.httpServer.WriteTimeout)
	require.Equal(t, DefaultIdleTimeout, srv.httpServer.IdleTimeout)
	require.Equal(t, DefaultGracePeriod, srv.gracePeriod)
}

func TestNew_ExplicitTimeoutsAreKept(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host:              loopbackHost,
		Port:              0,
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	require.Equal(t, 1*time.Second, srv.httpServer.ReadHeaderTimeout)
	require.Equal(t, 2*time.Second, srv.httpServer.ReadTimeout)
	require.Equal(t, 3*time.Second, srv.httpServer.WriteTimeout)
	require.Equal(t, 4*time.Second, srv.httpServer.IdleTimeout)
}

func TestStart_ReturnsErrorWhenPortInUse(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig

	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = listener.Close() })

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: addr.Port,
	})

	err = srv.Start(t.Context())

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to bind")
}

func TestStart_ServesRequests(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
	})

	srv.Root.GET("/health", func(ctx *echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	resp, err := http.Get("http://" + srv.Address() + "/health") //nolint:noctx
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
