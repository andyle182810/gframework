package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultTTL = time.Hour
)

var (
	ErrKeyNotFound     = errors.New("cache: key not found")
	ErrCacheMarshal    = errors.New("cache: failed to marshal value")
	ErrCacheUnmarshal  = errors.New("cache: failed to unmarshal value")
	ErrCacheGet        = errors.New("cache: failed to get")
	ErrCacheSet        = errors.New("cache: failed to set")
	ErrCacheTTL        = errors.New("cache: failed to set TTL")
	ErrCacheDelete     = errors.New("cache: failed to delete")
	ErrCacheInvalidate = errors.New("cache: failed to invalidate")
)

type Cache[K any, V any] struct {
	client     redis.UniversalClient
	ttl        time.Duration
	hashKey    string
	keyEncoder KeyEncoder
}

func New[K any, V any](
	client redis.UniversalClient,
	hashKey string,
	ttl time.Duration,
	keyEncoder KeyEncoder,
) *Cache[K, V] {
	if ttl == 0 {
		ttl = defaultTTL
	}

	return &Cache[K, V]{
		client:     client,
		ttl:        ttl,
		hashKey:    hashKey,
		keyEncoder: keyEncoder,
	}
}

func (c *Cache[K, V]) Get(ctx context.Context, key K) (*V, error) {
	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		return nil, err
	}

	data, err := c.client.HGet(ctx, c.hashKey, encodedKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrKeyNotFound
		}

		return nil, fmt.Errorf("%w: %w", ErrCacheGet, err)
	}

	var value V
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCacheUnmarshal, err)
	}

	return &value, nil
}

func (c *Cache[K, V]) Set(ctx context.Context, key K, value *V) error {
	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		return err
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCacheMarshal, err)
	}

	if err := c.client.HSet(ctx, c.hashKey, encodedKey, data).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCacheSet, err)
	}

	if err := c.client.Expire(ctx, c.hashKey, c.ttl).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCacheTTL, err)
	}

	return nil
}

func (c *Cache[K, V]) Delete(ctx context.Context, key K) error {
	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		return err
	}

	if err := c.client.HDel(ctx, c.hashKey, encodedKey).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCacheDelete, err)
	}

	return nil
}

func (c *Cache[K, V]) Invalidate(ctx context.Context) error {
	if err := c.client.Del(ctx, c.hashKey).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCacheInvalidate, err)
	}

	return nil
}
