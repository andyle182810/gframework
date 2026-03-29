package service

import (
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type GetUserRequest struct {
	UserID string `param:"userId" validate:"required,uuid"`
}

type GetUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type GetUserExecutor struct {
	log  zerolog.Logger
	repo *repo.Repository
}

func NewGetUserExecutor(
	log zerolog.Logger,
	repo *repo.Repository,
) *GetUserExecutor {
	return &GetUserExecutor{
		log:  log,
		repo: repo,
	}
}

func (e *GetUserExecutor) Execute(
	c *echo.Context,
	req *GetUserRequest,
) (*httpserver.HandlerResponse[GetUserResponse], *echo.HTTPError) {
	ctx := c.Request().Context()

	e.log.Info().Str("user_id", req.UserID).Msg("Fetching user")

	user, err := e.repo.User.GetUserByID(ctx, req.UserID)
	if err != nil {
		if pgxscan.NotFound(err) {
			e.log.Error().Err(err).Str("user_id", req.UserID).Msg("User not found")

			return nil, httpserver.NotFoundError(err, "User not found")
		}

		e.log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to fetch user")

		return nil, httpserver.InternalError(err, "Failed to fetch user")
	}

	e.log.Info().Str("user_id", req.UserID).Msg("User fetched successfully")

	return httpserver.NewResponse(GetUserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}), nil
}

// GetUser godoc
//
//	@Summary		Get a user by ID
//	@Description	Retrieves a user by their unique identifier.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			userId	path		string	true	"User ID (UUID)"
//	@Success		200		{object}	httpserver.APIResponse[GetUserResponse]	"User retrieved successfully"
//	@Failure		400		{object}	echo.HTTPError	"Invalid input"
//	@Failure		404		{object}	echo.HTTPError	"User not found"
//	@Failure		500		{object}	echo.HTTPError	"Internal server error"
//	@Router			/v1/users/{userId} [get]
func (s *Service) GetUser(ctx *echo.Context, req *GetUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *GetUserRequest,
	) (*httpserver.HandlerResponse[GetUserResponse], *echo.HTTPError) {
		exec := NewGetUserExecutor(log, s.repo)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "GetUser", delegator)
}
