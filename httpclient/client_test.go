package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyle182810/gframework/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTokenFetchFailed = errors.New("token fetch failed")
	errSomeOtherError   = errors.New("some other error")
)

type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetToken(_ context.Context) (string, error) {
	return m.token, m.err
}

func (m *mockTokenProvider) InvalidateToken() {}

func TestNew_CreatesClientWithDefaultSettings(t *testing.T) {
	t.Parallel()

	client := httpclient.New("https://api.example.com")

	require.NotNil(t, client)
	require.Equal(t, "https://api.example.com", client.BaseURL())
}

func TestNew_TrimsTrailingSlashFromBaseURL(t *testing.T) {
	t.Parallel()

	client := httpclient.New("https://api.example.com/")

	require.Equal(t, "https://api.example.com", client.BaseURL())
}

func TestNew_AppliesMaxResponseSizeOption(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")

		data := make([]byte, 1024)
		for idx := range data {
			data[idx] = 'a'
		}

		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"data": string(data)})
	}))
	defer server.Close()

	client := httpclient.New(server.URL, httpclient.WithMaxResponseSize(100))

	var response map[string]string
	err := client.Get(t.Context(), "/test", &response)

	require.ErrorIs(t, err, httpclient.ErrResponseTooLarge)
}

func TestWithTimeout_SetsClientTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL, httpclient.WithTimeout(10*time.Millisecond))

	err := client.Get(t.Context(), "/test", nil)

	require.Error(t, err)
}

func TestWithTokenProvider_AddsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "test-token", err: nil}
	client := httpclient.New(server.URL, httpclient.WithTokenProvider(provider))

	err := client.Get(t.Context(), "/test", nil)

	require.NoError(t, err)
}

func TestWithTokenProvider_ReturnsErrorWhenProviderFails(t *testing.T) {
	t.Parallel()

	provider := &mockTokenProvider{token: "", err: errTokenFetchFailed}
	client := httpclient.New("https://api.example.com", httpclient.WithTokenProvider(provider))

	err := client.Get(t.Context(), "/test", nil)

	require.ErrorIs(t, err, httpclient.ErrAuthFailed)
}

func TestWithDefaultHeaders_SetsCustomHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "secret", r.Header.Get("X-Api-Key"))
		assert.Equal(t, "1.0", r.Header.Get("X-Client-Ver"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL, httpclient.WithDefaultHeaders(map[string]string{
		"X-Api-Key":    "secret",
		"X-Client-Ver": "1.0",
	}))

	err := client.Get(t.Context(), "/test", nil)

	require.NoError(t, err)
}

func TestClient_Get(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]string
	err := client.Get(t.Context(), "/test", &response)

	require.NoError(t, err)
	require.Equal(t, "success", response["message"])
}

func TestClient_Post(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test", body["name"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"id": 1})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]int
	err := client.Post(t.Context(), "/test", map[string]string{"name": "test"}, &response)

	require.NoError(t, err)
	require.Equal(t, 1, response["id"])
}

func TestClient_Put(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]string
	err := client.Put(t.Context(), "/test/1", map[string]string{"name": "updated"}, &response)

	require.NoError(t, err)
	require.Equal(t, "updated", response["status"])
}

func TestClient_Patch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "patched"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]string
	err := client.Patch(t.Context(), "/test/1", map[string]string{"field": "value"}, &response)

	require.NoError(t, err)
	require.Equal(t, "patched", response["status"])
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]bool
	err := client.Delete(t.Context(), "/test/1", &response)

	require.NoError(t, err)
	require.True(t, response["deleted"])
}

func TestClient_Do(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"method": r.Method})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	var response map[string]string
	err := client.Do(t.Context(), "CUSTOM", "/test", nil, &response)

	require.NoError(t, err)
	require.Equal(t, "CUSTOM", response["method"])
}

func TestClient_Head(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	response, err := client.Head(t.Context(), "/test")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "test-value", response.Headers["X-Custom-Header"])
}

func TestClient_Options(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodOptions, r.Method)
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	response, err := client.Options(t.Context(), "/test")

	require.NoError(t, err)
	require.Equal(t, "GET, POST, PUT, DELETE", response.Headers["Allow"])
}

func TestClient_NilResponseIsHandled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Delete(t.Context(), "/test/1", nil)

	require.NoError(t, err)
}

func TestWithRequestHeader_SetsCustomHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "custom-value", req.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil, httpclient.WithRequestHeader("X-Custom", "custom-value"))

	require.NoError(t, err)
}

func TestWithRequestTimeout_TimesOutRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil, httpclient.WithRequestTimeout(10*time.Millisecond))

	require.ErrorIs(t, err, httpclient.ErrRequestFailed)
}

func TestWithRequestID_SetsRequestIDHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "custom-request-id", req.Header.Get("X-Request-Id"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil, httpclient.WithRequestID("custom-request-id"))

	require.NoError(t, err)
}

