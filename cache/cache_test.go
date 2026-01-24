package cache_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/andyle182810/gframework/cache"
	"github.com/andyle182810/gframework/testutil"
	"github.com/andyle182810/gframework/valkey"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setupCache[K any, V any](t *testing.T, hashKey string, ttl time.Duration, encoder cache.KeyEncoder) *cache.Cache[K, V] {
	t.Helper()

	container := testutil.SetupValkeyContainer(t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &valkey.Config{ //nolint:exhaustruct
		Host: container.Host,
		Port: port,
	}

	client, err := valkey.New(opts)
	require.NoError(t, err)

	return cache.New[K, V](client, hashKey, ttl, encoder)
}

type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestCacheSetAndGet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	user := &TestUser{
		ID:   1,
		Name: "John Doe",
		Age:  30,
	}

	err := c.Set(ctx, "user:1", user)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, "user:1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, user.ID, retrieved.ID)
	require.Equal(t, user.Name, retrieved.Name)
	require.Equal(t, user.Age, retrieved.Age)
}

func TestCacheGetNonExistentKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	retrieved, err := c.Get(ctx, "nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrKeyNotFound)
	require.Nil(t, retrieved)
}

func TestCacheDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	user := &TestUser{ID: 1, Name: "Jane Doe", Age: 25}

	err := c.Set(ctx, "user:1", user)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, "user:1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	err = c.Delete(ctx, "user:1")
	require.NoError(t, err)

	retrieved, err = c.Get(ctx, "user:1")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrKeyNotFound)
	require.Nil(t, retrieved)
}

func TestCacheInvalidate(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	user1 := &TestUser{ID: 1, Name: "User 1", Age: 20}
	user2 := &TestUser{ID: 2, Name: "User 2", Age: 30}

	err := c.Set(ctx, "user:1", user1)
	require.NoError(t, err)

	err = c.Set(ctx, "user:2", user2)
	require.NoError(t, err)

	err = c.Invalidate(ctx)
	require.NoError(t, err)

	retrieved1, err := c.Get(ctx, "user:1")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrKeyNotFound)
	require.Nil(t, retrieved1)

	retrieved2, err := c.Get(ctx, "user:2")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrKeyNotFound)
	require.Nil(t, retrieved2)
}

func TestCacheDefaultTTL(t *testing.T) {
	t.Parallel()

	container := testutil.SetupValkeyContainer(t)

	port, err := strconv.Atoi(container.Port.Port())
	require.NoError(t, err)

	opts := &valkey.Config{ //nolint:exhaustruct
		Host: container.Host,
		Port: port,
	}

	client, err := valkey.New(opts)
	require.NoError(t, err)

	c := cache.New[string, TestUser](client, "users", 0, cache.NewStringKeyEncoder())

	ctx := t.Context()
	user := &TestUser{ID: 1, Name: "Test User", Age: 25}

	err = c.Set(ctx, "user:1", user)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, "user:1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
}

func TestCacheWithIntKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[int, TestUser](t, "users_by_id", time.Minute, cache.NewIntKeyEncoder())

	user := &TestUser{ID: 42, Name: "Int Key User", Age: 35}

	err := c.Set(ctx, 42, user)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, user.ID, retrieved.ID)
	require.Equal(t, user.Name, retrieved.Name)
}

func TestCacheWithUUIDKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[uuid.UUID, TestUser](t, "users_by_uuid", time.Minute, cache.NewUUIDKeyEncoder())

	userID := uuid.New()
	user := &TestUser{ID: 1, Name: "UUID Key User", Age: 28}

	err := c.Set(ctx, userID, user)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, user.Name, retrieved.Name)
}

func TestCacheUpdateExistingKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	user := &TestUser{ID: 1, Name: "Original Name", Age: 25}

	err := c.Set(ctx, "user:1", user)
	require.NoError(t, err)

	updatedUser := &TestUser{ID: 1, Name: "Updated Name", Age: 26}

	err = c.Set(ctx, "user:1", updatedUser)
	require.NoError(t, err)

	retrieved, err := c.Get(ctx, "user:1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, updatedUser.Name, retrieved.Name)
	require.Equal(t, updatedUser.Age, retrieved.Age)
}

func TestCacheMultipleKeys(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := setupCache[string, TestUser](t, "users", time.Minute, cache.NewStringKeyEncoder())

	users := []*TestUser{
		{ID: 1, Name: "User 1", Age: 20},
		{ID: 2, Name: "User 2", Age: 30},
		{ID: 3, Name: "User 3", Age: 40},
	}

	for i, user := range users {
		err := c.Set(ctx, "user:"+strconv.Itoa(i+1), user)
		require.NoError(t, err)
	}

	for i, user := range users {
		retrieved, err := c.Get(ctx, "user:"+strconv.Itoa(i+1))
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		require.Equal(t, user.Name, retrieved.Name)
	}
}

func TestStringKeyEncoder(t *testing.T) {
	t.Parallel()

	encoder := cache.NewStringKeyEncoder()

	encoded, err := encoder.Encode("test-key")
	require.NoError(t, err)
	require.Equal(t, "test-key", encoded)

	_, err = encoder.Encode(123)
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrCacheInvalidKeyType)
}

func TestIntKeyEncoder(t *testing.T) {
	t.Parallel()

	encoder := cache.NewIntKeyEncoder()

	encoded, err := encoder.Encode(42)
	require.NoError(t, err)
	require.Equal(t, "42", encoded)

	_, err = encoder.Encode("not-an-int")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrCacheInvalidKeyType)
}

func TestUUIDKeyEncoder(t *testing.T) {
	t.Parallel()

	encoder := cache.NewUUIDKeyEncoder()

	id := uuid.New()
	encoded, err := encoder.Encode(id)
	require.NoError(t, err)
	require.Equal(t, id.String(), encoded)

	_, err = encoder.Encode("not-a-uuid")
	require.Error(t, err)
	require.ErrorIs(t, err, cache.ErrCacheInvalidKeyType)
}

func TestKeyEncoderValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		encoder     cache.KeyEncoder
		key         any
		expectError bool
	}{
		{
			name:        "string encoder with string key",
			encoder:     cache.NewStringKeyEncoder(),
			key:         "valid",
			expectError: false,
		},
		{
			name:        "string encoder with int key",
			encoder:     cache.NewStringKeyEncoder(),
			key:         123,
			expectError: true,
		},
		{
			name:        "int encoder with int key",
			encoder:     cache.NewIntKeyEncoder(),
			key:         456,
			expectError: false,
		},
		{
			name:        "int encoder with string key",
			encoder:     cache.NewIntKeyEncoder(),
			key:         "invalid",
			expectError: true,
		},
		{
			name:        "uuid encoder with uuid key",
			encoder:     cache.NewUUIDKeyEncoder(),
			key:         uuid.New(),
			expectError: false,
		},
		{
			name:        "uuid encoder with string key",
			encoder:     cache.NewUUIDKeyEncoder(),
			key:         "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.encoder.Encode(tt.key)
			if tt.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, cache.ErrCacheInvalidKeyType)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
