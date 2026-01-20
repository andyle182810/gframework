//nolint:paralleltest
package service_test

import (
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

func TestService_ReadinessCheck(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	svc, _, _ := setupTestService(ctx, t)

	echoCtx, rec, _ := testutil.SetupEchoContext(t, &testutil.Options{ //nolint:exhaustruct
		Method: http.MethodGet,
		Path:   "/ready",
	})

	handler := httpserver.Wrapper(svc.CheckReadiness)

	err := handler(echoCtx)
	if err != nil {
		echoCtx.Echo().HTTPErrorHandler(echoCtx, err)
	}

	testutil.AssertStatusCode(t, rec, http.StatusOK)
	testutil.AssertSuccessResponse(t, rec)

	var resp struct {
		Data struct {
			Status   string         `json:"status"`
			Services map[string]any `json:"services"`
		} `json:"data"`
	}

	testutil.AssertJSONResponse(t, rec, &resp)
	require.Equal(t, "ready", resp.Data.Status)
	require.NotNil(t, resp.Data.Services)

	require.Contains(t, resp.Data.Services, "postgres")
	require.Contains(t, resp.Data.Services, "valkey")
}
