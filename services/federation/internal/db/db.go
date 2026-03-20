// Package db wraps the federation service's Postgres connection pool and
// provides typed access to the federation schema tables.
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgxpool for the federation schema.
type Pool struct {
	pool *pgxpool.Pool
}

// New creates and pings a connection pool for the federation database.
func New(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Pool{pool: pool}, nil
}

// Close releases the connection pool.
func (p *Pool) Close() {
	p.pool.Close()
}

// ─── ActorKey ────────────────────────────────────────────────────────────────

// ActorKey is one row from the actor_keys table.
type ActorKey struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Username      string
	PublicKeyPem  string
	PrivateKeyEnc []byte
	CreatedAt     time.Time
}

// EnsureActorKey inserts a new actor key row if none exists for userID.
// It is idempotent: concurrent inserts on the same user_id are silently
// ignored via ON CONFLICT DO NOTHING.
func (p *Pool) EnsureActorKey(ctx context.Context, userID uuid.UUID, username, pubPEM string, privEnc []byte) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO actor_keys (user_id, username, public_key_pem, private_key_enc)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, username, pubPEM, privEnc)
	return err
}

// GetActorKeyByUsername fetches the actor key for a given username.
// Returns nil, nil if no row exists.
func (p *Pool) GetActorKeyByUsername(ctx context.Context, username string) (*ActorKey, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, user_id, username, public_key_pem, private_key_enc, created_at
		FROM actor_keys
		WHERE username = $1
	`, username)

	var k ActorKey
	err := row.Scan(&k.ID, &k.UserID, &k.Username, &k.PublicKeyPem, &k.PrivateKeyEnc, &k.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get actor key: %w", err)
	}
	return &k, nil
}

// ─── RemoteFollower ───────────────────────────────────────────────────────────

// RemoteFollower is one row from the remote_followers table.
type RemoteFollower struct {
	ID            uuid.UUID
	LocalUsername string
	ActorURL      string
	InboxURL      string
	CreatedAt     time.Time
}

// AddRemoteFollower records that remoteActorURL follows localUsername.
// Idempotent via ON CONFLICT DO NOTHING.
func (p *Pool) AddRemoteFollower(ctx context.Context, localUsername, actorURL, inboxURL string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO remote_followers (local_username, actor_url, inbox_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (local_username, actor_url) DO UPDATE SET inbox_url = EXCLUDED.inbox_url
	`, localUsername, actorURL, inboxURL)
	return err
}

// RemoveRemoteFollower deletes the follower record for remoteActorURL.
func (p *Pool) RemoveRemoteFollower(ctx context.Context, localUsername, actorURL string) error {
	_, err := p.pool.Exec(ctx, `
		DELETE FROM remote_followers WHERE local_username = $1 AND actor_url = $2
	`, localUsername, actorURL)
	return err
}

// ListRemoteFollowers returns all remote follower inbox URLs for a local user.
func (p *Pool) ListRemoteFollowers(ctx context.Context, localUsername string) ([]RemoteFollower, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, local_username, actor_url, inbox_url, created_at
		FROM remote_followers
		WHERE local_username = $1
		ORDER BY created_at ASC
	`, localUsername)
	if err != nil {
		return nil, fmt.Errorf("list remote followers: %w", err)
	}
	defer rows.Close()

	var out []RemoteFollower
	for rows.Next() {
		var f RemoteFollower
		if err := rows.Scan(&f.ID, &f.LocalUsername, &f.ActorURL, &f.InboxURL, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan remote follower: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// HasRemoteFollowers returns true if the local user has at least one remote follower.
func (p *Pool) HasRemoteFollowers(ctx context.Context, localUsername string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM remote_followers WHERE local_username = $1)
	`, localUsername).Scan(&exists)
	return exists, err
}

// ─── DeliveryQueue ────────────────────────────────────────────────────────────

// DeliveryItem is one row from the delivery_queue table.
type DeliveryItem struct {
	ID             uuid.UUID
	LocalUsername  string
	TargetInbox    string
	ActivityJSON   map[string]any
	Attempts       int
	NextAttemptAt  time.Time
	LastError      *string
	Status         string
	CreatedAt      time.Time
}

