package httpserver

import (
	"fmt"

	"github.com/labstack/echo/v5"
)

func HTTPError(code int, err error, details ...string) *echo.HTTPError {
	message := err.Error()

	if len(details) > 0 {
		message = fmt.Sprintf("%s: %s", message, details[0])
	}

	httpErr := echo.NewHTTPError(code, message)
	_ = httpErr.Wrap(err)

	return httpErr
}
