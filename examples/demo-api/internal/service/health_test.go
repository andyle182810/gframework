//nolint:paralleltest
package service_test

import (
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

func TestService_HealthCheck(t *testing.T) {
	ctx := testutil.Context(t)
	svc, _, _ := setupTestService(ctx, t)

	echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodGet,
		Path:   "/health",
	})

	handler := httpserver.Wrapper(svc.CheckHealth)

	err := handler(echoCtx)
	if err != nil {
		echoCtx.Echo().HTTPErrorHandler(err, echoCtx)
	}

	testutil.AssertStatusCode(t, rec, http.StatusOK)
	testutil.AssertSuccessResponse(t, rec)

	var resp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	testutil.AssertJSONResponse(t, rec, &resp)
	require.Equal(t, "healthy", resp.Data.Status)
}
