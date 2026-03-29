package service

import (
	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

type ListUsersRequest struct {
	Page     int `query:"page"`
	PageSize int `query:"pageSize"`
}

type ListUsersResponse struct {
	Users []GetUserResponse `json:"users"`
}

type ListUsersExecutor struct {
	log  zerolog.Logger
	repo *repo.Repository
}

func NewListUsersExecutor(
	log zerolog.Logger,
	repo *repo.Repository,
) *ListUsersExecutor {
	return &ListUsersExecutor{
		log:  log,
		repo: repo,
	}
}

func (e *ListUsersExecutor) Execute(
	c *echo.Context,
	req *ListUsersRequest,
) (*httpserver.HandlerResponse[ListUsersResponse], *echo.HTTPError) {
	ctx := c.Request().Context()
	page, pageSize, offset := httpserver.NormalizePage(req.Page, req.PageSize)

	e.log.Info().
		Int("page", page).
		Int("page_size", pageSize).
		Msg("Listing users")

	totalCount, err := e.repo.User.CountUsers(ctx)
	if err != nil {
		e.log.Error().Err(err).Msg("Failed to count users")

		return nil, httpserver.InternalError(err, "Failed to count users")
	}

	users, err := e.repo.User.ListUsers(ctx, pageSize, offset)
	if err != nil {
		e.log.Error().Err(err).Msg("Failed to list users")

		return nil, httpserver.InternalError(err, "Failed to list users")
	}

	userResponses := make([]GetUserResponse, 0, len(users))
	for _, user := range users {
		userResponses = append(userResponses, GetUserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		})
	}

	e.log.Info().
		Int("total_count", totalCount).
		Int("returned", len(users)).
		Msg("Users listed successfully")

	return httpserver.NewPaginatedResponse(
		ListUsersResponse{
			Users: userResponses,
		},
		httpserver.NewPagination(page, pageSize, totalCount),
	), nil
}

// ListUsers godoc
//
//	@Summary		List users
//	@Description	Returns a paginated list of users.
//	@Description
//	@Description	Optional query parameters:
//	@Description	  - `page` (int): Page number (default: 1)
//	@Description	  - `pageSize` (int): Number of items per page (default: 20, max: 100)
//	@Tags			users
//	@Produce		json
//	@Param			page		query		int	false	"Page number"
//	@Param			pageSize	query		int	false	"Page size"
//	@Success		200			{object}	httpserver.APIResponse[ListUsersResponse]	"Users listed successfully"
//	@Failure		500			{object}	echo.HTTPError	"Internal server error"
//	@Router			/v1/users [get]
func (s *Service) ListUsers(ctx *echo.Context, req *ListUsersRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *ListUsersRequest,
	) (*httpserver.HandlerResponse[ListUsersResponse], *echo.HTTPError) {
		exec := NewListUsersExecutor(log, s.repo)

		return exec.Execute(ctx, req)
	}

	return httpserver.ExecuteStandardized(ctx, req, "ListUsers", delegator)
}