// EnqueueDelivery inserts a new pending delivery for the given activity.
func (p *Pool) EnqueueDelivery(ctx context.Context, localUsername, targetInbox string, activity map[string]any) error {
	raw, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO delivery_queue (local_username, target_inbox, activity_json)
		VALUES ($1, $2, $3)
	`, localUsername, targetInbox, raw)
	return err
}

// PollPendingDeliveries fetches up to limit pending items whose next_attempt_at
// is <= now, and atomically marks them as in-flight by bumping attempts.
func (p *Pool) PollPendingDeliveries(ctx context.Context, limit int) ([]DeliveryItem, error) {
	rows, err := p.pool.Query(ctx, `
		UPDATE delivery_queue
		SET attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM delivery_queue
			WHERE status = 'pending' AND next_attempt_at <= now()
			ORDER BY next_attempt_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, local_username, target_inbox, activity_json,
		          attempts, next_attempt_at, last_error, status, created_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("poll pending deliveries: %w", err)
	}
	defer rows.Close()

	return scanDeliveryRows(rows)
}

// MarkDeliveryDone marks a delivery as successfully completed.
func (p *Pool) MarkDeliveryDone(ctx context.Context, id uuid.UUID) error {
	_, err := p.pool.Exec(ctx, `UPDATE delivery_queue SET status = 'done' WHERE id = $1`, id)
	return err
}

// MarkDeliveryRetry schedules a retry with exponential back-off.
// After maxAttempts the status is set to 'failed'.
func (p *Pool) MarkDeliveryRetry(ctx context.Context, id uuid.UUID, attempts int, errMsg string) error {
	const maxAttempts = 5
	if attempts >= maxAttempts {
		_, err := p.pool.Exec(ctx, `
			UPDATE delivery_queue
			SET status = 'failed', last_error = $2
			WHERE id = $1
		`, id, errMsg)
		return err
	}
	// Exponential back-off: 1m, 5m, 30m, 2h, 12h
	backoffs := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		12 * time.Hour,
	}
	delay := backoffs[attempts-1]
	next := time.Now().Add(delay)
	_, err := p.pool.Exec(ctx, `
		UPDATE delivery_queue
		SET status = 'pending', next_attempt_at = $2, last_error = $3
		WHERE id = $1
	`, id, next, errMsg)
	return err
}

// ─── RemoteFollowing ──────────────────────────────────────────────────────────

// RemoteFollowing is one row from the remote_following table.
type RemoteFollowing struct {
	ID               uuid.UUID
	LocalUsername    string
	ActorURL         string
	InboxURL         string
	FollowActivityID string
	Accepted         bool
	CreatedAt        time.Time
}

// AddRemoteFollowing records that localUsername is following actorURL.
// Idempotent via ON CONFLICT DO UPDATE so re-following refreshes the activity ID.
func (p *Pool) AddRemoteFollowing(ctx context.Context, localUsername, actorURL, inboxURL, followActivityID string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO remote_following (local_username, actor_url, inbox_url, follow_activity_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (local_username, actor_url) DO UPDATE
		  SET inbox_url = EXCLUDED.inbox_url,
		      follow_activity_id = EXCLUDED.follow_activity_id,
		      accepted = false
	`, localUsername, actorURL, inboxURL, followActivityID)
	return err
}

// RemoveRemoteFollowing deletes the following record.
func (p *Pool) RemoveRemoteFollowing(ctx context.Context, localUsername, actorURL string) error {
	_, err := p.pool.Exec(ctx, `
		DELETE FROM remote_following WHERE local_username = $1 AND actor_url = $2
	`, localUsername, actorURL)
	return err
}

// MarkFollowingAccepted sets accepted=true for a remote follow.
func (p *Pool) MarkFollowingAccepted(ctx context.Context, localUsername, actorURL string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE remote_following SET accepted = true WHERE local_username = $1 AND actor_url = $2
	`, localUsername, actorURL)
	return err
}

