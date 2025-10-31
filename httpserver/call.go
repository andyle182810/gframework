package httpserver

import (
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/notifylog"
	"github.com/labstack/echo/v4"
)

type HandlerFunc[REQ any, RES any] func(log notifylog.NotifyLog, c echo.Context, request *REQ) (*HandlerResponse[RES], *echo.HTTPError)

func ExecuteStandardized[REQ any, RES any](e echo.Context, request *REQ, handlerName string, delegate HandlerFunc[REQ, RES]) (any, *echo.HTTPError) {
	log := notifylog.New(handlerName, notifylog.JSON)

	internalResponse, delegateError := delegate(log, e, request)
	if delegateError != nil {
		log.Error().
			Int("status", delegateError.Code).
			Any("cause", delegateError.Internal).
			Msgf("Request failed with HTTP error: %s", delegateError.Message)

		return nil, delegateError
	}

	requestID, ok := e.Get(middleware.ContextKeyRequestID).(string)
	if !ok {
		requestID = "N/A"
	}

	finalPayload := &APIResponse[RES]{
		RequestID:  requestID,
		Data:       internalResponse.Data,
		Pagination: internalResponse.Pagination,
	}

	return finalPayload, nil
}
