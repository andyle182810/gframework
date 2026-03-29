//nolint:paralleltest,thelper
package service_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/service"
	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

func TestService_ListUsers(t *testing.T) { //nolint:funlen
	testutil.SkipIfShort(t)

	ctx := t.Context()

	svc, repository, _ := setupTestService(t)

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
				require.LessOrEqual(t, len(resp.Data.Users), httpserver.DefaultPageSize)
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
				require.Len(t, resp.Data.Users, 5)
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
				require.Len(t, resp.Data.Users, 5)
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
				require.Equal(t, httpserver.DefaultPage, resp.Pagination.Page)
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
				require.Equal(t, httpserver.MaxPageSize, resp.Pagination.PageSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
				Method:      http.MethodGet,
				Path:        "/v1/users",
				QueryParams: tt.queryParams,
			})

			handler := httpserver.Wrapper(svc.ListUsers)

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
