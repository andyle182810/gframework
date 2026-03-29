package service

import (
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type UpdateUserRequest struct {
	UserID string `param:"userId" validate:"required,uuid"`
	Name   string `json:"name"    validate:"required,min=2,max=100"`
	Email  string `json:"email"   validate:"required,email"`
}

type UpdateUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UpdateUserExecutor struct {
	log    zerolog.Logger
	repo   *repo.Repository
	valkey valkeyClient
}

func NewUpdateUserExecutor(
	log zerolog.Logger,
	repo *repo.Repository,
	valkey valkeyClient,
) *UpdateUserExecutor {
	return &UpdateUserExecutor{
		log:    log,
		repo:   repo,
		valkey: valkey,
	}
}

func (e *UpdateUserExecutor) Execute(
	c *echo.Context,
	req *UpdateUserRequest,
) (*httpserver.HandlerResponse[UpdateUserResponse], *echo.HTTPError) {
	ctx := c.Request().Context()

	e.log.Info().
		Str("user_id", req.UserID).
		Str("name", req.Name).
		Str("email", req.Email).
		Msg("Updating user")

	user, err := e.repo.User.UpdateUser(ctx, req.UserID, req.Name, req.Email)
	if err != nil {
		if pgxscan.NotFound(err) {
			e.log.Error().Err(err).Str("user_id", req.UserID).Msg("User not found")

			return nil, httpserver.NotFoundError(err, "User not found")
		}

		e.log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to update user")

		return nil, httpserver.InternalError(err, "Failed to update user")
	}

	cacheKey := "user:email:" + req.Email

	const cacheExpiration = 5 * time.Minute
	if err := e.valkey.Set(ctx, cacheKey, user.ID, cacheExpiration).Err(); err != nil {
		e.log.Warn().Err(err).Msg("Failed to cache user email")
	}

	e.log.Info().Str("user_id", req.UserID).Msg("User updated successfully")

	return httpserver.NewResponse(UpdateUserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}), nil
}

// UpdateUser godoc
//
//	@Summary		Update a user
//	@Description	Updates an existing user's name and email.
//	@Description
//	@Description	Mandatory fields:
//	@Description	  - `name` (string): User name (2-100 characters)
//	@Description	  - `email` (string): Valid email address
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			userId	path		string				true	"User ID (UUID)"
//	@Param			request	body		UpdateUserRequest	true	"User update payload"
//	@Success		200		{object}	httpserver.APIResponse[UpdateUserResponse]	"User updated successfully"
//	@Failure		400		{object}	echo.HTTPError	"Invalid input"
//	@Failure		404		{object}	echo.HTTPError	"User not found"
//	@Failure		500		{object}	echo.HTTPError	"Internal server error"
//	@Router			/v1/users/{userId} [patch]
func (s *Service) UpdateUser(ctx *echo.Context, req *UpdateUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *UpdateUserRequest,
	) (*httpserver.HandlerResponse[UpdateUserResponse], *echo.HTTPError) {
		exec := NewUpdateUserExecutor(log, s.repo, s.valkey)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "UpdateUser", delegator)
}
