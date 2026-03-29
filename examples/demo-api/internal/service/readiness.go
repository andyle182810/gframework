package service

import (
	"net/http"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/valkey"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type ReadinessCheckRequest struct{}

type ReadinessCheckResponse struct {
	Status   string          `json:"status"`
	Services map[string]Info `json:"services"`
}

type Info struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ReadinessCheckExecutor struct {
	log    zerolog.Logger
	db     *postgres.Postgres
	valkey *valkey.Valkey
}

func NewReadinessCheckExecutor(
	log zerolog.Logger,
	db *postgres.Postgres,
	valkey *valkey.Valkey,
) *ReadinessCheckExecutor {
	return &ReadinessCheckExecutor{
		log:    log,
		db:     db,
		valkey: valkey,
	}
}

func (e *ReadinessCheckExecutor) Execute(
	c *echo.Context,
	_ *ReadinessCheckRequest,
) (*httpserver.HandlerResponse[ReadinessCheckResponse], *echo.HTTPError) {
	ctx := c.Request().Context()

	e.log.Info().Msg("Readiness check requested")

	services := make(map[string]Info)
	allReady := true

	if err := e.db.HealthCheck(ctx); err != nil {
		services["postgres"] = Info{
			Status: "not_ready",
			Error:  err.Error(),
		}
		allReady = false

		e.log.Error().Err(err).Msg("Postgres health check failed")
	} else {
		services["postgres"] = Info{
			Status: "ready",
			Error:  "",
		}
	}

	if err := e.valkey.HealthCheck(ctx); err != nil {
		services["valkey"] = Info{
			Status: "not_ready",
			Error:  err.Error(),
		}
		allReady = false

		e.log.Error().Err(err).Msg("Valkey health check failed")
	} else {
		services["valkey"] = Info{
			Status: "ready",
			Error:  "",
		}
	}

	response := &httpserver.HandlerResponse[ReadinessCheckResponse]{
		Data: ReadinessCheckResponse{
			Status:   "ready",
			Services: services,
		},
		Pagination: nil,
	}

	if !allReady {
		response.Data.Status = "not_ready"

		return response, echo.NewHTTPError(http.StatusServiceUnavailable, "Service not ready")
	}

	return response, nil
}

// CheckReadiness godoc
//
//	@Summary		Readiness check
//	@Description	Returns the readiness status of the service and its dependencies.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	httpserver.APIResponse[ReadinessCheckResponse]	"Service is ready"
//	@Failure		503	{object}	echo.HTTPError	"Service not ready"
//	@Router			/ready [get]
func (s *Service) CheckReadiness(ctx *echo.Context, req *ReadinessCheckRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *ReadinessCheckRequest,
	) (*httpserver.HandlerResponse[ReadinessCheckResponse], *echo.HTTPError) {
		exec := NewReadinessCheckExecutor(log, s.db, s.valkey)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "CheckReadiness", delegator)
}
