package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/testutil"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

var errMintFailed = errors.New("mint failed")

type mockTokenProvider struct {
	token string
	err   error
}

func (m mockTokenProvider) GetToken(_ context.Context) (string, error) {
	return m.token, m.err
}

func TestInjectInternalToken_SetsHeaderAndStripsInbound(t *testing.T) {
	t.Parallel()

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/api/test",
		Body:          nil,
		Headers:       map[string]string{middleware.HeaderXInternalAuthorization: "Bearer spoofed"},
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	mw := middleware.InjectInternalToken(mockTokenProvider{token: "minted", err: nil}, "")

	err := mw(echoSuccessHandler)(ctx)

	require.NoError(t, err)
	require.Equal(t, "Bearer minted", ctx.Request().Header.Get(middleware.HeaderXInternalAuthorization))
}

func TestInjectInternalToken_FailsClosed(t *testing.T) {
	t.Parallel()

	ctx, _, _ := testutil.SetupEchoContext(t, &testutil.Options{
		Method:        http.MethodGet,
		Path:          "/api/test",
		Body:          nil,
		Headers:       map[string]string{middleware.HeaderXInternalAuthorization: "Bearer spoofed"},
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: true,
	})

	mw := middleware.InjectInternalToken(mockTokenProvider{token: "", err: errMintFailed}, "")

	err := mw(echoSuccessHandler)(ctx)

	var httpErr *echo.HTTPError

	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusBadGateway, httpErr.Code)
	require.Empty(t, ctx.Request().Header.Get(middleware.HeaderXInternalAuthorization))
}
