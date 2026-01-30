package httpserver

import (
	"net/http"
	"os"

	"github.com/andyle182810/gframework/middleware"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

func Wrapper[TREQ any](wrapped func(*echo.Context, *TREQ) (any, *echo.HTTPError)) echo.HandlerFunc {
	return func(c *echo.Context) error {
		logger := getOrCreateLogger(c)
		requestID := middleware.GetRequestID(c)

		logCtx := logger.With().Str("request_id", requestID).Logger()
		logCtx.Info().Str("path", c.Request().RequestURI).Msg("Request started")

		req, err := bindAndValidate[TREQ](c)
		if err != nil {
			return err
		}

		c.Set(middleware.ContextKeyBody, req)

		res, err := wrapped(c, req)
		if err != nil {
			return err
		}

		status := http.StatusOK
		if response, errx := echo.UnwrapResponse(c.Response()); errx == nil && response != nil && response.Status != 0 {
			status = response.Status
		}

		return c.JSON(status, res)
	}
}

func getOrCreateLogger(c *echo.Context) *zerolog.Logger {
	logger := zerolog.Ctx(c.Request().Context())
	if logger.GetLevel() == zerolog.Disabled {
		newLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		ctx := newLogger.WithContext(c.Request().Context())
		c.SetRequest(c.Request().WithContext(ctx))

		return &newLogger
	}

	return logger
}

func bindAndValidate[TREQ any](c *echo.Context) (*TREQ, *echo.HTTPError) {
	var req TREQ

	if err := c.Bind(&req); err != nil {
		httpErr := echo.NewHTTPError(http.StatusBadRequest, "Invalid request body: "+err.Error())
		_ = httpErr.Wrap(err)

		return nil, httpErr
	}

	if err := c.Validate(&req); err != nil {
		httpErr := echo.NewHTTPError(http.StatusBadRequest, "Validation failed: "+err.Error())
		_ = httpErr.Wrap(err)

		return nil, httpErr
	}

	return &req, nil
}
