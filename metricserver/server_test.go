package metricserver_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/andyle182810/gframework/metricserver"
	"github.com/stretchr/testify/require"
)

func startServer(t *testing.T) *metricserver.Server {
	t.Helper()

	opts := &metricserver.Config{
		Host:         "127.0.0.1",
		Port:         9090,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		GracePeriod:  2 * time.Second,
	}

	server := metricserver.New(opts)
	require.NotNil(t, server)

	go server.Run()

	// Allow server to start
	time.Sleep(500 * time.Millisecond)

	return server
}

func testEndpoint(t *testing.T, url string, expectedStatus int, expectedBody string) {
	t.Helper()

	client := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, expectedStatus, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), expectedBody)

	_ = resp.Body.Close()
}

func shutdownServer(t *testing.T, server *metricserver.Server) {
	t.Helper()

	err := server.Stop(context.Background())
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Ensure server is stopped
	client := &http.Client{
		Timeout:       1 * time.Second,
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:9090/status", nil)
	require.NoError(t, err)

	//nolint:bodyclose
	_, err = client.Do(req)

	require.Error(t, err) // Should fail since server is stopped
}

func TestMetricServer(t *testing.T) {
	t.Parallel()

	server := startServer(t)

	testEndpoint(t, "http://127.0.0.1:9090/status", http.StatusOK, `"status":"ok"`)
	testEndpoint(t, "http://127.0.0.1:9090/metrics", http.StatusOK, "")

	shutdownServer(t, server)
}
