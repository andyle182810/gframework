package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var errMockKeyfunc = errors.New("mock keyfunc error")

type mockKeyfunc struct {
	key       *rsa.PrivateKey
	shouldErr bool
}

func newMockKeyfunc(t *testing.T) *mockKeyfunc {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return &mockKeyfunc{
		key:       key,
		shouldErr: false,
	}
}

func (m *mockKeyfunc) Keyfunc(_ *jwt.Token) (any, error) {
	if m.shouldErr {
		return nil, errMockKeyfunc
	}

	return &m.key.PublicKey, nil
}

func (m *mockKeyfunc) KeyfuncCtx(_ context.Context) jwt.Keyfunc {
	return m.Keyfunc
}

//nolint:ireturn
func (m *mockKeyfunc) Storage() jwkset.Storage {
	return nil
}

func (m *mockKeyfunc) VerificationKeySet(_ context.Context) (jwt.VerificationKeySet, error) {
	return jwt.VerificationKeySet{}, nil //nolint:exhaustruct
}

func createTestToken(t *testing.T, key *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	tokenString, err := token.SignedString(key)
	require.NoError(t, err)

	return tokenString
}

func TestJWT_ValidToken(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-client-app",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx, rec, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	mw := middleware.JWT(mock)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	storedToken := middleware.GetToken(ctx)
	require.Equal(t, token, storedToken)

	storedClaims, err := middleware.GetExtendedClaimsFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, "my-client-app", storedClaims.GetAzp())
}

