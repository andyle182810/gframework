package middleware_test

import (
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

const (
	permissionClient = "vendor-service"
	vendorsRead      = "vendors:read"
)

type permissionCase struct {
	name        string
	client      string
	permissions []string
	claims      *middleware.ExtendedClaims
	wantCode    int
}

func permissionContext(t *testing.T, claims *middleware.ExtendedClaims) *echo.Context {
	t.Helper()

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          apiTestPath,
		Body:          nil,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	if claims != nil {
		ctx.Set(middleware.ContextKeyClaims, claims)
	}

	return ctx
}

func claimsWithResourceRoles(access middleware.ResourceAccess) *middleware.ExtendedClaims {
	return &middleware.ExtendedClaims{ResourceAccess: access} //nolint:exhaustruct
}

func permissionCases() []permissionCase {
	return []permissionCase{
		{
			name:        "holds the permission",
			client:      permissionClient,
			permissions: []string{vendorsRead},
			claims: claimsWithResourceRoles(middleware.ResourceAccess{
				permissionClient: {Roles: []string{vendorsRead}},
			}),
			wantCode: 0,
		},
		{
			name:        "holds one of several",
			client:      permissionClient,
			permissions: []string{"vendors:read:all", "vendors:read:assigned"},
			claims: claimsWithResourceRoles(middleware.ResourceAccess{
				permissionClient: {Roles: []string{"vendors:read:assigned"}},
			}),
			wantCode: 0,
		},
		{
			name:        "missing the permission",
			client:      permissionClient,
			permissions: []string{"vendors:write"},
			claims: claimsWithResourceRoles(middleware.ResourceAccess{
				permissionClient: {Roles: []string{vendorsRead}},
			}),
			wantCode: http.StatusForbidden,
		},
		{
			name:        "same role on another client",
			client:      permissionClient,
			permissions: []string{vendorsRead},
			claims: claimsWithResourceRoles(middleware.ResourceAccess{
				"product-service": {Roles: []string{vendorsRead}},
			}),
			wantCode: http.StatusForbidden,
		},
		{
			name:        "realm role of the same name does not count",
			client:      permissionClient,
			permissions: []string{vendorsRead},
			claims: &middleware.ExtendedClaims{ //nolint:exhaustruct
				RealmAccess: middleware.RoleAccess{Roles: []string{vendorsRead}},
			},
			wantCode: http.StatusForbidden,
		},
		{
			name:        "no claims",
			client:      permissionClient,
			permissions: []string{vendorsRead},
			claims:      nil,
			wantCode:    http.StatusUnauthorized,
		},
		{
			name:        "no permissions configured",
			client:      permissionClient,
			permissions: nil,
			claims:      claimsWithResourceRoles(nil),
			wantCode:    http.StatusInternalServerError,
		},
		{
			name:        "no client configured",
			client:      "",
			permissions: []string{vendorsRead},
			claims:      claimsWithResourceRoles(nil),
			wantCode:    http.StatusInternalServerError,
		},
	}
}

func TestRequireAnyPermission(t *testing.T) {
	t.Parallel()

	for _, tt := range permissionCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := permissionContext(t, tt.claims)

			err := middleware.RequireAnyPermission(tt.client, tt.permissions...)(echoSuccessHandler)(ctx)

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

func TestRequirePermission(t *testing.T) {
	t.Parallel()

	ctx := permissionContext(t, claimsWithResourceRoles(middleware.ResourceAccess{
		permissionClient: {Roles: []string{vendorsRead}},
	}))

	require.NoError(t, middleware.RequirePermission(permissionClient, "vendors:read")(echoSuccessHandler)(ctx))
}
