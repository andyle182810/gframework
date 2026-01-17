package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/pagination"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T, ctx context.Context) (*service.Service, *repo.Repository, *valkey.Valkey) {
	t.Helper()

	pgContainer := testutil.SetupPostgresContainer(ctx, t)
	redisContainer := testutil.SetupRedisContainer(ctx, t)

	pg, err := postgres.New(&postgres.Config{
		URL:                      pgContainer.ConnectionString(),
		MaxConnection:            10,
		MinConnection:            2,
		MaxConnectionIdleTime:    5 * time.Minute,
		MaxConnectionLifetime:    30 * time.Minute,
		HealthCheckPeriod:        1 * time.Minute,
		ConnectTimeout:           5 * time.Second,
		LogLevel:                 tracelog.LogLevelInfo,
		StatementTimeout:         30 * time.Second,
		LockTimeout:              10 * time.Second,
		IdleInTransactionTimeout: 30 * time.Second,
	})
	require.NoError(t, err)

	err = testutil.RunMigrations(ctx, pg, "../../db/migrations")
	require.NoError(t, err)

	testutil.CleanupDatabase(t, ctx, pg)

	valkey, err := valkey.New(&valkey.Config{
		Host:     redisContainer.Host,
		Port:     redisContainer.Port.Int(),
		Password: "",
		DB:       0,
	})
	require.NoError(t, err)

	repository := repo.New(pg)
	service := service.New(repository, pg, valkey)

	return service, repository, valkey
}

