package keycloak_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andyle182810/gframework/keycloak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRealm = "my-realm"

// keycloakStub is a minimal fake of the subset of Keycloak endpoints used by
// AdminClient tests. Any unexpected path returns 501 so missing routes fail
// loud rather than silently returning empty results.
type keycloakStub struct {
	tokenCalls       atomic.Int32
	createUserCalls  atomic.Int32
	setPasswordCalls atomic.Int32
	getRoleCalls     atomic.Int32
	assignRoleCalls  atomic.Int32

	tokenDelay      time.Duration
	setPasswordFail bool
	roleNotFound    bool
	expiresIn       int
	createdUserID   string
}

func newStub() *keycloakStub {
	return &keycloakStub{ //nolint:exhaustruct
		expiresIn:     3600,
		createdUserID: "user-abc-123",
	}
}

// newServer returns an httptest.Server wired to the stub's handler. Path
// matching mirrors Keycloak's URL layout exactly so gocloak/v13 sees valid
// endpoints.
func (s *keycloakStub) newServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/realms/"+testRealm+"/protocol/openid-connect/token", s.handleToken)

	adminPrefix := "/admin/realms/" + testRealm
	mux.HandleFunc(adminPrefix+"/users", s.handleUsers)
	mux.HandleFunc(adminPrefix+"/users/", s.handleUserSub)
	mux.HandleFunc(adminPrefix+"/roles/", s.handleRole)

	return httptest.NewServer(mux)
}

func (s *keycloakStub) handleToken(w http.ResponseWriter, _ *http.Request) {
	s.tokenCalls.Add(1)

	if s.tokenDelay > 0 {
		time.Sleep(s.tokenDelay)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w,
		`{"access_token":"svc-token","token_type":"Bearer","expires_in":%d}`,
		s.expiresIn,
	)
}

func (s *keycloakStub) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotImplemented)

		return
	}

	s.createUserCalls.Add(1)

	location := "/admin/realms/" + testRealm + "/users/" + s.createdUserID
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
}

// handleUserSub dispatches paths under /admin/realms/{realm}/users/.
func (s *keycloakStub) handleUserSub(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/reset-password") && r.Method == http.MethodPut:
		s.setPasswordCalls.Add(1)

		if s.setPasswordFail {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		w.WriteHeader(http.StatusNoContent)

	case strings.HasSuffix(r.URL.Path, "/role-mappings/realm") && r.Method == http.MethodPost:
		s.assignRoleCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"username":"alice"}`, s.createdUserID)

	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (s *keycloakStub) handleRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotImplemented)

		return
	}

	s.getRoleCalls.Add(1)

	if s.roleNotFound {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	roleName := parts[len(parts)-1]

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"id":"role-id","name":%q}`, roleName)
}

func TestClient_CreateUser_Happy(t *testing.T) {
	t.Parallel()

	stub := newStub()
	server := stub.newServer(t)

	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	id, err := client.CreateUser(t.Context(), keycloak.CreateUserParams{
		Username:  "alice",
		Email:     "alice@example.com",
		FirstName: "Alice",
		LastName:  "Liddell",
		Password:  "s3cret",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-abc-123", id)
	assert.Equal(t, int32(1), stub.createUserCalls.Load())
	assert.Equal(t, int32(1), stub.setPasswordCalls.Load())
}

func TestClient_CreateUser_SetPasswordFailureReturnsUserID(t *testing.T) {
	t.Parallel()

	stub := newStub()
	stub.setPasswordFail = true

	server := stub.newServer(t)
	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	id, err := client.CreateUser(t.Context(), keycloak.CreateUserParams{
		Username:  "alice",
		Email:     "alice@example.com",
		FirstName: "Alice",
		LastName:  "Liddell",
		Password:  "s3cret",
	})

	require.Error(t, err)
	require.Equal(t, "user-abc-123", id,
		"id must be non-empty so the caller can roll back the partially-created user")
}

func TestClient_CreateUser_InvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   keycloak.CreateUserParams
	}{
		{
			name: "empty username",
			in: keycloak.CreateUserParams{
				Username: "", Email: "a@b", FirstName: "", LastName: "", Password: "p",
			},
		},
		{
			name: "empty email",
			in: keycloak.CreateUserParams{
				Username: "u", Email: "", FirstName: "", LastName: "", Password: "p",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := newStub()
			server := stub.newServer(t)

			defer server.Close()

			client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

			id, err := client.CreateUser(t.Context(), tt.in)

			require.Error(t, err)
			require.ErrorIs(t, err, keycloak.ErrInvalidInput)
			require.Empty(t, id)
			// Validation runs before the token fetch.
			require.Equal(t, int32(0), stub.tokenCalls.Load())
		})
	}
}

