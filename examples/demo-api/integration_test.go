package main

// import (
// 	"context"
// 	"net/http"
// 	"testing"
// 	"time"

// 	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
// 	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
// 	"github.com/andyle182810/gframework/goredis"
// 	"github.com/andyle182810/gframework/httpserver"
// 	"github.com/andyle182810/gframework/postgres"
// 	"github.com/andyle182810/gframework/testutil"
// 	"github.com/google/uuid"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// // TestIntegration_FullUserWorkflow tests the complete user management workflow.
// func TestIntegration_FullUserWorkflow(t *testing.T) {
// 	testutil.SkipIfShort(t)

// 	ctx := testutil.Context(t)

// 	// Setup infrastructure
// 	pgContainer := testutil.SetupPostgresContainer(ctx, t)
// 	redisContainer := testutil.SetupRedisContainer(ctx, t)

// 	pg, err := postgres.New(ctx, &postgres.Config{
// 		ConnString: pgContainer.ConnectionString(),
// 	})
// 	require.NoError(t, err)

// 	redis, err := goredis.NewRedis(ctx, &goredis.Config{
// 		Host:     redisContainer.Host,
// 		Port:     redisContainer.Port.Port(),
// 		Password: "",
// 		DB:       0,
// 	})
// 	require.NoError(t, err)

// 	// Run migrations
// 	_, err = pg.Exec(ctx, `
// 		CREATE TABLE IF NOT EXISTS users (
// 			id UUID PRIMARY KEY,
// 			name VARCHAR(100) NOT NULL,
// 			email VARCHAR(255) NOT NULL UNIQUE,
// 			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
// 			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
// 		);
// 		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
// 		CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);
// 	`)
// 	require.NoError(t, err)

// 	// Initialize services
// 	repository := repo.New(pg)
// 	svc := service.New(repository, pg, redis)

// 	// Test 1: Create a user
// 	t.Run("Create User", func(t *testing.T) {
// 		email := testutil.RandomEmail()
// 		createReq := service.CreateUserRequest{
// 			Name:  "Integration Test User",
// 			Email: email,
// 		}

// 		echoCtx, rec, _ := testutil.SetupEchoContextWithJSON(t,
// 			http.MethodPost,
// 			"/v1/users",
// 			createReq,
// 		)

// 		handler := httpserver.Wrapper(svc.CreateUser)
// 		err := handler(echoCtx)
// 		require.NoError(t, err)

// 		testutil.AssertStatusCode(t, rec, http.StatusCreated)

// 		var resp struct {
// 			Data service.CreateUserResponse `json:"data"`
// 		}
// 		testutil.AssertJSONResponse(t, rec, &resp)

// 		assert.NotEmpty(t, resp.Data.ID)
// 		assert.Equal(t, "Integration Test User", resp.Data.Name)
// 		assert.Equal(t, email, resp.Data.Email)

// 		// Test 2: Verify cache was set
// 		t.Run("Verify Redis Cache", func(t *testing.T) {
// 			testutil.Eventually(t, func() bool {
// 				cacheKey := "user:email:" + email
// 				cachedID, err := redis.Get(ctx, cacheKey).Result()
// 				return err == nil && cachedID == resp.Data.ID
// 			}, 2*time.Second, 100*time.Millisecond)
// 		})

// 		// Test 3: Get the user by ID
// 		t.Run("Get User By ID", func(t *testing.T) {
// 			echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 				Method: http.MethodGet,
// 				Path:   "/v1/users/:userId",
// 				PathParams: map[string]string{
// 					"userId": resp.Data.ID,
// 				},
// 			})

// 			handler := httpserver.Wrapper(svc.GetUser)
// 			err := handler(echoCtx)
// 			require.NoError(t, err)

// 			testutil.AssertStatusCode(t, rec, http.StatusOK)

// 			var getResp struct {
// 				Data service.GetUserResponse `json:"data"`
// 			}
// 			testutil.AssertJSONResponse(t, rec, &getResp)

