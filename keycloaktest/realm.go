package keycloaktest

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type Realm struct {
	realmRoles  map[string]realmRole
	clientRoles map[string]map[string]realmRole
	clients     map[string]realmClient
	users       []realmUser
	scope       realmScope
}

type realmComposites struct {
	Realm  []string            `json:"realm"`
	Client map[string][]string `json:"client"`
}

type realmRole struct {
	Name       string          `json:"name"`
	Composites realmComposites `json:"composites"`
}

type realmClient struct {
	ClientID         string `json:"clientId"`
	FullScopeAllowed *bool  `json:"fullScopeAllowed"`
}

type realmUser struct {
	Username               string              `json:"username"`
	ServiceAccountClientID string              `json:"serviceAccountClientId"`
	RealmRoles             []string            `json:"realmRoles"`
	ClientRoles            map[string][]string `json:"clientRoles"`
}

type realmScopeMapping struct {
	Client string   `json:"client"`
	Roles  []string `json:"roles"`
}

type realmScope struct {
	realm  map[string][]string
	client map[string]map[string][]string
}

type realmExport struct {
	Roles struct {
		Realm  []realmRole            `json:"realm"`
		Client map[string][]realmRole `json:"client"`
	} `json:"roles"`
	Clients             []realmClient                  `json:"clients"`
	Users               []realmUser                    `json:"users"`
	ScopeMappings       []realmScopeMapping            `json:"scopeMappings"`
	ClientScopeMappings map[string][]realmScopeMapping `json:"clientScopeMappings"`
}

func LoadRealm(t *testing.T, path string) *Realm {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var export realmExport

	require.NoError(t, json.Unmarshal(raw, &export))

	realm := &Realm{
		realmRoles:  map[string]realmRole{},
		clientRoles: map[string]map[string]realmRole{},
		clients:     map[string]realmClient{},
		users:       export.Users,
		scope:       realmScope{realm: map[string][]string{}, client: map[string]map[string][]string{}},
	}

	for _, role := range export.Roles.Realm {
		realm.realmRoles[role.Name] = role
	}

	for client, roles := range export.Roles.Client {
		realm.clientRoles[client] = map[string]realmRole{}

		for _, role := range roles {
			realm.clientRoles[client][role.Name] = role
		}
	}

	for _, client := range export.Clients {
		realm.clients[client.ClientID] = client
	}

	for _, mapping := range export.ScopeMappings {
		realm.scope.realm[mapping.Client] = append(realm.scope.realm[mapping.Client], mapping.Roles...)
	}

	for roleClient, mappings := range export.ClientScopeMappings {
		for _, mapping := range mappings {
			if realm.scope.client[mapping.Client] == nil {
				realm.scope.client[mapping.Client] = map[string][]string{}
			}

			realm.scope.client[mapping.Client][roleClient] = append(
				realm.scope.client[mapping.Client][roleClient], mapping.Roles...)
		}
	}

	return realm
}

func (r *Realm) ClientRoles(client string) []string {
	roles := make([]string, 0, len(r.clientRoles[client]))
	for name := range r.clientRoles[client] {
		roles = append(roles, name)
	}

	slices.Sort(roles)

	return roles
}

func (r *Realm) UserClaims(subject string, realmRoles ...string) *middleware.ExtendedClaims {
	return r.claims(subject, "", realmRoles, nil)
}

func (r *Realm) ServiceAccountClaims(t *testing.T, clientID string) *middleware.ExtendedClaims {
	t.Helper()

	for _, user := range r.users {
		if user.ServiceAccountClientID == clientID {
			return r.claims(user.Username, clientID, user.RealmRoles, user.ClientRoles)
		}
	}

	require.Failf(t, "service account not found", "client %q has no service account", clientID)

	return nil
}

func (r *Realm) claims(
	subject, azp string,
	realmRoles []string,
	clientRoles map[string][]string,
) *middleware.ExtendedClaims {
	realmSet := map[string]bool{}
	clientSet := map[string]map[string]bool{}

	for _, role := range realmRoles {
		r.expandRealmRole(role, realmSet, clientSet)
	}

	for client, roles := range clientRoles {
		for _, role := range roles {
			r.expandClientRole(client, role, realmSet, clientSet)
		}
	}

	if client, ok := r.clients[azp]; ok && client.FullScopeAllowed != nil && !*client.FullScopeAllowed {
		r.narrowToScope(azp, realmSet, clientSet)
	}

	claims := &middleware.ExtendedClaims{ //nolint:exhaustruct
		Azp:              azp,
		RealmAccess:      middleware.RoleAccess{Roles: sortedKeys(realmSet)},
		ResourceAccess:   middleware.ResourceAccess{},
		RegisteredClaims: jwt.RegisteredClaims{Subject: subject}, //nolint:exhaustruct
	}

	for client, roles := range clientSet {
		if len(roles) > 0 {
			claims.ResourceAccess[client] = middleware.RoleAccess{Roles: sortedKeys(roles)}
		}
	}

	return claims
}

func (r *Realm) narrowToScope(azp string, realmSet map[string]bool, clientSet map[string]map[string]bool) {
	for role := range realmSet {
		if !slices.Contains(r.scope.realm[azp], role) {
			delete(realmSet, role)
		}
	}

	for client, roles := range clientSet {
		for role := range roles {
			if !slices.Contains(r.scope.client[azp][client], role) {
				delete(roles, role)
			}
		}
	}
}

func (r *Realm) expandRealmRole(name string, realmSet map[string]bool, clientSet map[string]map[string]bool) {
	role, ok := r.realmRoles[name]
	if !ok || realmSet[name] {
		return
	}

	realmSet[name] = true

	r.expandComposites(role.Composites, realmSet, clientSet)
}

func (r *Realm) expandClientRole(
	client, name string,
	realmSet map[string]bool,
	clientSet map[string]map[string]bool,
) {
	role, ok := r.clientRoles[client][name]
	if !ok || clientSet[client][name] {
		return
	}

	if clientSet[client] == nil {
		clientSet[client] = map[string]bool{}
	}

	clientSet[client][name] = true

	r.expandComposites(role.Composites, realmSet, clientSet)
}

func (r *Realm) expandComposites(
	composites realmComposites,
	realmSet map[string]bool,
	clientSet map[string]map[string]bool,
) {
	for _, role := range composites.Realm {
		r.expandRealmRole(role, realmSet, clientSet)
	}

	for client, roles := range composites.Client {
		for _, role := range roles {
			r.expandClientRole(client, role, realmSet, clientSet)
		}
	}
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}
