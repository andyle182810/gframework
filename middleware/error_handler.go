package middleware

import (
	"errors"

	"github.com/labstack/echo/v4"
)

func ErrorHandler(next echo.HTTPErrorHandler) echo.HTTPErrorHandler {
	return func(err error, ectx echo.Context) {
		if ectx.Response().Committed {
			return
		}

		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			_ = ectx.JSON(httpErr.Code, httpErr)

			return
		}

		if next != nil {
			next(err, ectx)
		}
	}
}
