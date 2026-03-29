//nolint:paralleltest,thelper
package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_GetUser(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := t.Context()

	svc, repository, _ := setupTestService(t)

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
				require.Contains(t, rec.Body.String(), "UserID must be a valid UUID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
				Method: http.MethodGet,
				Path:   "/v1/users/:userId",
				PathParams: map[string]string{
					"userId": tt.userID,
				},
			})

			handler := httpserver.Wrapper(svc.GetUser)

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
