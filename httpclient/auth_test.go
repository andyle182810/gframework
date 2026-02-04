package httpclient_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenProvider_FetchesTokenOnFirstCall(t *testing.T) {
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

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	token, err := provider.GetToken(t.Context())

	require.NoError(t, err)
	require.Equal(t, "test-access-token", token)
}

func TestTokenProvider_ReturnsCachedToken(t *testing.T) {
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

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	token1, err := provider.GetToken(t.Context())
	require.NoError(t, err)

	token2, err := provider.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, token1, token2)
	require.Equal(t, int32(1), callCount.Load())
}

func TestTokenProvider_RefreshesExpiredToken(t *testing.T) {
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

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	_, err := provider.GetToken(t.Context())
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	_, err = provider.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, int32(2), callCount.Load())
}

func TestTokenProvider_HandlesTokenRequestFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "wrong-secret",
		TokenURL:     server.URL,
	})

	_, err := provider.GetToken(t.Context())

	require.ErrorIs(t, err, httpclient.ErrTokenRequestFailed)
}

func TestTokenProvider_HandlesInvalidJSONResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	_, err := provider.GetToken(t.Context())

	require.Error(t, err)
}

func TestTokenProvider_HandlesNetworkError(t *testing.T) {
	t.Parallel()

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     "http://invalid-host-that-does-not-exist.local",
	})

	_, err := provider.GetToken(t.Context())

	require.Error(t, err)
}

func TestTokenProvider_InvalidateTokenClearsCache(t *testing.T) {
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

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	token1, err := provider.GetToken(t.Context())
	require.NoError(t, err)

	provider.InvalidateToken()

	token2, err := provider.GetToken(t.Context())
	require.NoError(t, err)

	require.NotEqual(t, token1, token2)
	require.Equal(t, int32(2), callCount.Load())
}

func TestTokenProvider_ConcurrentRequestsShareToken(t *testing.T) {
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

	provider := httpclient.NewTokenProviderForTest(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     server.URL,
	})

	var wg sync.WaitGroup

	tokens := make([]string, 10)
	errs := make([]error, 10)

	for idx := range 10 {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			tokens[idx], errs[idx] = provider.GetToken(t.Context())
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

func TestWithAuth_AddsAuthorizationHeaderToRequests(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oauth-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer oauth-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
	}))
	defer apiServer.Close()

	client := httpclient.New(apiServer.URL, httpclient.WithAuth(httpclient.AuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     tokenServer.URL,
	}))

	var response map[string]string
	err := client.Get(t.Context(), "/protected", &response)

	require.NoError(t, err)
	require.Equal(t, "authenticated", response["status"])
}
