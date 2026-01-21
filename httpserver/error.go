package httpserver

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

const (
	ErrCodeValidation     = "VALIDATION_ERROR"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
	ErrCodeForbidden      = "FORBIDDEN"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeBadRequest     = "BAD_REQUEST"
	ErrCodeServiceUnavail = "SERVICE_UNAVAILABLE"
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

func BadRequestError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusBadRequest, err, details...)
}

func NotFoundError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusNotFound, err, details...)
}

func UnauthorizedError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusUnauthorized, err, details...)
}

func ForbiddenError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusForbidden, err, details...)
}

func ConflictError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusConflict, err, details...)
}

func InternalError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusInternalServerError, err, details...)
}

func ServiceUnavailableError(err error, details ...string) *echo.HTTPError {
	return HTTPError(http.StatusServiceUnavailable, err, details...)
}
