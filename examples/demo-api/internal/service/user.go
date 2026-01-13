package service

import (
	"net/http"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/pagination"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/labstack/echo/v4"
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

type GetUserRequest struct {
	UserID string `param:"userId" validate:"required,uuid"`
}

type GetUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type ListUsersRequest struct {
	Page     int `query:"page"`
	PageSize int `query:"pageSize"`
}

type ListUsersResponse struct {
	Users []GetUserResponse `json:"users"`
}

func (s *Service) CreateUser(ctx echo.Context, req *CreateUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx echo.Context,
		req *CreateUserRequest,
	) (*httpserver.HandlerResponse[CreateUserResponse], *echo.HTTPError) {
		log.Info().
			Str("name", req.Name).
			Str("email", req.Email).
			Msg("Creating new user")

		user, err := s.repo.User.CreateUser(ctx.Request().Context(), req.Name, req.Email)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create user")

			return nil, httpserver.HTTPError(http.StatusInternalServerError, err, "Failed to create user")
		}

		cacheKey := "user:email:" + req.Email

		const cacheExpiration = 5 * time.Minute
		if err := s.redis.Set(ctx.Request().Context(), cacheKey, user.ID, cacheExpiration).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache user email")
		}

		log.Info().Str("user_id", user.ID).Msg("User created successfully")

		return &httpserver.HandlerResponse[CreateUserResponse]{
			Data: CreateUserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
			},
			Pagination: nil,
		}, nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "CreateUser", delegator)
}

func (s *Service) GetUser(ctx echo.Context, req *GetUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx echo.Context,
		req *GetUserRequest,
	) (*httpserver.HandlerResponse[GetUserResponse], *echo.HTTPError) {
		log.Info().Str("user_id", req.UserID).Msg("Fetching user")

		user, err := s.repo.User.GetUserByID(ctx.Request().Context(), req.UserID)
		if err != nil {
			if pgxscan.NotFound(err) {
				log.Error().Err(err).Str("user_id", req.UserID).Msg("User not found")

				return nil, httpserver.HTTPError(http.StatusNotFound, err, "User not found")
			}

			log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to fetch user")

			return nil, httpserver.HTTPError(http.StatusInternalServerError, err, "Failed to fetch user")
		}

		log.Info().Str("user_id", req.UserID).Msg("User fetched successfully")

		return &httpserver.HandlerResponse[GetUserResponse]{
			Data: GetUserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
			},
			Pagination: nil,
		}, nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "GetUser", delegator)
}

func (s *Service) ListUsers(ctx echo.Context, req *ListUsersRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx echo.Context,
		req *ListUsersRequest,
	) (*httpserver.HandlerResponse[ListUsersResponse], *echo.HTTPError) {
		page, pageSize, offset := pagination.Normalize(req.Page, req.PageSize)

		log.Info().
			Int("page", page).
			Int("page_size", pageSize).
			Msg("Listing users")

		totalCount, err := s.repo.User.CountUsers(ctx.Request().Context())
		if err != nil {
			log.Error().Err(err).Msg("Failed to count users")

			return nil, httpserver.HTTPError(http.StatusInternalServerError, err, "Failed to count users")
		}

		users, err := s.repo.User.ListUsers(ctx.Request().Context(), pageSize, offset)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list users")

			return nil, httpserver.HTTPError(http.StatusInternalServerError, err, "Failed to list users")
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

		totalPages := pagination.ComputeTotals(totalCount, pageSize)

		log.Info().
			Int("total_count", totalCount).
			Int("returned", len(users)).
			Msg("Users listed successfully")

		return &httpserver.HandlerResponse[ListUsersResponse]{
			Data: ListUsersResponse{
				Users: userResponses,
			},
			Pagination: &httpserver.Pagination{
				Page:       page,
				PageSize:   pageSize,
				TotalCount: totalCount,
				TotalPages: totalPages,
			},
		}, nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "ListUsers", delegator)
}
