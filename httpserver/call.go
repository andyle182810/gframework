//nolint:nonamedreturns
package httpserver

import (
	"net/http"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type HandlerFunc[REQ any, RES any] func(
	log zerolog.Logger,
	c *echo.Context,
	request *REQ,
) (*HandlerResponse[RES], *echo.HTTPError)

func ExecuteStandardized[REQ any, RES any](
	c *echo.Context,
	request *REQ,
	handlerName string,
	delegate HandlerFunc[REQ, RES],
) (resp any, httpErr *echo.HTTPError) {
	// Set handler name in context for error logging
	c.Set(middleware.ContextKeyHandler, handlerName)

	log := zerolog.Ctx(c.Request().Context()).With().Str("handler", handlerName).Logger()

	// Protect against handler panics crashing the server
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Msg("Handler panicked")

			httpErr = echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
		}
	}()

	if c.Request().Context().Err() != nil {
		return nil, echo.NewHTTPError(499, "client closed request") //nolint:mnd
	}

	internalResponse, delegateError := delegate(log, c, request)
	if delegateError != nil {
		return nil, delegateError
	}

	requestID := middleware.GetRequestID(c)

	finalPayload := &APIResponse[RES]{
		RequestID:  requestID,
		Data:       internalResponse.Data,
		Pagination: internalResponse.Pagination,
	}

	return finalPayload, nil
}
