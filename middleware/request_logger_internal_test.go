package middleware

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no query string untouched",
			in:   "/api/users",
			want: "/api/users",
		},
		{
			name: "benign params untouched",
			in:   "/api/users?page=2&limit=10",
			want: "/api/users?page=2&limit=10",
		},
		{
			name: "token redacted",
			in:   "/callback?token=super-secret&state=xyz",
			want: "/callback?state=xyz&token=REDACTED",
		},
		{
			name: "access_token redacted case-insensitively",
			in:   "/cb?Access_Token=abc",
			want: "/cb?Access_Token=REDACTED",
		},
		{
			name: "multiple sensitive params redacted",
			in:   "/cb?code=auth-code&api_key=k123&page=1",
			want: "/cb?api_key=REDACTED&code=REDACTED&page=1",
		},
		{
			name: "password redacted",
			in:   "/login?password=hunter2",
			want: "/login?password=REDACTED",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := url.Parse(testCase.in)
			require.NoError(t, err)

			require.Equal(t, testCase.want, redactQueryParams(parsed))
		})
	}
}
