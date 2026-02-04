package authtoken_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/authtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_FetchesTokenOnFirstCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		assert.NoError(t, err)

		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "test-client", r.Form.Get("client_id"))
		assert.Equal(t, "test-secret", r.Form.Get("client_secret"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	token, err := client.GetToken(t.Context())

	require.NoError(t, err)
	require.Equal(t, "test-access-token", token)
}

func TestClient_ReturnsCachedToken(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "cached-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	token1, err := client.GetToken(t.Context())
	require.NoError(t, err)

	token2, err := client.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, token1, token2)
	require.Equal(t, int32(1), callCount.Load())
}

func TestClient_RefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", count),
			"token_type":   "Bearer",
			"expires_in":   1, // Expires in 1 second (immediately due to 30s buffer)
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	_, err := client.GetToken(t.Context())
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	_, err = client.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, int32(2), callCount.Load())
}

func TestClient_HandlesTokenRequestFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "wrong-secret")

	_, err := client.GetToken(t.Context())

	require.ErrorIs(t, err, authtoken.ErrTokenRequestFailed)
}

func TestClient_HandlesInvalidJSONResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}

func TestClient_HandlesNetworkError(t *testing.T) {
	t.Parallel()

	client := authtoken.New(
		"http://invalid-host-that-does-not-exist.local",
		"test-client",
		"test-secret",
	)

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}

func TestClient_InvalidateTokenClearsCache(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", count),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	token1, err := client.GetToken(t.Context())
	require.NoError(t, err)

	client.InvalidateToken()

	token2, err := client.GetToken(t.Context())
	require.NoError(t, err)

	require.NotEqual(t, token1, token2)
	require.Equal(t, int32(2), callCount.Load())
}

func TestClient_ConcurrentRequestsShareToken(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow token endpoint

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "concurrent-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	var wg sync.WaitGroup

	tokens := make([]string, 10)
	errs := make([]error, 10)

	for idx := range 10 {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			tokens[idx], errs[idx] = client.GetToken(t.Context())
		}(idx)
	}

	wg.Wait()

	for idx, err := range errs {
		require.NoError(t, err, "goroutine %d", idx)
	}

	for idx, token := range tokens {
		require.Equal(t, "concurrent-token", token, "goroutine %d", idx)
	}

	require.LessOrEqual(t, callCount.Load(), int32(2))
}

func TestClient_HandlesEmptyAccessToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(server.URL, "test-client", "test-secret")

	_, err := client.GetToken(t.Context())

	require.ErrorIs(t, err, authtoken.ErrNoAccessToken)
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{ //nolint:exhaustruct
		Timeout: 5 * time.Second,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "custom-client-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(
		server.URL,
		"test-client",
		"test-secret",
		authtoken.WithHTTPClient(customClient),
	)

	token, err := client.GetToken(t.Context())

	require.NoError(t, err)
	require.Equal(t, "custom-client-token", token)
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "slow-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := authtoken.New(
		server.URL,
		"test-client",
		"test-secret",
		authtoken.WithTimeout(50*time.Millisecond),
	)

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}