// GetRemoteFollowing fetches a single following record. Returns nil, nil if not found.
func (p *Pool) GetRemoteFollowing(ctx context.Context, localUsername, actorURL string) (*RemoteFollowing, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, local_username, actor_url, inbox_url, follow_activity_id, accepted, created_at
		FROM remote_following
		WHERE local_username = $1 AND actor_url = $2
	`, localUsername, actorURL)
	var f RemoteFollowing
	err := row.Scan(&f.ID, &f.LocalUsername, &f.ActorURL, &f.InboxURL, &f.FollowActivityID, &f.Accepted, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get remote following: %w", err)
	}
	return &f, nil
}

// ListRemoteFollowing returns all remote actors that localUsername follows.
func (p *Pool) ListRemoteFollowing(ctx context.Context, localUsername string) ([]RemoteFollowing, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, local_username, actor_url, inbox_url, follow_activity_id, accepted, created_at
		FROM remote_following
		WHERE local_username = $1
		ORDER BY created_at DESC
	`, localUsername)
	if err != nil {
		return nil, fmt.Errorf("list remote following: %w", err)
	}
	defer rows.Close()
	var out []RemoteFollowing
	for rows.Next() {
		var f RemoteFollowing
		if err := rows.Scan(&f.ID, &f.LocalUsername, &f.ActorURL, &f.InboxURL, &f.FollowActivityID, &f.Accepted, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan remote following: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ─── RemotePost ───────────────────────────────────────────────────────────────

// RemotePost is one row from the remote_posts table.
type RemotePost struct {
	ID             uuid.UUID
	ActivityID     string
	ActorURL       string
	LocalRecipient string
	Content        string
	PublishedAt    time.Time
	RawActivity    map[string]any
	CreatedAt      time.Time
}

// UpsertRemotePost stores an incoming federated post. Idempotent on activity_id.
func (p *Pool) UpsertRemotePost(ctx context.Context, activityID, actorURL, localRecipient, content string, publishedAt time.Time, rawActivity map[string]any) error {
	raw, err := json.Marshal(rawActivity)
	if err != nil {
		return fmt.Errorf("marshal raw activity: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO remote_posts (activity_id, actor_url, local_recipient, content, published_at, raw_activity)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (activity_id) DO NOTHING
	`, activityID, actorURL, localRecipient, content, publishedAt, raw)
	return err
}

// ListRemotePosts returns remote posts for a local recipient, newest first.
// If before is set, returns only posts older than that timestamp.
func (p *Pool) ListRemotePosts(ctx context.Context, localRecipient string, limit int, before *time.Time) ([]RemotePost, error) {
	var rows pgx.Rows
	var err error
	if before != nil {
		rows, err = p.pool.Query(ctx, `
			SELECT id, activity_id, actor_url, local_recipient, content, published_at, raw_activity, created_at
			FROM remote_posts
			WHERE local_recipient = $1 AND published_at < $2
			ORDER BY published_at DESC
			LIMIT $3
		`, localRecipient, before, limit)
	} else {
		rows, err = p.pool.Query(ctx, `
			SELECT id, activity_id, actor_url, local_recipient, content, published_at, raw_activity, created_at
			FROM remote_posts
			WHERE local_recipient = $1
			ORDER BY published_at DESC
			LIMIT $2
		`, localRecipient, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list remote posts: %w", err)
	}
	defer rows.Close()
	var out []RemotePost
	for rows.Next() {
		var rp RemotePost
		var rawJSON []byte
		if err := rows.Scan(&rp.ID, &rp.ActivityID, &rp.ActorURL, &rp.LocalRecipient, &rp.Content, &rp.PublishedAt, &rawJSON, &rp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan remote post: %w", err)
		}
		if err := json.Unmarshal(rawJSON, &rp.RawActivity); err != nil {
			return nil, fmt.Errorf("unmarshal raw activity: %w", err)
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}

func scanDeliveryRows(rows pgx.Rows) ([]DeliveryItem, error) {
	var items []DeliveryItem
	for rows.Next() {
		var item DeliveryItem
		var rawJSON []byte
		err := rows.Scan(
			&item.ID, &item.LocalUsername, &item.TargetInbox, &rawJSON,
			&item.Attempts, &item.NextAttemptAt, &item.LastError, &item.Status, &item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan delivery item: %w", err)
		}
		if err := json.Unmarshal(rawJSON, &item.ActivityJSON); err != nil {
			return nil, fmt.Errorf("unmarshal activity: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
