// Package limiter provides a Redis-backed per-account login attempt counter
// to protect against brute-force and credential-stuffing attacks.
//
// Key schema:   login_fail:{email}
// TTL:          15 minutes (resets on every failure; cleared on success)
// Threshold:    10 failures → account is locked until TTL expires
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	maxAttempts = 10
	window      = 15 * time.Minute
)

// Limiter gates login attempts for a given email address.
type Limiter interface {
	// IsLocked returns true if the account has exceeded the failure threshold.
	IsLocked(ctx context.Context, email string) (bool, error)
	// RecordFailure increments the failure counter and resets its TTL.
	// The counter persists for [window] after the last failure.
	RecordFailure(ctx context.Context, email string) error
	// Reset clears the failure counter on a successful login.
	Reset(ctx context.Context, email string) error
	// CheckAndIncrement returns true (= blocked) if the key has exceeded maxAttempts within the window.
	CheckAndIncrement(ctx context.Context, key string, maxAttempts int, window time.Duration) (bool, error)
}

// ─── Redis implementation ─────────────────────────────────────────────────────

type redisLimiter struct {
	rdb *redis.Client
}

func key(email string) string { return "login_fail:" + email }

// New returns a Limiter backed by Redis at redisURL.
// If redisURL is empty, a no-op limiter is returned so the auth service starts
// normally without Redis (suitable for local development).
func New(ctx context.Context, redisURL string) Limiter {
	if redisURL == "" {
		log.Info().Msg("login limiter disabled (AUTH_REDIS_URL not set)")
		return nopLimiter{}
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Warn().Err(err).Msg("invalid AUTH_REDIS_URL — login limiter disabled")
		return nopLimiter{}
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("cannot reach Redis — login limiter disabled")
		return nopLimiter{}
	}
	log.Info().Str("url", redisURL).Msg("login limiter connected to Redis")
	return &redisLimiter{rdb: rdb}
}

func (l *redisLimiter) IsLocked(ctx context.Context, email string) (bool, error) {
	val, err := l.rdb.Get(ctx, key(email)).Int()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("limiter get: %w", err)
	}
	return val >= maxAttempts, nil
}

func (l *redisLimiter) RecordFailure(ctx context.Context, email string) error {
	k := key(email)
	pipe := l.rdb.Pipeline()
	pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, window)
	_, err := pipe.Exec(ctx)
	return err
}

func (l *redisLimiter) Reset(ctx context.Context, email string) error {
	return l.rdb.Del(ctx, key(email)).Err()
}

func (r *redisLimiter) CheckAndIncrement(ctx context.Context, key string, maxAttempts int, window time.Duration) (bool, error) {
	pipe := r.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() > int64(maxAttempts), nil
}

// ─── No-op implementation ─────────────────────────────────────────────────────

// nopLimiter allows the auth service to run without Redis.
// All accounts are always unlocked and no counters are tracked.
type nopLimiter struct{}

func (nopLimiter) IsLocked(_ context.Context, _ string) (bool, error) { return false, nil }
func (nopLimiter) RecordFailure(_ context.Context, _ string) error     { return nil }
func (nopLimiter) Reset(_ context.Context, _ string) error             { return nil }
func (n nopLimiter) CheckAndIncrement(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return false, nil
}
