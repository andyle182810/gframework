package httpclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyle182810/gframework/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestGetJSON_ReturnsTypedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testUser{ID: 1, Name: "John"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	user, err := httpclient.GetJSON[testUser](t.Context(), client, "/users/1")

	require.NoError(t, err)
	require.Equal(t, 1, user.ID)
	require.Equal(t, "John", user.Name)
}

func TestPostJSON_SendsBodyAndReturnsTypedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var input testUser
		_ = json.NewDecoder(r.Body).Decode(&input)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testUser{ID: 1, Name: input.Name})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	user, err := httpclient.PostJSON[testUser](t.Context(), client, "/users", testUser{ID: 0, Name: "Jane"})

	require.NoError(t, err)
	require.Equal(t, 1, user.ID)
	require.Equal(t, "Jane", user.Name)
}

func TestPutJSON_SendsBodyAndReturnsTypedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testUser{ID: 1, Name: "Updated"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	user, err := httpclient.PutJSON[testUser](t.Context(), client, "/users/1", testUser{ID: 0, Name: "Updated"})

	require.NoError(t, err)
	require.Equal(t, "Updated", user.Name)
}

func TestPatchJSON_SendsBodyAndReturnsTypedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testUser{ID: 1, Name: "Patched"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	user, err := httpclient.PatchJSON[testUser](t.Context(), client, "/users/1", map[string]string{"name": "Patched"})

	require.NoError(t, err)
	require.Equal(t, "Patched", user.Name)
}

func TestDeleteJSON_ReturnsTypedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	result, err := httpclient.DeleteJSON[map[string]bool](t.Context(), client, "/users/1")

	require.NoError(t, err)
	require.True(t, result["deleted"])
}

func TestDoJSON_SupportsCustomMethod(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"method": r.Method})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	result, err := httpclient.DoJSON[map[string]string](t.Context(), client, "CUSTOM", "/test", nil)

	require.NoError(t, err)
	require.Equal(t, "CUSTOM", result["method"])
}

func TestGetJSON_SupportsRequestOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Custom"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testUser{ID: 1, Name: "John"})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	user, err := httpclient.GetJSON[testUser](t.Context(), client, "/users",
		httpclient.WithRequestHeader("X-Custom", "value"),
		httpclient.WithQuery("page", "1"),
	)

	require.NoError(t, err)
	require.Equal(t, 1, user.ID)
}

func TestGetJSON_ReturnsServiceErrorOnFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(responseWriter).Encode(httpclient.ErrorResponse{
			Message:  "User not found",
			Internal: "no rows in result set",
		})
	}))
	defer server.Close()

	client := httpclient.New(server.URL)

	_, err := httpclient.GetJSON[testUser](t.Context(), client, "/users/999")

	svcErr, ok := httpclient.IsServiceError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, svcErr.StatusCode)
	require.Equal(t, "User not found", svcErr.Message)
	require.Equal(t, "no rows in result set", svcErr.Internal)
}
