package keycloak_test

import (
	"testing"

	"github.com/Nerzal/gocloak/v13"
	"github.com/andyle182810/gframework/keycloak"
	"github.com/stretchr/testify/assert"
)

func TestRoleNames(t *testing.T) {
	t.Parallel()

	name := func(s string) *string { return &s }

	cases := []struct {
		name string
		in   []*gocloak.Role
		want []string
	}{
		{
			name: "nil slice",
			in:   nil,
			want: []string{},
		},
		{
			name: "empty slice",
			in:   []*gocloak.Role{},
			want: []string{},
		},
		{
			name: "nil entries skipped",
			in:   []*gocloak.Role{nil, {Name: name("admin")}, nil},
			want: []string{"admin"},
		},
		{
			name: "nil names skipped",
			in: []*gocloak.Role{
				{Name: nil},
				{Name: name("ops")},
			},
			want: []string{"ops"},
		},
		{
			name: "populated",
			in: []*gocloak.Role{
				{Name: name("admin")},
				{Name: name("user")},
				{Name: name("guest")},
			},
			want: []string{"admin", "user", "guest"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, keycloak.RoleNames(tt.in))
		})
	}
}
