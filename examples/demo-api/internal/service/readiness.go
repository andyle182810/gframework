package service

import (
	"net/http"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/labstack/echo/v4"
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

func (s *Service) CheckReadiness(ctx echo.Context, req *ReadinessCheckRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		eCtx echo.Context,
		_ *ReadinessCheckRequest,
	) (*httpserver.HandlerResponse[ReadinessCheckResponse], *echo.HTTPError) {
		log.Info().Msg("Readiness check requested")

		services := make(map[string]Info)
		allReady := true

		if err := s.db.HealthCheck(eCtx.Request().Context()); err != nil {
			services["postgres"] = Info{
				Status: "not_ready",
				Error:  err.Error(),
			}
			allReady = false

			log.Error().Err(err).Msg("Postgres health check failed")
		} else {
			services["postgres"] = Info{
				Status: "ready",
				Error:  "",
			}
		}

		if err := s.valkey.HealthCheck(eCtx.Request().Context()); err != nil {
			services["valkey"] = Info{
				Status: "not_ready",
				Error:  err.Error(),
			}
			allReady = false

			log.Error().Err(err).Msg("Valkey health check failed")
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

	return httpserver.ExecuteStandardized(ctx, req, "CheckReadiness", delegator)
}
