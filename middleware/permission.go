package middleware

import (
	"errors"
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
)

var (
	ErrPermissionForbidden = errors.New("permission: required permission not found")
	ErrPermissionsEmpty    = errors.New("permission: no permissions configured")
)

func RequirePermission(client, permission string) echo.MiddlewareFunc {
	return RequireAnyPermission(client, permission)
}

func RequireAnyPermission(client string, permissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			if client == "" || len(permissions) == 0 {
				return echo.NewHTTPError(http.StatusInternalServerError, ErrPermissionsEmpty.Error())
			}

			claims, err := GetExtendedClaimsFromContext(ctx)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			if slices.ContainsFunc(permissions, func(permission string) bool {
				return claims.HasResourceRole(client, permission)
			}) {
				return next(ctx)
			}

			return echo.NewHTTPError(http.StatusForbidden, ErrPermissionForbidden.Error())
		}
	}
}
