package httpserver

import (
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/notifylog"
	"github.com/labstack/echo/v4"
)

func Wrapper[TREQ any](wrapped func(echo.Context, *TREQ) (any, *echo.HTTPError)) echo.HandlerFunc {
	return func(ectx echo.Context) error {
		requestURI := ectx.Request().RequestURI
		requestID := ectx.Request().Header.Get(middleware.HeaderXRequestID)
		handlerName := runtime.FuncForPC(reflect.ValueOf(wrapped).Pointer()).Name()
		log := notifylog.New("wrapper", notifylog.JSON).With(map[string]string{
			"request_id": requestID,
		})

		// Log the start of the request
		logRequestStart(&log, requestURI, handlerName)

		// Bind and validate the request
		req, err := bindAndValidate[TREQ](ectx, &log, requestURI)
		if err != nil {
			return err
		}

		// Set the request object in context
		ectx.Set(middleware.ContextKeyBody, req)

		// Call the wrapped handler
		res, err := wrapped(ectx, req)
		if err != nil {
			return err
		}

		// Send the response
		return sendResponse(ectx, &log, ectx.Response().Status, res)
	}
}

func logRequestStart(log *notifylog.NotifyLog, path, handler string) {
	log.Info().
		Str("path", path).
		Str("handler", handler).
		Msg("request started - processing incoming request")
}

func logRequestEnd(log *notifylog.NotifyLog, status int) {
	log.Info().
		Time("at", time.Now()).
		Int("status", status).
		Msg("request completed - response sent to client")
}

func logError(log *notifylog.NotifyLog, err error, path string, req any, msg string) {
	log.Error().
		Err(err).
		Any("path", path).
		Any("request_object", req).
		Msg(msg)
}

func bindAndValidate[TREQ any](ectx echo.Context, log *notifylog.NotifyLog, path string) (*TREQ, *echo.HTTPError) {
	var req TREQ

	// Bind the request
	if err := ectx.Bind(&req); err != nil {
		logError(log, err, path, req, "failed to bind request body to the expected structure")

		return nil, &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  err.Error(),
			Internal: err,
		}
	}

	// Validate the request
	if err := ectx.Validate(&req); err != nil {
		logError(log, err, path, req, "request validation failed")

		return nil, &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  err.Error(),
			Internal: err,
		}
	}

	return &req, nil
}

func sendResponse(ectx echo.Context, log *notifylog.NotifyLog, status int, res any) error {
	if status != 0 {
		logRequestEnd(log, status)

		return ectx.JSON(status, res)
	}

	logRequestEnd(log, status)

	return ectx.JSON(http.StatusOK, res)
}
