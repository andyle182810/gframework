package service

import (
	"net/http"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/georgysavva/scany/v2/pgxscan"
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

func (s *Service) CreateUser(ctx *echo.Context, req *CreateUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
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
		if err := s.valkey.Set(ctx.Request().Context(), cacheKey, user.ID, cacheExpiration).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache user email")
		}

		log.Info().Str("user_id", user.ID).Msg("User created successfully")

		return httpserver.NewResponse(
			CreateUserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
			},
		), nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "CreateUser", delegator)
}

func (s *Service) GetUser(ctx *echo.Context, req *GetUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *GetUserRequest,
	) (*httpserver.HandlerResponse[GetUserResponse], *echo.HTTPError) {
		log.Info().Str("user_id", req.UserID).Msg("Fetching user")

		user, err := s.repo.User.GetUserByID(ctx.Request().Context(), req.UserID)
		if err != nil {
			if pgxscan.NotFound(err) {
				log.Error().Err(err).Str("user_id", req.UserID).Msg("User not found")

				return nil, httpserver.NotFoundError(err, "User not found")
			}

			log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to fetch user")

			return nil, httpserver.InternalError(err, "Failed to fetch user")
		}

		log.Info().Str("user_id", req.UserID).Msg("User fetched successfully")

		return httpserver.NewResponse(
			GetUserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
			},
		), nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "GetUser", delegator)
}

func (s *Service) UpdateUser(ctx *echo.Context, req *UpdateUserRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *UpdateUserRequest,
	) (*httpserver.HandlerResponse[UpdateUserResponse], *echo.HTTPError) {
		log.Info().
			Str("user_id", req.UserID).
			Str("name", req.Name).
			Str("email", req.Email).
			Msg("Updating user")

		user, err := s.repo.User.UpdateUser(ctx.Request().Context(), req.UserID, req.Name, req.Email)
		if err != nil {
			if pgxscan.NotFound(err) {
				log.Error().Err(err).Str("user_id", req.UserID).Msg("User not found")

				return nil, httpserver.NotFoundError(err, "User not found")
			}

			log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to update user")

			return nil, httpserver.InternalError(err, "Failed to update user")
		}

		// Invalidate cache for the old email and update with new email
		cacheKey := "user:email:" + req.Email

		const cacheExpiration = 5 * time.Minute
		if err := s.valkey.Set(ctx.Request().Context(), cacheKey, user.ID, cacheExpiration).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache user email")
		}

		log.Info().Str("user_id", req.UserID).Msg("User updated successfully")

		return httpserver.NewResponse(
			UpdateUserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
			},
		), nil
	}

	return httpserver.ExecuteStandardized(ctx, req, "UpdateUser", delegator)
}

func (s *Service) ListUsers(ctx *echo.Context, req *ListUsersRequest) (any, *echo.HTTPError) {
	delegator := func(
		log zerolog.Logger,
		ctx *echo.Context,
		req *ListUsersRequest,
	) (*httpserver.HandlerResponse[ListUsersResponse], *echo.HTTPError) {
		page, pageSize, offset := httpserver.NormalizePage(req.Page, req.PageSize)

		log.Info().
			Int("page", page).
			Int("page_size", pageSize).
			Msg("Listing users")

		totalCount, err := s.repo.User.CountUsers(ctx.Request().Context())
		if err != nil {
			log.Error().Err(err).Msg("Failed to count users")

			return nil, httpserver.InternalError(err, "Failed to count users")
		}

		users, err := s.repo.User.ListUsers(ctx.Request().Context(), pageSize, offset)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list users")

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

		log.Info().
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

	return httpserver.ExecuteStandardized(ctx, req, "ListUsers", delegator)
}
