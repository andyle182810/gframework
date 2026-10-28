package keycloaktest

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/assert"
)

type Principal struct {
	Name   string
	Claims *middleware.ExtendedClaims
}

type RouteAccess struct {
	Method  string
	Path    string
	Allowed []string
}

func (r *Realm) UserPrincipal(realmRoles ...string) Principal {
	return Principal{
		Name:   strings.Join(realmRoles, "+"),
		Claims: r.UserClaims("user-"+strings.Join(realmRoles, "-"), realmRoles...),
	}
}

func (r *Realm) ServiceAccountPrincipal(t *testing.T, clientID string) Principal {
	t.Helper()

	return Principal{Name: clientID, Claims: r.ServiceAccountClaims(t, clientID)}
}

func AssertRouteAccess(t *testing.T, register func(api *echo.Group), principals []Principal, routes []RouteAccess) {
	t.Helper()

	for _, principal := range principals {
		e := echo.New()
		e.Use(echomw.Recover())

		api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(ctx *echo.Context) error {
				ctx.Set(middleware.ContextKeyClaims, principal.Claims)

				return next(ctx)
			}
		})
		register(api)

		// An allowed request only has to get past the route's middleware: handlers run on
		// zero-value dependencies, so whatever they return (or a recovered panic) counts.
		for _, route := range routes {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), route.Method, route.Path, nil))

			if slices.Contains(route.Allowed, principal.Name) {
				assert.NotContains(t, []int{
					http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed,
				}, rec.Code, "%s %s as %s should be allowed", route.Method, route.Path, principal.Name)

				continue
			}

			assert.Equal(t, http.StatusForbidden, rec.Code,
				"%s %s as %s should be forbidden", route.Method, route.Path, principal.Name)
		}
	}
}
