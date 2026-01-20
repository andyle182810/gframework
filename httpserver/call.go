package httpserver

import (
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
) (any, *echo.HTTPError) {
	log := zerolog.Ctx(c.Request().Context()).With().Str("handler", handlerName).Logger()

	internalResponse, delegateError := delegate(log, c, request)
	if delegateError != nil {
		log.Error().
			Int("status_code", delegateError.Code).
			Interface("error_cause", delegateError.Unwrap()).
			Interface("error_message", delegateError.Message).
			Msg("The request has failed with an HTTP error")

		return nil, delegateError
	}

	requestID, _ := c.Get(middleware.ContextKeyRequestID).(string)

	finalPayload := &APIResponse[RES]{
		RequestID:  requestID,
		Data:       internalResponse.Data,
		Pagination: internalResponse.Pagination,
	}

	return finalPayload, nil
}
