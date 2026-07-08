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
		Host:              "127.0.0.1",
		Port:              0,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       2 * time.Second,
		GracePeriod:       2 * time.Second,
	}

	server := metricserver.New(opts)
	require.NotNil(t, server)

	require.NoError(t, server.Start(t.Context()))

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

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
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

func shutdownServer(t *testing.T, server *metricserver.Server, address string) {
	t.Helper()

	err := server.Stop()
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Ensure server is stopped
	client := &http.Client{
		Timeout:       1 * time.Second,
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/status", nil)
	require.NoError(t, err)

	//nolint:bodyclose
	_, err = client.Do(req)

	require.Error(t, err) // Should fail since server is stopped
}

func TestMetricServer(t *testing.T) {
	t.Parallel()

	server := startServer(t)
	address := server.Address()

	testEndpoint(t, "http://"+address+"/status", http.StatusOK, `"status":"ok"`)
	testEndpoint(t, "http://"+address+"/metrics", http.StatusOK, "")

	shutdownServer(t, server, address)
}