// 			assert.Equal(t, resp.Data.ID, getResp.Data.ID)
// 			assert.Equal(t, resp.Data.Name, getResp.Data.Name)
// 			assert.Equal(t, resp.Data.Email, getResp.Data.Email)
// 		})
// 	})

// 	// Test 4: List users
// 	t.Run("List Users", func(t *testing.T) {
// 		// Create a few more users
// 		for i := 0; i < 5; i++ {
// 			createReq := service.CreateUserRequest{
// 				Name:  testutil.RandomString(10),
// 				Email: testutil.RandomEmail(),
// 			}

// 			echoCtx, rec, _ := testutil.SetupEchoContextWithJSON(t,
// 				http.MethodPost,
// 				"/v1/users",
// 				createReq,
// 			)

// 			handler := httpserver.Wrapper(svc.CreateUser)
// 			_ = handler(echoCtx)
// 			testutil.AssertStatusCode(t, rec, http.StatusCreated)
// 		}

// 		// List users with pagination
// 		echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 			Method: http.MethodGet,
// 			Path:   "/v1/users",
// 			QueryParams: map[string]string{
// 				"page":     "1",
// 				"pageSize": "5",
// 			},
// 		})

// 		handler := httpserver.Wrapper(svc.ListUsers)
// 		err := handler(echoCtx)
// 		require.NoError(t, err)

// 		testutil.AssertStatusCode(t, rec, http.StatusOK)

// 		var listResp struct {
// 			Data       service.ListUsersResponse  `json:"data"`
// 			Pagination *httpserver.Pagination     `json:"pagination"`
// 		}
// 		testutil.AssertJSONResponse(t, rec, &listResp)

// 		assert.Equal(t, 5, len(listResp.Data.Users))
// 		assert.Equal(t, 1, listResp.Pagination.Page)
// 		assert.Equal(t, 5, listResp.Pagination.PageSize)
// 		assert.GreaterOrEqual(t, listResp.Pagination.TotalCount, 6) // At least 6 users
// 	})

// 	// Test 5: Health check
// 	t.Run("Health Check", func(t *testing.T) {
// 		echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 			Method: http.MethodGet,
// 			Path:   "/health",
// 		})

// 		err := svc.HealthCheck(echoCtx)
// 		require.NoError(t, err)

// 		testutil.AssertStatusCode(t, rec, http.StatusOK)

// 		var resp struct {
// 			Data struct {
// 				Status string `json:"status"`
// 			} `json:"data"`
// 		}
// 		testutil.AssertJSONResponse(t, rec, &resp)
// 		assert.Equal(t, "healthy", resp.Data.Status)
// 	})

// 	// Test 6: Readiness check
// 	t.Run("Readiness Check", func(t *testing.T) {
// 		echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 			Method: http.MethodGet,
// 			Path:   "/ready",
// 		})

// 		err := svc.ReadinessCheck(echoCtx)
// 		require.NoError(t, err)

// 		testutil.AssertStatusCode(t, rec, http.StatusOK)

// 		var resp struct {
// 			Data struct {
// 				Status   string                 `json:"status"`
// 				Services map[string]interface{} `json:"services"`
// 			} `json:"data"`
// 		}
// 		testutil.AssertJSONResponse(t, rec, &resp)
// 		assert.Equal(t, "ready", resp.Data.Status)
// 		assert.Contains(t, resp.Data.Services, "postgres")
// 		assert.Contains(t, resp.Data.Services, "redis")
// 	})
// }

// // TestIntegration_ErrorHandling tests error handling scenarios.
// func TestIntegration_ErrorHandling(t *testing.T) {
// 	testutil.SkipIfShort(t)

// 	ctx := testutil.Context(t)

// 	// Setup infrastructure
// 	pgContainer := testutil.SetupPostgresContainer(ctx, t)
// 	redisContainer := testutil.SetupRedisContainer(ctx, t)

// 	pg, err := postgres.New(ctx, &postgres.Config{
// 		ConnString: pgContainer.ConnectionString(),
// 	})
// 	require.NoError(t, err)

// 	redis, err := goredis.NewRedis(ctx, &goredis.Config{
// 		Host:     redisContainer.Host,
// 		Port:     redisContainer.Port.Port(),
// 		Password: "",
// 		DB:       0,
// 	})
// 	require.NoError(t, err)

