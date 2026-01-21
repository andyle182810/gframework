package httpserver

import (
	"net/http"
	"os"
	"time"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

func Wrapper[TREQ any](wrapped func(*echo.Context, *TREQ) (any, *echo.HTTPError)) echo.HandlerFunc {
	return func(c *echo.Context) error {
		requestURI := c.Request().RequestURI
		requestID := c.Request().Header.Get(middleware.HeaderXRequestID)

		logger := zerolog.Ctx(c.Request().Context())
		if logger.GetLevel() == zerolog.Disabled {
			newLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
			logger = &newLogger
			ctx := logger.WithContext(c.Request().Context())
			c.SetRequest(c.Request().WithContext(ctx))
		}

		logCtx := logger.With().
			Str("request_id", requestID).
			Logger()

		logRequestStart(logCtx, requestURI)

		req, err := bindAndValidate[TREQ](c, logCtx, requestURI)
		if err != nil {
			return err
		}

		c.Set(middleware.ContextKeyBody, req)

		res, err := wrapped(c, req)
		if err != nil {
			return err
		}

		response, errx := echo.UnwrapResponse(c.Response())
		if errx != nil {
			return errx
		}

		status := 0

		if response != nil {
			status = response.Status
		}

		return sendResponse(c, logCtx, status, res)
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

func bindAndValidate[TREQ any](c *echo.Context, log zerolog.Logger, path string) (*TREQ, *echo.HTTPError) {
	var req TREQ

	if err := c.Bind(&req); err != nil {
		logError(log, err, path, req, "The request body failed to bind to the expected structure")

		httpErr := echo.NewHTTPError(http.StatusBadRequest, err.Error())
		_ = httpErr.Wrap(err)

		return nil, httpErr
	}

	if err := c.Validate(&req); err != nil {
		logError(log, err, path, req, "The request validation has failed")

		httpErr := echo.NewHTTPError(http.StatusBadRequest, err.Error())
		_ = httpErr.Wrap(err)

		return nil, httpErr
	}

	return &req, nil
}

func sendResponse(c *echo.Context, log zerolog.Logger, status int, res any) error {
	if status != 0 {
		logRequestEnd(log, status)

		return c.JSON(status, res)
	}

	logRequestEnd(log, status)

	return c.JSON(http.StatusOK, res)
}
