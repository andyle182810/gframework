package httpserver

import (
	"net/http"
	"os"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func Wrapper[TREQ any](wrapped func(echo.Context, *TREQ) (any, *echo.HTTPError)) echo.HandlerFunc {
	return func(ectx echo.Context) error {
		requestURI := ectx.Request().RequestURI
		requestID := ectx.Request().Header.Get(middleware.HeaderXRequestID)

		logger := zerolog.Ctx(ectx.Request().Context())
		if logger.GetLevel() == zerolog.Disabled {
			newLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
			logger = &newLogger
			ctx := logger.WithContext(ectx.Request().Context())
			ectx.SetRequest(ectx.Request().WithContext(ctx))
		}

		logCtx := logger.With().
			Str("request_id", requestID).
			Logger()

		logRequestStart(logCtx, requestURI)

		req, err := bindAndValidate[TREQ](ectx, logCtx, requestURI)
		if err != nil {
			return err
		}

		ectx.Set(middleware.ContextKeyBody, req)

		res, err := wrapped(ectx, req)
		if err != nil {
			return err
		}

		return sendResponse(ectx, logCtx, ectx.Response().Status, res)
	}
}

func logRequestStart(log zerolog.Logger, path string) {
	log.Info().
		Str("path", path).
		Msg("The request has been started and is being processed")
}

func logRequestEnd(log zerolog.Logger, status int) {
	log.Info().
		Time("completed_at", time.Now()).
		Int("status_code", status).
		Msg("The request has been completed and the response has been sent to the client")
}

func logError(log zerolog.Logger, err error, path string, req any, msg string) {
	log.Error().
		Err(err).
		Str("path", path).
		Interface("request_object", req).
		Msg(msg)
}

func bindAndValidate[TREQ any](ectx echo.Context, log zerolog.Logger, path string) (*TREQ, *echo.HTTPError) {
	var req TREQ

	if err := ectx.Bind(&req); err != nil {
		logError(log, err, path, req, "The request body failed to bind to the expected structure")

		return nil, &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  err.Error(),
			Internal: err,
		}
	}

	if err := ectx.Validate(&req); err != nil {
		logError(log, err, path, req, "The request validation has failed")

		return nil, &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  err.Error(),
			Internal: err,
		}
	}

	return &req, nil
}

func sendResponse(ectx echo.Context, log zerolog.Logger, status int, res any) error {
	if status != 0 {
		logRequestEnd(log, status)

		return ectx.JSON(status, res)
	}

	logRequestEnd(log, status)

	return ectx.JSON(http.StatusOK, res)
}
