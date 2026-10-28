package middleware_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

const (
	apiTestPath       = "/api/test"
	apiGatewayService = "api-gateway-service"
)

func internalToken(t *testing.T, key *mockKeyfunc, azp string) string {
	t.Helper()

	return createTestToken(t, key.key, &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp: azp,
		RegisteredClaims: jwt.RegisteredClaims{ //nolint:exhaustruct
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
}

func internalAuthContext(t *testing.T, header string) *echo.Context {
	t.Helper()

	headers := map[string]string{}

	if header != "" {
		headers[middleware.HeaderXInternalAuthorization] = header
	}

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          apiTestPath,
		Body:          nil,
		Headers:       headers,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	return ctx
}

func TestInternalAuth_AllowedClient(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	token := internalToken(t, mock, apiGatewayService)
	ctx := internalAuthContext(t, "Bearer "+token)

	mw := middleware.InternalAuth(mock, []string{apiGatewayService, "purchase-order-service"})

	err := mw(echoSuccessHandler)(ctx)

	require.NoError(t, err)
}

func TestInternalAuth_ClientNotAllowed(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	token := internalToken(t, mock, "operations-portal")
	ctx := internalAuthContext(t, "Bearer "+token)

	mw := middleware.InternalAuth(mock, []string{apiGatewayService})

	err := mw(echoSuccessHandler)(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestInternalAuth_MissingHeader(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	ctx := internalAuthContext(t, "")

	mw := middleware.InternalAuth(mock, []string{apiGatewayService})

	err := mw(echoSuccessHandler)(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestInternalAuth_InvalidToken(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	ctx := internalAuthContext(t, "Bearer not-a-real-token")

	mw := middleware.InternalAuth(mock, []string{apiGatewayService})

	err := mw(echoSuccessHandler)(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestInternalAuth_EmptyAllowlistRejects(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	token := internalToken(t, mock, apiGatewayService)
	ctx := internalAuthContext(t, "Bearer "+token)

	mw := middleware.InternalAuth(mock, nil)

	err := mw(echoSuccessHandler)(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestInternalAuthWithRealmRole(t *testing.T) {
	t.Parallel()

	const internalService = "internal_service"

	tests := []struct {
		name           string
		realmRoles     []string
		allowedClients []string
		azp            string
		wantCode       int
	}{
		{
			name:           "holds the role",
			realmRoles:     []string{internalService},
			allowedClients: nil,
			azp:            permissionClient,
			wantCode:       0,
		},
		{
			name:           "missing the role",
			realmRoles:     []string{"administrator"},
			allowedClients: nil,
			azp:            "operations-portal",
			wantCode:       http.StatusForbidden,
		},
		{
			name:           "role and allowlist both required",
			realmRoles:     []string{internalService},
			allowedClients: []string{apiGatewayService},
			azp:            permissionClient,
			wantCode:       http.StatusForbidden,
		},
		{
			name:           "role and allowlist both met",
			realmRoles:     []string{internalService},
			allowedClients: []string{apiGatewayService},
			azp:            apiGatewayService,
			wantCode:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockKeyfunc(t)
			token := createTestToken(t, mock.key, &middleware.ExtendedClaims{ //nolint:exhaustruct
				Azp:         tt.azp,
				RealmAccess: middleware.RoleAccess{Roles: tt.realmRoles},
				RegisteredClaims: jwt.RegisteredClaims{ //nolint:exhaustruct
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			})
			ctx := internalAuthContext(t, "Bearer "+token)

			config := middleware.DefaultInternalAuthConfig()
			config.Keyfunc = mock
			config.RequiredRealmRole = internalService
			config.AllowedClients = tt.allowedClients

			err := middleware.InternalAuthWithConfig(config)(echoSuccessHandler)(ctx)

			if tt.wantCode == 0 {
				require.NoError(t, err)

				return
			}

			var httpErr *echo.HTTPError

			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, tt.wantCode, httpErr.Code)
		})
	}
}

func TestInternalAuthWithRealmRole_Constructor(t *testing.T) {
	t.Parallel()

	mock := newMockKeyfunc(t)
	token := createTestToken(t, mock.key, &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp:         permissionClient,
		RealmAccess: middleware.RoleAccess{Roles: []string{"internal_service"}},
		RegisteredClaims: jwt.RegisteredClaims{ //nolint:exhaustruct
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	ctx := internalAuthContext(t, "Bearer "+token)

	require.NoError(t, middleware.InternalAuthWithRealmRole(mock, "internal_service")(echoSuccessHandler)(ctx))
}