func TestWithQuery_SetsSingleQueryParam(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.URL.Query().Get("key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil, httpclient.WithQuery("key", "value"))

	require.NoError(t, err)
}

func TestWithQueryParams_SetsMultipleQueryParams(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bar", r.URL.Query().Get("foo"))
		assert.Equal(t, "qux", r.URL.Query().Get("baz"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil, httpclient.WithQueryParams(map[string]string{
		"foo": "bar",
		"baz": "qux",
	}))

	require.NoError(t, err)
}

func TestWithQuery_URLEncodesSpecialCharacters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test@example.com", r.URL.Query().Get("email"))
		assert.Equal(t, "John Doe", r.URL.Query().Get("name"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil,
		httpclient.WithQuery("email", "test@example.com"),
		httpclient.WithQuery("name", "John Doe"),
	)

	require.NoError(t, err)
}

func TestWithRequestIDKey_ExtractsRequestIDFromContext(t *testing.T) {
	t.Parallel()

	type ctxKey string

	key := ctxKey("request-id")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ctx-request-id", r.Header.Get("X-Request-Id"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL, httpclient.WithRequestIDKey(key))
	ctx := context.WithValue(t.Context(), key, "ctx-request-id")

	err := client.Get(ctx, "/test", nil)

	require.NoError(t, err)
}

func TestClient_ParsesJSONErrorResponseWithMessageAndInternal(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(httpclient.ErrorResponse{
			Message:  "Service Unavailable",
			Internal: "database connection failed",
		})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil)

	svcErr, ok := httpclient.IsServiceError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusServiceUnavailable, svcErr.StatusCode)
	require.Equal(t, "Service Unavailable", svcErr.Message)
	require.Equal(t, "database connection failed", svcErr.Internal)
}

func TestClient_HandlesNonJSONErrorResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Internal Server Error")
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil)

	svcErr, ok := httpclient.IsServiceError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusInternalServerError, svcErr.StatusCode)
	require.Equal(t, "Internal Server Error", svcErr.Message)
}

func TestClient_Handles404Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(httpclient.ErrorResponse{
			Message:  "Not Found",
			Internal: "",
		})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/test", nil)

	svcErr, ok := httpclient.IsServiceError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, svcErr.StatusCode)
}

func TestClient_ReturnsErrRequestFailedOnNetworkError(t *testing.T) {
	t.Parallel()

	client := httpclient.New("http://invalid-host-that-does-not-exist.local")

	err := client.Get(t.Context(), "/test", nil)

	require.ErrorIs(t, err, httpclient.ErrRequestFailed)
}

func TestServiceError_ReturnsMessageWhenSet(t *testing.T) {
	t.Parallel()

	err := httpclient.NewServiceError(500, "Something went wrong", "internal error", "req-123")

	require.Equal(t, "Something went wrong", err.Error())
}

func TestServiceError_ReturnsDefaultMessageWhenMessageIsEmpty(t *testing.T) {
	t.Parallel()

	err := httpclient.NewServiceError(500, "", "", "req-123")

	require.Equal(t, "httpclient: service returned status 500", err.Error())
}

func TestServiceError_IsReturnsTrueForErrServiceError(t *testing.T) {
	t.Parallel()

	err := httpclient.NewServiceError(500, "error", "", "req-123")

	require.ErrorIs(t, err, httpclient.ErrServiceError)
}

func TestServiceError_UnwrapReturnsErrServiceError(t *testing.T) {
	t.Parallel()

	err := httpclient.NewServiceError(500, "error", "", "req-123")

	require.Equal(t, httpclient.ErrServiceError, err.Unwrap())
}

func TestIsServiceError_ExtractsServiceErrorFromErrorChain(t *testing.T) {
	t.Parallel()

	svcErr := httpclient.NewServiceError(404, "Not Found", "record not in database", "req-123")

	extracted, ok := httpclient.IsServiceError(svcErr)

	require.True(t, ok)
	require.Equal(t, 404, extracted.StatusCode)
	require.Equal(t, "record not in database", extracted.Internal)
}

func TestIsServiceError_ReturnsFalseForNonServiceError(t *testing.T) {
	t.Parallel()

	err := errSomeOtherError

	_, ok := httpclient.IsServiceError(err)

	require.False(t, ok)
}

func TestClient_BuildsURLWithoutLeadingSlash(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "users", nil)

	require.NoError(t, err)
}

func TestClient_BuildsURLWithLeadingSlash(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/users", req.URL.Path)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/users", nil)

	require.NoError(t, err)
}

func TestClient_BuildsURLWithEmptyPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/", req.URL.Path)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "", nil)

	require.NoError(t, err)
}

func TestClient_BuildsURLWithQueryParams(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	err := client.Get(t.Context(), "/users", nil, httpclient.WithQueryParams(map[string]string{
		"page":  "1",
		"limit": "10",
	}))

	require.NoError(t, err)
}

func TestClient_BaseURLReturnsConfiguredURL(t *testing.T) {
	t.Parallel()

	client := httpclient.New("https://api.example.com")

	require.Equal(t, "https://api.example.com", client.BaseURL())
}
