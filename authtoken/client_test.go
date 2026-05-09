package authtoken_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/andyle182810/gframework/authtoken"
	"github.com/stretchr/testify/require"
)

const testRealm = "my-realm"

type tokenStub struct {
	tokenCalls atomic.Int32
	tokenDelay time.Duration
	expiresIn  int
	token      string
	statusCode int
	body       string
}

func newTokenStub() *tokenStub {
	return &tokenStub{
		tokenCalls: atomic.Int32{},
		tokenDelay: 0,
		expiresIn:  3600,
		token:      "svc-token",
		statusCode: http.StatusOK,
		body:       "",
	}
}

func (s *tokenStub) newServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/"+testRealm+"/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		s.handleToken(t, w, r)
	})

	return httptest.NewServer(mux)
}

func (s *tokenStub) handleToken(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	s.tokenCalls.Add(1)

	if s.tokenDelay > 0 {
		time.Sleep(s.tokenDelay)
	}

	require.Equal(t, http.MethodPost, r.Method)

	err := r.ParseForm()
	require.NoError(t, err)
	require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
	require.Equal(t, "test-client", r.Form.Get("client_id"))

	if s.statusCode != http.StatusOK {
		w.WriteHeader(s.statusCode)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if s.body != "" {
		_, _ = w.Write([]byte(s.body))

		return
	}

	_, err = fmt.Fprintf(
		w,
		`{"access_token":%q,"token_type":"Bearer","expires_in":%d}`,
		s.token,
		s.expiresIn,
	)
	require.NoError(t, err)
}

func newClient(baseURL string) *authtoken.Client {
	return authtoken.New(baseURL, testRealm, "test-client", "test-secret")
}

func TestClient_UsesInjectedGoCloakClient(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()

	server := stub.newServer(t)
	defer server.Close()

	client := authtoken.New(
		"http://invalid-host-that-does-not-exist.local",
		testRealm,
		"test-client",
		"test-secret",
		authtoken.WithGoCloakClient(gocloak.NewClient(server.URL)),
	)

	token, err := client.GetToken(t.Context())

	require.NoError(t, err)
	require.Equal(t, "svc-token", token)
	require.Equal(t, int32(1), stub.tokenCalls.Load())
}

func TestClient_FetchesTokenOnFirstCall(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	token, err := client.GetToken(t.Context())

	require.NoError(t, err)
	require.Equal(t, "svc-token", token)
	require.Equal(t, int32(1), stub.tokenCalls.Load())
}

func TestClient_ReturnsCachedToken(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	token1, err := client.GetToken(t.Context())
	require.NoError(t, err)

	token2, err := client.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, token1, token2)
	require.Equal(t, int32(1), stub.tokenCalls.Load())
}

func TestClient_RefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.expiresIn = 1

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	_, err := client.GetToken(t.Context())
	require.NoError(t, err)

	_, err = client.GetToken(t.Context())
	require.NoError(t, err)

	require.Equal(t, int32(2), stub.tokenCalls.Load())
}

func TestClient_HandlesTokenRequestFailure(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.statusCode = http.StatusUnauthorized

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}

func TestClient_HandlesInvalidJSONResponse(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.body = "invalid json"

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}

func TestClient_HandlesNetworkError(t *testing.T) {
	t.Parallel()

	client := newClient("http://invalid-host-that-does-not-exist.local")

	_, err := client.GetToken(t.Context())

	require.Error(t, err)
}

func TestClient_InvalidateTokenClearsCache(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	token1, err := client.GetToken(t.Context())
	require.NoError(t, err)

	client.InvalidateToken()

	stub.token = "svc-token-2"

	token2, err := client.GetToken(t.Context())
	require.NoError(t, err)

	require.NotEqual(t, token1, token2)
	require.Equal(t, int32(2), stub.tokenCalls.Load())
}

func TestClient_ConcurrentRequestsShareToken(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.tokenDelay = 50 * time.Millisecond

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

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
		require.Equal(t, "svc-token", token, "goroutine %d", idx)
	}

	require.LessOrEqual(t, stub.tokenCalls.Load(), int32(2))
}

func TestClient_HandlesEmptyAccessToken(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.token = ""

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	_, err := client.GetToken(t.Context())

	require.ErrorIs(t, err, authtoken.ErrNoAccessToken)
}

func TestClient_ReturnsUpdatedTokenAfterExpiry(t *testing.T) {
	t.Parallel()

	stub := newTokenStub()
	stub.expiresIn = 1

	server := stub.newServer(t)
	defer server.Close()

	client := newClient(server.URL)

	firstToken, err := client.GetToken(t.Context())
	require.NoError(t, err)

	stub.token = firstToken + "-refreshed"

	secondToken, err := client.GetToken(t.Context())
	require.NoError(t, err)

	require.NotEqual(t, firstToken, secondToken)
	require.Equal(t, int32(2), stub.tokenCalls.Load())
}
