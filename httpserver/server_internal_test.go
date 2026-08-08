package httpserver

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andyle182810/gframework/i18n"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

const loopbackHost = "127.0.0.1"

func TestNew_AppliesSecureTimeoutDefaults(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	require.Equal(t, DefaultReadHeaderTimeout, srv.httpServer.ReadHeaderTimeout)
	require.Equal(t, DefaultReadTimeout, srv.httpServer.ReadTimeout)
	require.Equal(t, DefaultWriteTimeout, srv.httpServer.WriteTimeout)
	require.Equal(t, DefaultIdleTimeout, srv.httpServer.IdleTimeout)
	require.Equal(t, DefaultGracePeriod, srv.gracePeriod)
}

func TestNew_ExplicitTimeoutsAreKept(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host:              loopbackHost,
		Port:              0,
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	require.Equal(t, 1*time.Second, srv.httpServer.ReadHeaderTimeout)
	require.Equal(t, 2*time.Second, srv.httpServer.ReadTimeout)
	require.Equal(t, 3*time.Second, srv.httpServer.WriteTimeout)
	require.Equal(t, 4*time.Second, srv.httpServer.IdleTimeout)
}

func TestStart_ReturnsErrorWhenPortInUse(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig

	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = listener.Close() })

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: addr.Port,
	})

	err = srv.Start(t.Context())

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to bind")
}

// Covers the wiring rather than the lookup: a catalog set on Config must reach
// the error handler, and the builtin codes must survive being merged with it.
func TestNew_RendersMessageCodesFromConfig(t *testing.T) {
	t.Parallel()

	const code = "EMPLOYEE_NOT_FOUND"

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
		Messages: i18n.Catalog{
			code: {
				i18n.English:    "Employee not found",
				i18n.Vietnamese: "Không tìm thấy nhân viên",
			},
		},
	})

	srv.Root.GET("/missing", func(_ *echo.Context) error {
		return NotFoundError(nil, code)
	})
	srv.Root.GET("/denied", func(_ *echo.Context) error {
		return ForbiddenError(nil, i18n.CodeForbidden)
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		want           string
	}{
		{
			name:           "service catalog in the requested locale",
			path:           "/missing",
			acceptLanguage: "vi",
			want:           `{"code":"EMPLOYEE_NOT_FOUND","message":"Không tìm thấy nhân viên"}`,
		},
		{
			name:           "service catalog falls back to english",
			path:           "/missing",
			acceptLanguage: "",
			want:           `{"code":"EMPLOYEE_NOT_FOUND","message":"Employee not found"}`,
		},
		{
			name:           "builtin catalog survives the merge",
			path:           "/denied",
			acceptLanguage: "vi",
			want:           `{"code":"FORBIDDEN","message":"Bạn không có quyền thực hiện thao tác này"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(
				t.Context(), http.MethodGet, "http://"+srv.Address()+tt.path, nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Language", tt.acceptLanguage)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(body))
		})
	}
}

type localizedRequest struct {
	Name        string `json:"name"        validate:"required"`
	PhoneNumber string `json:"phoneNumber" validate:"omitempty,max=5"`
}

// Request validation fails inside Wrapper, before any handler runs, so this
// covers the one error path a service cannot key itself: the codes come from
// the validate tags and the field names from the catalog.
func TestNew_LocalizesRequestValidation(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
		Messages: i18n.Catalog{
			i18n.FieldCode("name"): {
				i18n.English:    "Name",
				i18n.Vietnamese: "Tên",
			},
		},
	})

	srv.Root.POST("/things", Wrapper(
		func(_ *echo.Context, req *localizedRequest) (any, *echo.HTTPError) { return req, nil },
	))

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	tests := []struct {
		name           string
		body           string
		acceptLanguage string
		want           string
	}{
		{
			name:           "labelled field in the requested locale",
			body:           `{}`,
			acceptLanguage: "vi",
			want:           `{"code":"VALIDATION_ERROR","message":"Tên là bắt buộc"}`,
		},
		{
			name:           "no header falls back to english",
			body:           `{}`,
			acceptLanguage: "",
			want:           `{"code":"VALIDATION_ERROR","message":"Name is required"}`,
		},
		{
			name:           "the tag parameter survives translation",
			body:           `{"name":"n","phoneNumber":"0903812447"}`,
			acceptLanguage: "vi",
			want:           `{"code":"VALIDATION_ERROR","message":"phoneNumber không được vượt quá 5"}`,
		},
		{
			name:           "a malformed body is a localized bad request",
			body:           `{`,
			acceptLanguage: "vi",
			want:           `{"code":"BAD_REQUEST","message":"Yêu cầu không hợp lệ"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(
				t.Context(), http.MethodPost, "http://"+srv.Address()+"/things",
				strings.NewReader(tt.body))
			require.NoError(t, err)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.Header.Set("Accept-Language", tt.acceptLanguage)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.JSONEq(t, tt.want, string(body))
		})
	}
}

func TestStart_ServesRequests(t *testing.T) {
	t.Parallel()

	srv := New(&Config{ //nolint:exhaustruct
		Host: loopbackHost,
		Port: 0,
	})

	srv.Root.GET("/health", func(ctx *echo.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	require.NoError(t, srv.Start(t.Context()))

	t.Cleanup(func() { _ = srv.Stop() })

	resp, err := http.Get("http://" + srv.Address() + "/health") //nolint:noctx
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
