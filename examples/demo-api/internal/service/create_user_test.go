//nolint:paralleltest,thelper
package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

func TestService_CreateUser(t *testing.T) { //nolint:funlen
	testutil.SkipIfShort(t)

	svc, _, _ := setupTestService(t)

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
				require.Contains(t, rec.Body.String(), "name must be at least 2")
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
				require.Contains(t, rec.Body.String(), "email must be a valid email address")
			},
		},
		{
			name: "validation error - missing name",
			requestBody: service.CreateUserRequest{
				Name:  "",
				Email: testutil.RandomEmail(),
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "name is required")
			},
		},
		{
			name: "validation error - missing email",
			requestBody: service.CreateUserRequest{
				Name:  "John Doe",
				Email: "",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Contains(t, rec.Body.String(), "email is required")
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
				echoCtx.Echo().HTTPErrorHandler(echoCtx, err)
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

	svc, _, _ := setupTestService(t)

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
		echoCtx1.Echo().HTTPErrorHandler(echoCtx1, err)
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
		echoCtx2.Echo().HTTPErrorHandler(echoCtx2, err)
	}

	require.NotEqual(t, http.StatusOK, rec2.Code)
}

func TestService_CreateUser_WithCache(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, _, redis := setupTestService(t)

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
		echoCtxCreate.Echo().HTTPErrorHandler(echoCtxCreate, err)
	}

	testutil.AssertStatusCode(t, recCreate, http.StatusOK)

	testutil.Eventually(t, func() bool {
		cacheKey := "user:email:" + createReq.Email
		cachedID, err := redis.Get(ctx, cacheKey).Result()

		return err == nil && cachedID != ""
	}, 2*time.Second, 100*time.Millisecond)
}
