package middleware

import (
	"errors"
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
)

var (
	ErrRealmRoleForbidden = errors.New("realm role: required role not found")
	ErrRealmRolesEmpty    = errors.New("realm role: no roles configured")
)

func RequireAnyRealmRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if len(roles) == 0 {
				return echo.NewHTTPError(http.StatusInternalServerError, ErrRealmRolesEmpty.Error())
			}

			claims, err := GetExtendedClaimsFromContext(ctx)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			if slices.ContainsFunc(roles, claims.HasRealmRole) {
				return next(ctx)
			}

			return echo.NewHTTPError(http.StatusForbidden, ErrRealmRoleForbidden.Error())
		}
	}
}