func TestJWT_MissingToken(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	mw := middleware.JWT(mock)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestJWT_InvalidToken(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)

	ctx, _, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer invalid-token")

	mw := middleware.JWT(mock)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestJWT_ExpiredToken(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-client-app",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx, _, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	mw := middleware.JWT(mock)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestJWTWithConfig_Skipper(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)

	ctx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/health",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	config := middleware.JWTConfig{
		Skipper: func(ctx *echo.Context) bool {
			return ctx.Request().URL.Path == "/health"
		},
		Logger:        nil,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        "",
		Audiences:     nil,
		Leeway:        0,
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJWTWithConfig_CustomLogger(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-client-app",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := createTestToken(t, mock.key, claims)

	logger := zerolog.Nop()

	ctx, rec, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	config := middleware.JWTConfig{
		Skipper:       nil,
		Logger:        &logger,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        "",
		Audiences:     nil,
		Leeway:        0,
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJWTWithConfig_CustomContextKey(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-client-app",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx, rec, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	config := middleware.JWTConfig{
		Skipper:       nil,
		Logger:        nil,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "jwt-token",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        "",
		Audiences:     nil,
		Leeway:        0,
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	jwtToken, ok := ctx.Get("jwt-token").(*jwt.Token)
	require.True(t, ok)
	require.NotNil(t, jwtToken)
}

func TestDefaultJWTConfig(t *testing.T) {
	t.Parallel()

	config := middleware.DefaultJWTConfig()

	require.NotNil(t, config.Skipper)
	require.NotNil(t, config.Logger)
	require.Nil(t, config.Keyfunc)
	require.NotNil(t, config.NewClaimsFunc)
	require.Equal(t, "user", config.ContextKey)
	require.Empty(t, config.TokenLookup)
	require.Equal(t, middleware.DefaultValidMethods(), config.ValidMethods)
	require.Empty(t, config.Issuer)
	require.Nil(t, config.Audiences)
	require.Equal(t, middleware.DefaultJWTLeeway, config.Leeway)
	require.NotContains(t, config.ValidMethods, "HS256")

	claims := config.NewClaimsFunc(nil)
	_, ok := claims.(*middleware.ExtendedClaims)
	require.True(t, ok)
}

func TestExtendedClaims_Getters(t *testing.T) {
	t.Parallel()

	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-authorized-party",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{},
	}

	require.Equal(t, "my-authorized-party", claims.GetAzp())
}

func TestJWTWithConfig_NilDefaults(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: "my-client-app",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx, rec, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	config := middleware.JWTConfig{
		Skipper:       nil,
		Logger:        nil,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        "",
		Audiences:     nil,
		Leeway:        0,
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJWTWithConfig_DefaultSkipper(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	config := middleware.JWTConfig{
		Skipper:       echomiddleware.DefaultSkipper,
		Logger:        nil,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        "",
		Audiences:     nil,
		Leeway:        0,
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func securityTestConfig(mock *mockKeyfunc, issuer string, audiences []string) middleware.JWTConfig {
	return middleware.JWTConfig{
		Skipper:       nil,
		Logger:        nil,
		Keyfunc:       mock,
		NewClaimsFunc: nil,
		ContextKey:    "",
		TokenLookup:   "",
		ValidMethods:  nil,
		Issuer:        issuer,
		Audiences:     audiences,
		Leeway:        0,
	}
}

func requireUnauthorized(t *testing.T, err error) {
	t.Helper()

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func authContext(t *testing.T, token string) *echo.Context {
	t.Helper()

	ctx, _, _ := testutil.SetupEchoContextWithAuth(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/test",
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	}, "Bearer "+token)

	return ctx
}

func TestJWT_RejectsHS256Token(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := hsToken.SignedString([]byte("attacker-controlled-secret"))
	require.NoError(t, err)

	ctx := authContext(t, tokenString)

	mw := middleware.JWT(mock)
	err = mw(echoSuccessHandler)(ctx)

	requireUnauthorized(t, err)
}

func TestJWT_RejectsTokenWithoutExpiry(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx := authContext(t, token)

	mw := middleware.JWT(mock)
	err := mw(echoSuccessHandler)(ctx)

	requireUnauthorized(t, err)
}

func TestJWTWithConfig_IssuerValidation(t *testing.T) {
	t.Parallel()

	const trustedIssuer = "https://auth.example.com/realms/my-realm"

	tests := []struct {
		name     string
		tokenIss string
		wantOK   bool
	}{
		{name: "matching issuer accepted", tokenIss: trustedIssuer, wantOK: true},
		{name: "foreign issuer rejected", tokenIss: "https://auth.example.com/realms/other", wantOK: false},
		{name: "missing issuer rejected", tokenIss: "", wantOK: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockKeyfunc(t)
			claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
				//nolint:exhaustruct
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    testCase.tokenIss,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			token := createTestToken(t, mock.key, claims)

			ctx := authContext(t, token)

			mw := middleware.JWTWithConfig(securityTestConfig(mock, trustedIssuer, nil))
			err := mw(echoSuccessHandler)(ctx)

			if testCase.wantOK {
				require.NoError(t, err)
			} else {
				requireUnauthorized(t, err)
			}
		})
	}
}

func TestJWTWithConfig_AudienceValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tokenAud jwt.ClaimStrings
		wantOK   bool
	}{
		{name: "matching audience accepted", tokenAud: jwt.ClaimStrings{"my-service"}, wantOK: true},
		{name: "one of several audiences accepted", tokenAud: jwt.ClaimStrings{"other", "my-service"}, wantOK: true},
		{name: "foreign audience rejected", tokenAud: jwt.ClaimStrings{"other-service"}, wantOK: false},
		{name: "missing audience rejected", tokenAud: nil, wantOK: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockKeyfunc(t)
			claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
				//nolint:exhaustruct
				RegisteredClaims: jwt.RegisteredClaims{
					Audience:  testCase.tokenAud,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			token := createTestToken(t, mock.key, claims)

			ctx := authContext(t, token)

			mw := middleware.JWTWithConfig(securityTestConfig(mock, "", []string{"my-service"}))
			err := mw(echoSuccessHandler)(ctx)

			if testCase.wantOK {
				require.NoError(t, err)
			} else {
				requireUnauthorized(t, err)
			}
		})
	}
}

func TestJWTWithConfig_LeewayAllowsClockSkew(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	// Token expired 10 seconds ago: rejected without leeway, accepted with the
	// default 30-second leeway.
	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Second)),
		},
	}
	token := createTestToken(t, mock.key, claims)

	ctx := authContext(t, token)

	mw := middleware.JWT(mock)
	err := mw(echoSuccessHandler)(ctx)

	require.NoError(t, err)
}