func TestService_CreateUser(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, _, _ := setupTestService(t, ctx)

	tests := []struct {
		name           string
		requestBody    service.CreateUserRequest
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "successful user creation",
			requestBody: service.CreateUserRequest{
				Name:  "John Doe",
				Email: testutil.RandomEmail(),
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data service.CreateUserResponse `json:"data"`
				}
				testutil.AssertJSONResponse(t, rec, &resp)
				require.NotEmpty(t, resp.Data.ID)
				require.Equal(t, "John Doe", resp.Data.Name)
				require.NotEmpty(t, resp.Data.Email)
				require.NotZero(t, resp.Data.CreatedAt)
			},
		},
		{
			name: "validation error - name too short",
			requestBody: service.CreateUserRequest{
				Name:  "J",
				Email: testutil.RandomEmail(),
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "Error:Field validation for 'Name'")
			},
		},
		{
			name: "validation error - invalid email",
			requestBody: service.CreateUserRequest{
				Name:  "John Doe",
				Email: "invalid-email",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "Error:Field validation for 'Email'")
			},
		},
		{
			name: "validation error - missing name",
			requestBody: service.CreateUserRequest{
				Email: testutil.RandomEmail(),
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "Error:Field validation for 'Name'")
			},
		},
		{
			name: "validation error - missing email",
			requestBody: service.CreateUserRequest{
				Name: "John Doe",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "Error:Field validation for 'Email'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			echoCtx, rec, _ := testutil.SetupEchoContextWithJSON(t,
				http.MethodPost,
				"/v1/users",
				tt.requestBody,
			)

			handler := httpserver.Wrapper(svc.CreateUser)

			err := handler(echoCtx)
			if err != nil {
				echoCtx.Echo().HTTPErrorHandler(err, echoCtx)
			}

			testutil.AssertStatusCode(t, rec, tt.expectedStatus)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestService_CreateUser_DuplicateEmail(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, _, _ := setupTestService(t, ctx)

	email := testutil.RandomEmail()

	req1 := service.CreateUserRequest{
		Name:  "First User",
		Email: email,
	}

	echoCtx1, rec1, _ := testutil.SetupEchoContextWithJSON(t,
		http.MethodPost,
		"/v1/users",
		req1,
	)

	handler := httpserver.Wrapper(svc.CreateUser)
	err := handler(echoCtx1)
	if err != nil {
		echoCtx1.Echo().HTTPErrorHandler(err, echoCtx1)
	}
	testutil.AssertStatusCode(t, rec1, http.StatusOK)

	req2 := service.CreateUserRequest{
		Name:  "Second User",
		Email: email,
	}

	echoCtx2, rec2, _ := testutil.SetupEchoContextWithJSON(t,
		http.MethodPost,
		"/v1/users",
		req2,
	)

	err = handler(echoCtx2)
	if err != nil {
		echoCtx2.Echo().HTTPErrorHandler(err, echoCtx2)
	}

	require.NotEqual(t, http.StatusOK, rec2.Code)
}

func TestService_GetUser(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, repository, _ := setupTestService(t, ctx)

	email := testutil.RandomEmail()
	user, err := repository.User.CreateUser(ctx, "Test User", email)
	require.NoError(t, err)

	userID, err := uuid.Parse(user.ID)
	require.NoError(t, err)

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "successful get user",
			userID:         userID.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Data service.GetUserResponse `json:"data"`
				}
				testutil.AssertJSONResponse(t, rec, &resp)
				require.Equal(t, userID.String(), resp.Data.ID)
				require.Equal(t, "Test User", resp.Data.Name)
				require.Equal(t, email, resp.Data.Email)
				require.NotZero(t, resp.Data.CreatedAt)
			},
		},
		{
			name:           "user not found",
			userID:         uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "not found")
			},
		},
		{
			name:           "invalid user ID format",
			userID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "Error:Field validation for 'UserID'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
				Method: http.MethodGet,
				Path:   "/v1/users/:userId",
				PathParams: map[string]string{
					"userId": tt.userID,
				},
			})

			handler := httpserver.Wrapper(svc.GetUser)

			err := handler(echoCtx)
			if err != nil {
				echoCtx.Echo().HTTPErrorHandler(err, echoCtx)
			}

			testutil.AssertStatusCode(t, rec, tt.expectedStatus)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestService_ListUsers(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, repository, _ := setupTestService(t, ctx)

	for i := range 15 {
		_, err := repository.User.CreateUser(ctx, fmt.Sprintf("User %d", i), testutil.RandomEmail())
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	type listUsersResponse struct {
		Data       service.ListUsersResponse `json:"data"`
		Pagination *httpserver.Pagination    `json:"pagination"`
	}

	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "list users - default pagination",
			queryParams:    map[string]string{},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp listUsersResponse
				testutil.AssertJSONResponse(t, rec, &resp)
				require.LessOrEqual(t, len(resp.Data.Users), pagination.DefaultPageSize)
				require.NotNil(t, resp.Pagination)
				require.Equal(t, 1, resp.Pagination.Page)
				require.Equal(t, 15, resp.Pagination.TotalCount)
			},
		},
		{
			name: "list users - custom page size",
			queryParams: map[string]string{
				"page":     "1",
				"pageSize": "5",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp listUsersResponse
				testutil.AssertJSONResponse(t, rec, &resp)
				require.Equal(t, 5, len(resp.Data.Users))
				require.Equal(t, 1, resp.Pagination.Page)
				require.Equal(t, 5, resp.Pagination.PageSize)
				require.Equal(t, 15, resp.Pagination.TotalCount)
				require.Equal(t, 3, resp.Pagination.TotalPages)
			},
		},
		{
			name: "list users - page 2",
			queryParams: map[string]string{
				"page":     "2",
				"pageSize": "5",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp listUsersResponse
				testutil.AssertJSONResponse(t, rec, &resp)
				require.Equal(t, 5, len(resp.Data.Users))
				require.Equal(t, 2, resp.Pagination.Page)
			},
		},
		{
			name: "list users - invalid page normalized to default",
			queryParams: map[string]string{
				"page": "0",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp listUsersResponse
				testutil.AssertJSONResponse(t, rec, &resp)
				require.Equal(t, pagination.DefaultPage, resp.Pagination.Page)
			},
		},
		{
			name: "list users - page size exceeding max normalized to max",
			queryParams: map[string]string{
				"pageSize": "200",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp listUsersResponse
				testutil.AssertJSONResponse(t, rec, &resp)
				require.Equal(t, pagination.MaxPageSize, resp.Pagination.PageSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
				Method:      http.MethodGet,
				Path:        "/v1/users",
				QueryParams: tt.queryParams,
			})

			handler := httpserver.Wrapper(svc.ListUsers)

			err := handler(echoCtx)
			if err != nil {
				echoCtx.Echo().HTTPErrorHandler(err, echoCtx)
			}

			testutil.AssertStatusCode(t, rec, tt.expectedStatus)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestService_GetUser_WithCache(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, _, redis := setupTestService(t, ctx)

	createReq := service.CreateUserRequest{
		Name:  "Cached User",
		Email: testutil.RandomEmail(),
	}

	echoCtxCreate, recCreate, _ := testutil.SetupEchoContextWithJSON(t,
		http.MethodPost,
		"/v1/users",
		createReq,
	)

	createHandler := httpserver.Wrapper(svc.CreateUser)
	err := createHandler(echoCtxCreate)
	if err != nil {
		echoCtxCreate.Echo().HTTPErrorHandler(err, echoCtxCreate)
	}
	testutil.AssertStatusCode(t, recCreate, http.StatusOK)

	testutil.Eventually(t, func() bool {
		cacheKey := "user:email:" + createReq.Email
		cachedID, err := redis.Get(ctx, cacheKey).Result()
		return err == nil && cachedID != ""
	}, 2*time.Second, 100*time.Millisecond)
}
