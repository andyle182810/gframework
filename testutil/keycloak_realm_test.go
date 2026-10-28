package testutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andyle182810/gframework/testutil"
	"github.com/stretchr/testify/require"
)

const testRealm = `{
  "roles": {
    "realm": [
      { "name": "staff", "composites": { "realm": ["viewer"], "client": { "vendor-service": ["vendors:write"] } } },
      { "name": "viewer", "composites": { "client": { "vendor-service": ["vendors:read"] } } },
      { "name": "internal_service" }
    ],
    "client": {
      "vendor-service": [{ "name": "vendors:read" }, { "name": "vendors:write" }],
      "truck-scale-service": [{ "name": "truck-scale-records:create" }, { "name": "truck-scale-records:sync" }]
    }
  },
  "clients": [
    { "clientId": "tracker", "fullScopeAllowed": false },
    { "clientId": "sync-service", "fullScopeAllowed": true }
  ],
  "users": [
    {
      "username": "service-account-tracker",
      "serviceAccountClientId": "tracker",
      "realmRoles": ["internal_service"],
      "clientRoles": { "truck-scale-service": ["truck-scale-records:create", "truck-scale-records:sync"] }
    },
    {
      "username": "service-account-sync-service",
      "serviceAccountClientId": "sync-service",
      "realmRoles": ["default-roles-grcoco", "internal_service"],
      "clientRoles": { "truck-scale-service": ["truck-scale-records:sync"] }
    }
  ],
  "clientScopeMappings": {
    "truck-scale-service": [{ "client": "tracker", "roles": ["truck-scale-records:create"] }]
  }
}`

func loadTestRealm(t *testing.T) *testutil.KeycloakRealm {
	t.Helper()

	path := filepath.Join(t.TempDir(), "realm.json")
	require.NoError(t, os.WriteFile(path, []byte(testRealm), 0o600))

	return testutil.LoadKeycloakRealm(t, path)
}

func TestKeycloakRealm_UserClaimsExpandComposites(t *testing.T) {
	t.Parallel()

	claims := loadTestRealm(t).UserClaims("user-1", "staff")

	require.Equal(t, "user-1", claims.Subject)
	require.Equal(t, []string{"staff", "viewer"}, claims.GetRealmRoles())
	require.Equal(t, []string{"vendors:read", "vendors:write"}, claims.GetResourceRoles("vendor-service"))
}

func TestKeycloakRealm_ServiceAccountClaims(t *testing.T) {
	t.Parallel()

	realm := loadTestRealm(t)

	tests := []struct {
		name       string
		clientID   string
		wantRealm  []string
		wantClient []string
	}{
		{
			name:       "full scope keeps every role",
			clientID:   "sync-service",
			wantRealm:  []string{"internal_service"},
			wantClient: []string{"truck-scale-records:sync"},
		},
		{
			name:       "restricted scope keeps only mapped roles",
			clientID:   "tracker",
			wantRealm:  []string{},
			wantClient: []string{"truck-scale-records:create"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims := realm.ServiceAccountClaims(t, tt.clientID)

			require.Equal(t, tt.clientID, claims.GetAzp())
			require.Equal(t, tt.wantRealm, claims.GetRealmRoles())
			require.Equal(t, tt.wantClient, claims.GetResourceRoles("truck-scale-service"))
		})
	}
}

func TestKeycloakRealm_ClientRoles(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"vendors:read", "vendors:write"}, loadTestRealm(t).ClientRoles("vendor-service"))
}