func TestClient_InvalidInput_AcrossMethods(t *testing.T) {
	t.Parallel()

	stub := newStub()
	server := stub.newServer(t)

	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	ctx := t.Context()

	_, err := client.GetUser(ctx, "")
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	err = client.SetEnabled(ctx, "", true)
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	err = client.UpdateUser(ctx, "", keycloak.UpdateUserParams{}) //nolint:exhaustruct
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	_, err = client.UserRoles(ctx, "")
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	err = client.AddRole(ctx, "", "role")
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	err = client.RemoveRole(ctx, "id", "")
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	_, err = client.ListUsers(ctx, -1, 10)
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	_, err = client.ListUsers(ctx, 0, 0)
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	err = client.SendActionsEmail(ctx, "id", nil)
	require.ErrorIs(t, err, keycloak.ErrInvalidInput)

	// No outbound calls were made during validation failures.
	require.Equal(t, int32(0), stub.tokenCalls.Load())
}

func TestClient_Token_CachesAcrossCalls(t *testing.T) {
	t.Parallel()

	stub := newStub()
	server := stub.newServer(t)

	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	for range 5 {
		_, err := client.CreateUser(t.Context(), keycloak.CreateUserParams{
			Username:  "alice",
			Email:     "alice@example.com",
			FirstName: "",
			LastName:  "",
			Password:  "",
		})
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), stub.tokenCalls.Load(), "token should be cached across calls")
	assert.Equal(t, int32(5), stub.createUserCalls.Load())
}

func TestClient_Token_ConcurrentRefreshTakesOneLogin(t *testing.T) {
	t.Parallel()

	stub := newStub()
	// Add a small delay so multiple goroutines race into refreshToken before
	// the first one completes; the RWMutex double-check must still collapse
	// them into a single login.
	stub.tokenDelay = 40 * time.Millisecond

	server := stub.newServer(t)
	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	const workers = 20

	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			_, _ = client.GetUser(t.Context(), "user-abc-123")
		})
	}

	wg.Wait()

	assert.Equal(t, int32(1), stub.tokenCalls.Load(),
		"concurrent callers must share exactly one token refresh")
}

func TestClient_InvalidateToken_ForcesRefresh(t *testing.T) {
	t.Parallel()

	stub := newStub()
	server := stub.newServer(t)

	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")
	ctx := t.Context()

	_, err := client.GetUser(ctx, "user-abc-123")
	require.NoError(t, err)
	require.Equal(t, int32(1), stub.tokenCalls.Load())

	client.InvalidateToken()

	_, err = client.GetUser(ctx, "user-abc-123")
	require.NoError(t, err)
	assert.Equal(t, int32(2), stub.tokenCalls.Load(),
		"InvalidateToken should cause the next call to re-login")
}

func TestClient_Token_SafetyBufferTriggersRefresh(t *testing.T) {
	t.Parallel()

	stub := newStub()
	// Very short-lived token; the safety buffer consumes the entire lifetime,
	// so every call should trigger a refresh.
	stub.expiresIn = 1

	server := stub.newServer(t)
	defer server.Close()

	client := keycloak.NewAdminClient(
		server.URL, testRealm, "svc", "sec",
		keycloak.WithTokenSafetyBuffer(5*time.Second),
	)

	ctx := t.Context()

	for range 3 {
		_, err := client.GetUser(ctx, "user-abc-123")
		require.NoError(t, err)
	}

	assert.Equal(t, int32(3), stub.tokenCalls.Load(),
		"safety buffer > expires_in should force refresh on every call")
}

func TestClient_AddRole_ReturnsErrRoleNotFound(t *testing.T) {
	t.Parallel()

	stub := newStub()
	stub.roleNotFound = true

	server := stub.newServer(t)
	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	err := client.AddRole(t.Context(), "user-abc-123", "admin")

	require.Error(t, err)
	require.ErrorIs(t, err, keycloak.ErrRoleNotFound)
	require.Equal(t, int32(1), stub.getRoleCalls.Load())
	require.Equal(t, int32(0), stub.assignRoleCalls.Load(),
		"assignment must not fire when the role lookup 404s")
}

func TestClient_AddRole_Happy(t *testing.T) {
	t.Parallel()

	stub := newStub()
	server := stub.newServer(t)

	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, testRealm, "svc", "sec")

	err := client.AddRole(t.Context(), "user-abc-123", "admin")

	require.NoError(t, err)
	assert.Equal(t, int32(1), stub.getRoleCalls.Load())
	assert.Equal(t, int32(1), stub.assignRoleCalls.Load())
}
