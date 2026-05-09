package cache_test

import (
	"testing"

	"github.com/andyle182810/gframework/cache"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBuildKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parts    []any
		expected string
	}{
		{
			name:     "returns empty string for no parts",
			parts:    nil,
			expected: "",
		},
		{
			name:     "returns single part as-is",
			parts:    []any{"user"},
			expected: "user",
		},
		{
			name:     "joins multiple string parts",
			parts:    []any{"user", "email", "john@example.com"},
			expected: "user:email:john@example.com",
		},
		{
			name:     "joins mixed value types",
			parts:    []any{"user", 123, uuid.Nil},
			expected: "user:123:00000000-0000-0000-0000-000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, cache.BuildKey(tt.parts...))
		})
	}
}

func TestBuildHashedKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parts    []any
		expected string
	}{
		{
			name:     "hashes multiple string parts",
			parts:    []any{"alice", "document:123", "read"},
			expected: "bf0eb987952221f2ef4c1800196bcd5101bb99a5bcdeab2399f6aaf5cf2ee6ae",
		},
		{
			name:     "hashes empty values consistently",
			parts:    []any{"", "", ""},
			expected: "565d240f5343e625ae579a4d45a770f1f02c6368b5ed4d06da4fbe6f47c28866",
		},
		{
			name:     "hashes mixed value types",
			parts:    []any{"user", 123, uuid.Nil},
			expected: "63ec4966fa69c2e6c35c442318d92ad128ff0f0a3ecef6b84c1acbef237ffe0d",
		},
		{
			name:     "hashes no parts as empty string",
			parts:    nil,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, cache.BuildHashedKey(tt.parts...))
		})
	}
}
