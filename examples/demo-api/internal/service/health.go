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

func (s *Service) CheckHealth(ctx *echo.Context, req *HealthCheckRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		_ *echo.Context,
		_ *HealthCheckRequest,
	) (*httpserver.HandlerResponse[HealthCheckResponse], *echo.HTTPError) {
		log.Info().Msg("Health check requested")

		return &httpserver.HandlerResponse[HealthCheckResponse]{
			Data: HealthCheckResponse{
				Status: "healthy",
			},
			Pagination: nil,
		}, nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "CheckHealth", delegator)
}
