// Package cache provides a generic, type-safe caching layer using Redis.
//
// This package uses generics to provide compile-time type safety for cached values.
// Cache entries are stored as JSON in Redis with TTL support. Keys can be encoded using
// custom KeyEncoder implementations for different types (strings, integers, UUIDs).
//
// Basic usage:
//
//	type User struct {
//	    ID   int
//	    Name string
//	}
//
//	cache := cache.New[string, User](
//	    redisClient,
//	    "users",           // hash key
//	    24 * time.Hour,    // TTL
//	    cache.NewStringKeyEncoder(),
//	)
//
//	// Set a value
//	user := User{ID: 123, Name: "Alice"}
//	if err := cache.Set(ctx, "user:123", &user); err != nil {
//	    return err
//	}
//
//	// Get a value
//	retrieved, err := cache.Get(ctx, "user:123")
//	if err != nil {
//	    return err
//	}
//
// Custom KeyEncoder implementations can be provided for non-standard key types:
//
//	cache := cache.New[CustomKeyType, MyValue](
//	    redisClient,
//	    "data",
//	    time.Hour,
//	    MyCustomKeyEncoder{},
//	)
//
// The cache layer automatically handles JSON marshaling/unmarshaling and provides
// operations for Set, Get, Delete, and Invalidate (clear all cached values for a hash).
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
	DefaultTTL5m  = 5 * time.Minute
	DefaultTTL10m = 10 * time.Minute
	DefaultTTL30m = 30 * time.Minute
	DefaultTTL1h  = time.Hour
	DefaultTTL24h = 24 * time.Hour
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
		ttl = DefaultTTL5m
	}

	return &Cache[K, V]{
		client:     client,
		ttl:        ttl,
		hashKey:    hashKey,
		keyEncoder: keyEncoder,
	}
}

func (c *Cache[K, V]) Get(ctx context.Context, key K) (*V, error) {
	startedAt := time.Now()

	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		recordOperation(c.hashKey, "get", "error", time.Since(startedAt))

		return nil, err
	}

	data, err := c.client.HGet(ctx, c.hashKey, encodedKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			recordOperation(c.hashKey, "get", "miss", time.Since(startedAt))

			return nil, ErrKeyNotFound
		}

		recordOperation(c.hashKey, "get", "error", time.Since(startedAt))

		return nil, fmt.Errorf("%w: %w", ErrCacheGet, err)
	}

	var value V
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		recordOperation(c.hashKey, "get", "error", time.Since(startedAt))

		return nil, fmt.Errorf("%w: %w", ErrCacheUnmarshal, err)
	}

	recordOperation(c.hashKey, "get", "hit", time.Since(startedAt))

	return &value, nil
}

func (c *Cache[K, V]) Set(ctx context.Context, key K, value *V) error {
	startedAt := time.Now()

	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		recordOperation(c.hashKey, "set", "error", time.Since(startedAt))

		return err
	}

	data, err := json.Marshal(value)
	if err != nil {
		recordOperation(c.hashKey, "set", "error", time.Since(startedAt))

		return fmt.Errorf("%w: %w", ErrCacheMarshal, err)
	}

	if err := c.client.HSet(ctx, c.hashKey, encodedKey, data).Err(); err != nil {
		recordOperation(c.hashKey, "set", "error", time.Since(startedAt))

		return fmt.Errorf("%w: %w", ErrCacheSet, err)
	}

	if err := c.client.Expire(ctx, c.hashKey, c.ttl).Err(); err != nil {
		recordOperation(c.hashKey, "set", "error", time.Since(startedAt))

		return fmt.Errorf("%w: %w", ErrCacheTTL, err)
	}

	recordOperation(c.hashKey, "set", "success", time.Since(startedAt))

	return nil
}

func (c *Cache[K, V]) Delete(ctx context.Context, key K) error {
	startedAt := time.Now()

	encodedKey, err := c.keyEncoder.Encode(key)
	if err != nil {
		recordOperation(c.hashKey, "delete", "error", time.Since(startedAt))

		return err
	}

	if err := c.client.HDel(ctx, c.hashKey, encodedKey).Err(); err != nil {
		recordOperation(c.hashKey, "delete", "error", time.Since(startedAt))

		return fmt.Errorf("%w: %w", ErrCacheDelete, err)
	}

	recordOperation(c.hashKey, "delete", "success", time.Since(startedAt))

	return nil
}

func (c *Cache[K, V]) Invalidate(ctx context.Context) error {
	startedAt := time.Now()

	if err := c.client.Del(ctx, c.hashKey).Err(); err != nil {
		recordOperation(c.hashKey, "invalidate", "error", time.Since(startedAt))

		return fmt.Errorf("%w: %w", ErrCacheInvalidate, err)
	}

	recordOperation(c.hashKey, "invalidate", "success", time.Since(startedAt))

	return nil
}
