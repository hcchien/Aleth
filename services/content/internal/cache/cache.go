// Package cache provides a thin Redis wrapper used by the content service.
// All methods are safe to call on a nil *Client — they become no-ops,
// which lets the service run without Redis in development.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis. A nil pointer silently disables caching.
type Client struct {
	rdb *redis.Client
}

// New connects to Redis at the given URL (e.g. "redis://localhost:6379/0").
// Returns (nil, nil) when redisURL is empty — caching is then disabled.
func New(ctx context.Context, redisURL string) (*Client, error) {
	if redisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

// Close releases the Redis connection pool.
func (c *Client) Close() {
	if c == nil {
		return
	}
	_ = c.rdb.Close()
}

// Get fetches and JSON-unmarshals a cached value into dst.
// Returns (false, nil) on cache miss so callers can fall through to DB.
func (c *Client) Get(ctx context.Context, key string, dst any) (bool, error) {
	if c == nil {
		return false, nil
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dst)
}

// Set JSON-marshals src and stores it under key with the given TTL.
func (c *Client) Set(ctx context.Context, key string, src any, ttl time.Duration) error {
	if c == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// Del removes one or more keys from Redis.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if c == nil {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}
