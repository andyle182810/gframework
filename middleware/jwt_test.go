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
	claims := &middleware.ExtendedClaims{
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
	claims := &middleware.ExtendedClaims{
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
	claims := &middleware.ExtendedClaims{
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
	claims := &middleware.ExtendedClaims{
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
	require.Nil(t, config.Logger)
	require.Nil(t, config.Keyfunc)
	require.NotNil(t, config.NewClaimsFunc)
	require.Equal(t, "user", config.ContextKey)
	require.Empty(t, config.TokenLookup)

	claims := config.NewClaimsFunc(nil)
	_, ok := claims.(*middleware.ExtendedClaims)
	require.True(t, ok)
}

func TestExtendedClaims_Getters(t *testing.T) {
	t.Parallel()

	claims := &middleware.ExtendedClaims{
		Azp: "my-authorized-party",
		//nolint:exhaustruct
		RegisteredClaims: jwt.RegisteredClaims{},
	}

	require.Equal(t, "my-authorized-party", claims.GetAzp())
}

func TestJWTWithConfig_NilDefaults(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	claims := &middleware.ExtendedClaims{
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
	}

	mw := middleware.JWTWithConfig(config)
	handler := mw(echoSuccessHandler)

	err := handler(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}
