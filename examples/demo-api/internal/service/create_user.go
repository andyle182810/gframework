package service

import (
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type CreateUserRequest struct {
	Name  string `json:"name"  validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateUserExecutor struct {
	log    zerolog.Logger
	repo   *repo.Repository
	valkey valkeyClient
}

func NewCreateUserExecutor(
	log zerolog.Logger,
	repo *repo.Repository,
	valkey valkeyClient,
) *CreateUserExecutor {
	return &CreateUserExecutor{
		log:    log,
		repo:   repo,
		valkey: valkey,
	}
}

func (e *CreateUserExecutor) Execute(
	c *echo.Context,
	req *CreateUserRequest,
) (*httpserver.HandlerResponse[CreateUserResponse], *echo.HTTPError) {
	ctx := c.Request().Context()

	e.log.Info().
		Str("name", req.Name).
		Str("email", req.Email).
		Msg("Creating new user")

	user, err := e.repo.User.CreateUser(ctx, req.Name, req.Email)
	if err != nil {
		e.log.Error().Err(err).Msg("Failed to create user")

		return nil, httpserver.InternalError(err, "Failed to create user")
	}

	cacheKey := "user:email:" + req.Email

	const cacheExpiration = 5 * time.Minute
	if err := e.valkey.Set(ctx, cacheKey, user.ID, cacheExpiration).Err(); err != nil {
		e.log.Warn().Err(err).Msg("Failed to cache user email")
	}

	e.log.Info().Str("user_id", user.ID).Msg("User created successfully")

	return httpserver.NewResponse(CreateUserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}), nil
}

// CreateUser godoc
//
//	@Summary		Create a new user
//	@Description	Creates a new user in the system.
//	@Description
//	@Description	Mandatory fields:
//	@Description	  - `name` (string): User name (2-100 characters)
//	@Description	  - `email` (string): Valid email address
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateUserRequest	true	"User creation payload"
//	@Success		200		{object}	httpserver.APIResponse[CreateUserResponse]	"User created successfully"
//	@Failure		400		{object}	echo.HTTPError	"Invalid input"
//	@Failure		500		{object}	echo.HTTPError	"Internal server error"
//	@Router			/v1/users [post]
func (s *Service) CreateUser(ctx *echo.Context, req *CreateUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *CreateUserRequest,
	) (*httpserver.HandlerResponse[CreateUserResponse], *echo.HTTPError) {
		exec := NewCreateUserExecutor(log, s.repo, s.valkey)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "CreateUser", delegator)
}
