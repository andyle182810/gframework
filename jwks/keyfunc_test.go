package jwks_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/jwks"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestJWKS(t *testing.T) (*rsa.PrivateKey, jwk.Set) { //nolint:ireturn
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwkKey, err := jwk.Import(privateKey.PublicKey)
	require.NoError(t, err)

	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)

	err = jwkKey.Set(jwk.AlgorithmKey, "RS256")
	require.NoError(t, err)

	err = jwkKey.Set(jwk.KeyUsageKey, "sig")
	require.NoError(t, err)

	keySet := jwk.NewSet()
	err = keySet.AddKey(jwkKey)
	require.NoError(t, err)

	return privateKey, keySet
}

func createJWKSServer(t *testing.T, keySet jwk.Set) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jwksBytes, marshalErr := json.Marshal(keySet)
		assert.NoError(t, marshalErr)

		_, writeErr := w.Write(jwksBytes)
		assert.NoError(t, writeErr)
	}))
}

func TestNew_ValidEndpoint(t *testing.T) {
	t.Parallel()

	_, keySet := createTestJWKS(t)
	server := createJWKSServer(t, keySet)

	defer server.Close()

	keyFuncResult, err := jwks.New(t.Context(), []string{server.URL})
	require.NoError(t, err)
	require.NotNil(t, keyFuncResult)
}

func TestNew_InvalidJWKSResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invalid": "response"}`))
	}))

	defer server.Close()

	keyFuncResult, err := jwks.New(t.Context(), []string{server.URL})
	require.NoError(t, err)
	require.NotNil(t, keyFuncResult)
}

func TestNew_EmptyURLs(t *testing.T) {
	t.Parallel()

	keyFuncResult, err := jwks.New(t.Context(), []string{})
	require.Error(t, err)
	require.Nil(t, keyFuncResult)
}

func TestNew_CustomOptions(t *testing.T) {
	t.Parallel()

	_, keySet := createTestJWKS(t)
	server := createJWKSServer(t, keySet)

	defer server.Close()

	customClient := &http.Client{Timeout: 5 * time.Second} //nolint:exhaustruct

	keyFuncResult, err := jwks.New(t.Context(), []string{server.URL},
		jwks.WithHTTPClient(customClient),
		jwks.WithRateLimitBurst(10),
		jwks.WithRefreshTimeout(15*time.Second),
		jwks.WithRefreshInterval(30*time.Minute),
		jwks.WithRateLimitWaitMax(5*time.Second),
	)
	require.NoError(t, err)
	require.NotNil(t, keyFuncResult)
}

func TestNew_ValidatesJWT(t *testing.T) {
	t.Parallel()

	privateKey, keySet := createTestJWKS(t)
	server := createJWKSServer(t, keySet)

	defer server.Close()

	keyFuncResult, err := jwks.New(t.Context(), []string{server.URL})
	require.NoError(t, err)
	require.NotNil(t, keyFuncResult)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "1234567890",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "test-key-id"

	signedToken, err := token.SignedString(privateKey)
	require.NoError(t, err)

	parsedToken, err := jwt.Parse(signedToken, keyFuncResult.Keyfunc)
	require.NoError(t, err)
	require.True(t, parsedToken.Valid)
}