// 	// Run migrations
// 	_, err = pg.Exec(ctx, `
// 		CREATE TABLE IF NOT EXISTS users (
// 			id UUID PRIMARY KEY,
// 			name VARCHAR(100) NOT NULL,
// 			email VARCHAR(255) NOT NULL UNIQUE,
// 			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
// 			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
// 		);
// 	`)
// 	require.NoError(t, err)

// 	repository := repo.New(pg)
// 	svc := service.New(repository, pg, redis)

// 	// Test 1: Invalid user ID format
// 	t.Run("Get User - Invalid ID", func(t *testing.T) {
// 		echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 			Method: http.MethodGet,
// 			Path:   "/v1/users/:userId",
// 			PathParams: map[string]string{
// 				"userId": "not-a-uuid",
// 			},
// 		})

// 		handler := httpserver.Wrapper(svc.GetUser)
// 		_ = handler(echoCtx)

// 		testutil.AssertStatusCode(t, rec, http.StatusBadRequest)
// 		assert.Contains(t, rec.Body.String(), "userId")
// 	})

// 	// Test 2: User not found
// 	t.Run("Get User - Not Found", func(t *testing.T) {
// 		echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 			Method: http.MethodGet,
// 			Path:   "/v1/users/:userId",
// 			PathParams: map[string]string{
// 				"userId": uuid.NewString(),
// 			},
// 		})

// 		handler := httpserver.Wrapper(svc.GetUser)
// 		_ = handler(echoCtx)

// 		testutil.AssertStatusCode(t, rec, http.StatusNotFound)
// 		assert.Contains(t, rec.Body.String(), "not found")
// 	})

// 	// Test 3: Duplicate email
// 	t.Run("Create User - Duplicate Email", func(t *testing.T) {
// 		email := testutil.RandomEmail()

// 		// Create first user
// 		createReq1 := service.CreateUserRequest{
// 			Name:  "First User",
// 			Email: email,
// 		}

// 		echoCtx1, rec1, _ := testutil.SetupEchoContextWithJSON(t,
// 			http.MethodPost,
// 			"/v1/users",
// 			createReq1,
// 		)

// 		handler := httpserver.Wrapper(svc.CreateUser)
// 		_ = handler(echoCtx1)
// 		testutil.AssertStatusCode(t, rec1, http.StatusCreated)

// 		// Try to create second user with same email
// 		createReq2 := service.CreateUserRequest{
// 			Name:  "Second User",
// 			Email: email,
// 		}

// 		echoCtx2, rec2, _ := testutil.SetupEchoContextWithJSON(t,
// 			http.MethodPost,
// 			"/v1/users",
// 			createReq2,
// 		)

// 		_ = handler(echoCtx2)
// 		assert.NotEqual(t, http.StatusCreated, rec2.Code)
// 	})

// 	// Test 4: Invalid pagination parameters
// 	t.Run("List Users - Invalid Pagination", func(t *testing.T) {
// 		tests := []struct {
// 			name        string
// 			queryParams map[string]string
// 		}{
// 			{
// 				name: "invalid page (0)",
// 				queryParams: map[string]string{
// 					"page": "0",
// 				},
// 			},
// 			{
// 				name: "invalid page size (too large)",
// 				queryParams: map[string]string{
// 					"pageSize": "101",
// 				},
// 			},
// 			{
// 				name: "invalid page (negative)",
// 				queryParams: map[string]string{
// 					"page": "-1",
// 				},
// 			},
// 		}

// 		for _, tt := range tests {
// 			t.Run(tt.name, func(t *testing.T) {
// 				echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{
// 					Method:      http.MethodGet,
// 					Path:        "/v1/users",
// 					QueryParams: tt.queryParams,
// 				})

// 				handler := httpserver.Wrapper(svc.ListUsers)
// 				_ = handler(echoCtx)

// 				testutil.AssertStatusCode(t, rec, http.StatusBadRequest)
// 			})
// 		}
// 	})
// }
