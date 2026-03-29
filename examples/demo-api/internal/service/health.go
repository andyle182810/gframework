package service

import (
	"github.com/andyle182810/gframework/httpserver"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type HealthCheckRequest struct{}

type HealthCheckResponse struct {
	Status string `example:"healthy" json:"status"`
}

type HealthCheckExecutor struct {
	log zerolog.Logger
}

func NewHealthCheckExecutor(log zerolog.Logger) *HealthCheckExecutor {
	return &HealthCheckExecutor{log: log}
}

func (e *HealthCheckExecutor) Execute(
	_ *echo.Context,
	_ *HealthCheckRequest,
) (*httpserver.HandlerResponse[HealthCheckResponse], *echo.HTTPError) {
	e.log.Info().Msg("Health check requested")

	return httpserver.NewResponse(HealthCheckResponse{
		Status: "healthy",
	}), nil
}

// CheckHealth godoc
//
//	@Summary		Health check
//	@Description	Returns the health status of the service.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	httpserver.APIResponse[HealthCheckResponse]	"Service is healthy"
//	@Router			/health [get]
func (s *Service) CheckHealth(ctx *echo.Context, req *HealthCheckRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *HealthCheckRequest,
	) (*httpserver.HandlerResponse[HealthCheckResponse], *echo.HTTPError) {
		exec := NewHealthCheckExecutor(log)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "CheckHealth", delegator)
}
