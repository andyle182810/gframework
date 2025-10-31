package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func RequestID(skipper middleware.Skipper) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()
			rid := strings.TrimSpace(req.Header.Get(HeaderXRequestID))

			if rid == "" {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprint("missing required header: ", HeaderXRequestID),
				)
			}

			if err := uuid.Validate(rid); err != nil {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf("invalid %s: must be a valid UUID", echo.HeaderXRequestID),
				)
			}

			ctx.Set(ContextKeyRequestID, rid)

			return next(ctx)
		}
	}
}
