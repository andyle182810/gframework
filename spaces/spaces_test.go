package spaces_test

import (
	"strings"
	"testing"
	"time"

	"github.com/andyle182810/gframework/spaces"
	"github.com/stretchr/testify/require"
)

func validOptions() spaces.Options {
	return spaces.Options{
		Region:    "nyc3",
		Endpoint:  "https://nyc3.digitaloceanspaces.com",
		Bucket:    "test-bucket",
		KeyPrefix: "tenant-1/",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}
}

func newTestClient(t *testing.T) *spaces.Client {
	t.Helper()

	client, err := spaces.New(t.Context(), validOptions())
	require.NoError(t, err)

	return client
}

func TestNew_ValidatesOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*spaces.Options)
	}{
		{name: "missing region", mutate: func(o *spaces.Options) { o.Region = "" }},
		{name: "missing endpoint", mutate: func(o *spaces.Options) { o.Endpoint = "" }},
		{name: "missing bucket", mutate: func(o *spaces.Options) { o.Bucket = "" }},
		{name: "missing access key", mutate: func(o *spaces.Options) { o.AccessKey = "" }},
		{name: "missing secret key", mutate: func(o *spaces.Options) { o.SecretKey = "" }},
		{name: "prefix without trailing slash", mutate: func(o *spaces.Options) { o.KeyPrefix = "tenant-1" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := validOptions()
			tc.mutate(&opts)

			_, err := spaces.New(t.Context(), opts)

			require.ErrorIs(t, err, spaces.ErrInvalidOptions)
		})
	}
}

func TestNew_EmptyPrefixIsAllowed(t *testing.T) {
	t.Parallel()

	opts := validOptions()
	opts.KeyPrefix = ""

	_, err := spaces.New(t.Context(), opts)

	require.NoError(t, err)
}

func TestPresignGet_RejectsMaliciousKeys(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "parent traversal", key: "../tenant-2/secret.txt"},
		{name: "embedded traversal", key: "files/../../tenant-2/secret.txt"},
		{name: "current dir segment", key: "files/./secret.txt"},
		{name: "leading slash", key: "/absolute/path"},
		{name: "double slash", key: "files//secret.txt"},
		{name: "control character", key: "files/secret\n.txt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.PresignGet(t.Context(), tc.key, time.Minute)

			require.ErrorIs(t, err, spaces.ErrInvalidKey)
		})
	}
}

func TestPresignGet_ValidKeyIncludesPrefix(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	url, err := client.PresignGet(t.Context(), "avatars/user.png", time.Minute)

	require.NoError(t, err)
	require.Contains(t, url, "tenant-1/avatars/user.png")
	require.True(t, strings.HasPrefix(url, "https://"))
}

func TestPresignPut_ValidatesTTL(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{name: "zero TTL", ttl: 0},
		{name: "negative TTL", ttl: -time.Minute},
		{name: "TTL above S3 limit", ttl: spaces.MaxPresignTTL + time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.PresignPut(t.Context(), "file.txt", "text/plain", 10, tc.ttl)

			require.ErrorIs(t, err, spaces.ErrInvalidTTL)
		})
	}
}

func TestPresignPut_ValidRequestSucceeds(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	url, err := client.PresignPut(t.Context(), "uploads/file.txt", "text/plain", 1024, 15*time.Minute)

	require.NoError(t, err)
	require.Contains(t, url, "tenant-1/uploads/file.txt")
}

func TestDelete_RejectsInvalidKey(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	err := client.Delete(t.Context(), "../escape")

	require.ErrorIs(t, err, spaces.ErrInvalidKey)
}

func TestHead_RejectsInvalidKey(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	_, err := client.Head(t.Context(), "../escape")

	require.ErrorIs(t, err, spaces.ErrInvalidKey)
}

func TestRangeGet_RejectsInvalidKey(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	_, err := client.RangeGet(t.Context(), "../escape", 64)

	require.ErrorIs(t, err, spaces.ErrInvalidKey)
}
