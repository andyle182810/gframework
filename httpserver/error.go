package httpserver

import (
	"errors"
	"net/http"

	"github.com/andyle182810/gframework/i18n"
	"github.com/labstack/echo/v5"
)

// Generic message codes, kept as aliases of the i18n codes the error handler
// reports so both packages cannot drift apart.
const (
	ErrCodeValidation     = i18n.CodeValidation
	ErrCodeNotFound       = i18n.CodeNotFound
	ErrCodeUnauthorized   = i18n.CodeUnauthorized
	ErrCodeForbidden      = i18n.CodeForbidden
	ErrCodeConflict       = i18n.CodeConflict
	ErrCodeInternal       = i18n.CodeInternal
	ErrCodeBadRequest     = i18n.CodeBadRequest
	ErrCodeServiceUnavail = i18n.CodeServiceUnavailable
)

// ErrorResponse is the body every error answers with. It exists so handlers can
// name the real shape in their @Failure annotations — echo.HTTPError declares
// only a message and would document the response as missing its code.
//
// The code is the contract a client branches on; the message is presentation
// and changes with the caller's Accept-Language, so nothing may depend on it.
type ErrorResponse struct {
	Code    string `example:"EMPLOYEE_NOT_FOUND" json:"code"`
	Message string `example:"Employee not found" json:"message"`
}

// HTTPError builds an echo error carrying the text shown to the caller.
//
// details is preferably a message code registered in the server's i18n catalog,
// in which case the error handler reports it as the response code and renders
// the text in the caller's locale:
//
//	httpserver.ConflictError(err, messages.PhoneNumberInUse)
//
// Any other value is treated as literal prose and returned verbatim under the
// generic code for the status. Omitting details falls back to err.Error(),
// which reaches the client as-is — pass a code for anything a user acts on.
func HTTPError(code int, err error, details ...string) *echo.HTTPError {
	message := http.StatusText(code)

	if err != nil {
		message = err.Error()
	}

	if len(details) > 0 {
		message = details[0]
	}

	httpErr := echo.NewHTTPError(code, message)

	if err == nil {
		return httpErr
	}

	// Wrap takes a value receiver and returns a new error instead of mutating,
	// so the result is the only one carrying the cause.
	var wrapped *echo.HTTPError
	if !errors.As(httpErr.Wrap(err), &wrapped) {
		return httpErr
	}

	return wrapped
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
