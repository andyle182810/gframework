package keycloak_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andyle182810/gframework/keycloak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUMAClient_Check_Allowed(t *testing.T) {
	t.Parallel()

	var (
		gotAuth        string
		gotContentType string
		gotGrantType   string
		gotAudience    string
		gotPermission  string
		gotMode        string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		assert.NoError(t, r.ParseForm())

		gotGrantType = r.Form.Get("grant_type")
		gotAudience = r.Form.Get("audience")
		gotPermission = r.Form.Get("permission")
		gotMode = r.Form.Get("response_mode")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := keycloak.NewUMAClient(server.URL, "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.NoError(t, err)
	require.True(t, allowed)

	assert.Equal(t, "Bearer user-jwt", gotAuth)
	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	assert.Equal(t, "urn:ietf:params:oauth:grant-type:uma-ticket", gotGrantType)
	assert.Equal(t, "resource-server", gotAudience)
	assert.Equal(t, "doc:42#read", gotPermission)
	assert.Equal(t, "decision", gotMode)
}

func TestUMAClient_Check_DeniedOn403(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := keycloak.NewUMAClient(server.URL, "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.NoError(t, err)
	require.False(t, allowed)
}

func TestUMAClient_Check_InvalidResponseOn401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_token"}`)
	}))
	defer server.Close()

	client := keycloak.NewUMAClient(server.URL, "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.Error(t, err)
	require.False(t, allowed)
	require.ErrorIs(t, err, keycloak.ErrUMAResponseInvalid)
	assert.Contains(t, err.Error(), "status=401")
}

func TestUMAClient_Check_InvalidResponseOn500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	client := keycloak.NewUMAClient(server.URL, "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.Error(t, err)
	require.False(t, allowed)
	require.ErrorIs(t, err, keycloak.ErrUMAResponseInvalid)
	assert.Contains(t, err.Error(), "status=500")
	assert.Contains(t, err.Error(), "boom")
}

func TestUMAClient_Check_TransportError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	server.Close()

	client := keycloak.NewUMAClient(server.URL, "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.Error(t, err)
	require.False(t, allowed)
	require.ErrorIs(t, err, keycloak.ErrUMARequestFailed)
}

func TestUMAClient_Check_InvalidInput(t *testing.T) {
	t.Parallel()

	client := keycloak.NewUMAClient("http://ignored", "resource-server")

	cases := []struct {
		name              string
		token, res, scope string
	}{
		{"empty token", "", "doc:1", "read"},
		{"empty resource", "tok", "", "read"},
		{"empty scope", "tok", "doc:1", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			allowed, err := client.Check(t.Context(), tc.token, tc.res, tc.scope)
			require.Error(t, err)
			require.False(t, allowed)
			require.ErrorIs(t, err, keycloak.ErrInvalidInput)
		})
	}
}

func TestUMAClient_Check_WithUMAHTTPClient(t *testing.T) {
	t.Parallel()

	var seen bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	custom := &http.Client{} //nolint:exhaustruct

	client := keycloak.NewUMAClient(server.URL, "resource-server", keycloak.WithUMAHTTPClient(custom))

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.NoError(t, err)
	require.True(t, allowed)
	require.True(t, seen, "expected request to be routed through the custom client")
}

func TestUMAClient_Check_BuildRequestError(t *testing.T) {
	t.Parallel()

	// An invalid URL ("://bad") causes http.NewRequestWithContext to fail.
	client := keycloak.NewUMAClient("://bad", "resource-server")

	allowed, err := client.Check(t.Context(), "user-jwt", "doc:42", "read")

	require.Error(t, err)
	require.False(t, allowed)
	require.ErrorIs(t, err, keycloak.ErrUMARequestFailed)
}
