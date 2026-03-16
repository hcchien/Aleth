// Package service handles counter updates driven by Pub/Sub domain events.
//
// # Design
//
// Posts carry denormalized counters (comment_count, reaction_counts) so the
// feed query can read a single row per post instead of doing COUNT(*) joins.
//
// These counters are intentionally eventually consistent: the Counter Service
// subscribes to domain events published by the Content Service and updates
// atomically via SQL UPDATE. A short propagation lag (typically < 1 second
// under normal load) is acceptable for display purposes.
//
// # Counter mapping
//
//   - post.created  (kind=reply)   → posts.comment_count  += 1  on parent_id
//   - comment.created              → articles.comment_count += 1 on article_id
//   - reaction.upserted            → posts.reaction_counts recomputed from post_likes
//   - reaction.removed             → posts.reaction_counts recomputed from post_likes
//
// # Idempotency
//
// Pub/Sub guarantees at-least-once delivery. The comment counter operations
// are atomic ±1 increments (a duplicate causes at most ±1 drift corrected by
// nightly reconciliation). The reaction_counts recompute is fully idempotent:
// running it twice on the same post produces the same result.
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// store is the subset of DB operations the counter service needs.
type store interface {
	IncrPostCommentCount(ctx context.Context, postID uuid.UUID) error
	IncrArticleCommentCount(ctx context.Context, articleID uuid.UUID) error
	UpdatePostReactionCounts(ctx context.Context, postID uuid.UUID) error
}

// CounterService updates denormalized counters in response to domain events.
type CounterService struct {
	db store
}

// New creates a CounterService backed by the given store.
func New(db store) *CounterService {
	return &CounterService{db: db}
}

// envelope matches the outer structure published by the Content Service.
type envelope struct {
	Payload json.RawMessage `json:"payload"`
}

// HandleEvent dispatches a Pub/Sub message to the appropriate counter handler.
// Unknown event types are silently ignored.
func (s *CounterService) HandleEvent(ctx context.Context, eventType string, data []byte) {
	var err error
	switch eventType {
	case "post.created":
		err = s.handlePostCreated(ctx, data)
	case "comment.created":
		err = s.handleCommentCreated(ctx, data)
	case "reaction.upserted", "reaction.removed":
		err = s.handleReactionEvent(ctx, data)
	default:
		return
	}
	if err != nil {
		log.Error().Err(err).Str("event_type", eventType).Msg("counter update failed")
	}
}

// ─── event payloads ───────────────────────────────────────────────────────────

type postCreatedPayload struct {
	Kind     string  `json:"kind"`
	ParentID *string `json:"parent_id,omitempty"`
}

type commentCreatedPayload struct {
	ArticleID string `json:"article_id"`
}

type reactionPayload struct {
	PostID string `json:"post_id"`
}

// ─── handlers ─────────────────────────────────────────────────────────────────

func (s *CounterService) handlePostCreated(ctx context.Context, data []byte) error {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	var p postCreatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Only replies increment a parent post's comment counter.
	if p.Kind != "reply" || p.ParentID == nil {
		return nil
	}

	parentID, err := uuid.Parse(*p.ParentID)
	if err != nil {
		return fmt.Errorf("parse parent_id: %w", err)
	}

	return s.db.IncrPostCommentCount(ctx, parentID)
}

func (s *CounterService) handleCommentCreated(ctx context.Context, data []byte) error {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	var p commentCreatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	articleID, err := uuid.Parse(p.ArticleID)
	if err != nil {
		return fmt.Errorf("parse article_id: %w", err)
	}

	return s.db.IncrArticleCommentCount(ctx, articleID)
}

// handleReactionEvent handles both reaction.upserted and reaction.removed.
// Instead of ±1 INT operations, it recomputes the full per-emotion JSONB
// breakdown from post_likes. This is idempotent and always produces a
// consistent result regardless of delivery order or duplicates.
func (s *CounterService) handleReactionEvent(ctx context.Context, data []byte) error {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	var p reactionPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	postID, err := uuid.Parse(p.PostID)
	if err != nil {
		return fmt.Errorf("parse post_id: %w", err)
	}

	return s.db.UpdatePostReactionCounts(ctx, postID)
}
