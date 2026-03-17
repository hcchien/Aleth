package events

import (
	"context"
	"encoding/json"
	"time"
)

// Publisher publishes domain events after a successful write.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// Event is a domain event envelope.
type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// Event type constants.
const (
	TypePostCreated      = "post.created"
	TypeCommentCreated   = "comment.created"
	TypeReactionUpserted = "reaction.upserted"
	TypeReactionRemoved  = "reaction.removed"
)

// PostCreatedPayload is the payload for TypePostCreated events.
type PostCreatedPayload struct {
	PostID         string  `json:"post_id"`
	AuthorID       string  `json:"author_id"`
	Kind           string  `json:"kind"` // "post" | "note" | "reply" | "reshare"
	ParentID       *string `json:"parent_id,omitempty"`
	ParentAuthorID *string `json:"parent_author_id,omitempty"` // set for reply and reshare
	PageID         *string `json:"page_id,omitempty"`          // set when this post belongs to a fan page
}

// CommentCreatedPayload is the payload for TypeCommentCreated events.
type CommentCreatedPayload struct {
	CommentID       string  `json:"comment_id"`
	ArticleID       string  `json:"article_id"`
	AuthorID        string  `json:"author_id"`
	ArticleAuthorID string  `json:"article_author_id"` // always set — the article's author to notify
	ParentID        *string `json:"parent_id,omitempty"`
	ParentAuthorID  *string `json:"parent_author_id,omitempty"` // set when this is a reply to another comment
}

// ReactionUpsertedPayload is the payload for TypeReactionUpserted events.
type ReactionUpsertedPayload struct {
	PostID  string `json:"post_id"`
	UserID  string `json:"user_id"`
	Emotion string `json:"emotion"`
}

// ReactionRemovedPayload is the payload for TypeReactionRemoved events.
type ReactionRemovedPayload struct {
	PostID string `json:"post_id"`
	UserID string `json:"user_id"`
}
