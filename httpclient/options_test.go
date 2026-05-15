package httpclient_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/httpclient"
	"github.com/stretchr/testify/require"
)

const expiryBufferTestRealm = "my-realm"

func newKeycloakStub(t *testing.T, expiresIn int, calls *atomic.Int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/realms/"+expiryBufferTestRealm+"/protocol/openid-connect/token",
		func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(
				w,
				`{"access_token":"svc-token","token_type":"Bearer","expires_in":%d}`,
				expiresIn,
			)
		},
	)

	return httptest.NewServer(mux)
}

func newAPIStub() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestWithAuth_ForwardsExpiryBufferToTokenProvider(t *testing.T) {
	t.Parallel()

	var tokenCalls atomic.Int32

	keycloak := newKeycloakStub(t, 60, &tokenCalls)
	defer keycloak.Close()

	api := newAPIStub()
	defer api.Close()

	client := httpclient.New(api.URL, httpclient.WithAuth(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		BaseURL:      keycloak.URL,
		Realm:        expiryBufferTestRealm,
		ExpiryBuffer: 90 * time.Second,
	}))

	require.NoError(t, client.Get(t.Context(), "/test", nil))
	require.NoError(t, client.Get(t.Context(), "/test", nil))

	require.Equal(t, int32(2), tokenCalls.Load(),
		"ExpiryBuffer larger than token lifetime should force a refresh on every call")
}

func TestWithAuth_DefaultBufferCachesToken(t *testing.T) {
	t.Parallel()

	var tokenCalls atomic.Int32

	keycloak := newKeycloakStub(t, 60, &tokenCalls)
	defer keycloak.Close()

	api := newAPIStub()
	defer api.Close()

	client := httpclient.New(api.URL, httpclient.WithAuth(httpclient.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		BaseURL:      keycloak.URL,
		Realm:        expiryBufferTestRealm,
		ExpiryBuffer: 0,
	}))

	require.NoError(t, client.Get(t.Context(), "/test", nil))
	require.NoError(t, client.Get(t.Context(), "/test", nil))

	require.Equal(t, int32(1), tokenCalls.Load(),
		"default 30s buffer should cache a 60s token across consecutive calls")
}
