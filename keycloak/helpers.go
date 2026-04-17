package keycloak

import (
	"github.com/Nerzal/gocloak/v13"
)

func RoleNames(roles []*gocloak.Role) []string {
	out := make([]string, 0, len(roles))

	for _, r := range roles {
		if r != nil && r.Name != nil {
			out = append(out, *r.Name)
		}
	}

	return out
}
