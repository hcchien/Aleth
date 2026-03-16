// Package service contains the notification business logic and the Pub/Sub
// event consumer that translates content events into notification rows.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/aleth/notification/internal/db"
)

// store is the subset of db.Store used by the service, enabling test fakes.
type store interface {
	CreateNotification(ctx context.Context, p db.CreateNotificationParams) error
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	ListNotifications(ctx context.Context, userID uuid.UUID, limit int) ([]db.Notification, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
}

// NotificationService handles notification creation and retrieval.
type NotificationService struct {
	db        store
	readQueue *ReadQueue
}

func NewNotificationService(db store) *NotificationService {
	return &NotificationService{db: db, readQueue: NewReadQueue(512)}
}

// ReadQueue is a bounded in-process async queue for mark-read operations.
// The HTTP handler enqueues jobs and returns 202 immediately; a background
// worker drains the queue and writes to the DB.
type ReadQueue struct {
	ch chan markReadJob
}

type markReadJob struct {
	userID uuid.UUID
	ids    []uuid.UUID // nil = mark all
}

// NewReadQueue creates a ReadQueue with the given buffer capacity.
func NewReadQueue(capacity int) *ReadQueue {
	return &ReadQueue{ch: make(chan markReadJob, capacity)}
}

// Enqueue adds a mark-read job. Returns false and logs a warning if the queue
// is full — the write will be dropped rather than blocking the HTTP handler.
func (q *ReadQueue) Enqueue(userID uuid.UUID, ids []uuid.UUID) bool {
	select {
	case q.ch <- markReadJob{userID: userID, ids: ids}:
		return true
	default:
		log.Warn().Str("user_id", userID.String()).Msg("read queue full: mark-read job dropped")
		return false
	}
}

// Run processes jobs from the queue until ctx is cancelled, then drains any
// remaining jobs with a short deadline before returning.
func (q *ReadQueue) Run(ctx context.Context, db store) {
	for {
		select {
		case <-ctx.Done():
			q.drain(db)
			return
		case job := <-q.ch:
			q.process(ctx, db, job)
		}
	}
}

func (q *ReadQueue) drain(db store) {
	drainCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case job := <-q.ch:
			q.process(drainCtx, db, job)
		default:
			return
		}
	}
}

func (q *ReadQueue) process(ctx context.Context, db store, job markReadJob) {
	var err error
	if len(job.ids) == 0 {
		err = db.MarkAllRead(ctx, job.userID)
	} else {
		err = db.MarkRead(ctx, job.userID, job.ids)
	}
	if err != nil {
		log.Error().Err(err).Str("user_id", job.userID.String()).Msg("async mark-read failed")
	}
}

// ─── Event payload shapes (mirror services/content/internal/events) ───────────

type postCreatedPayload struct {
	PostID         string  `json:"post_id"`
	AuthorID       string  `json:"author_id"`
	Kind           string  `json:"kind"`
	ParentID       *string `json:"parent_id,omitempty"`
	ParentAuthorID *string `json:"parent_author_id,omitempty"`
}

type commentCreatedPayload struct {
	CommentID       string  `json:"comment_id"`
	ArticleID       string  `json:"article_id"`
	AuthorID        string  `json:"author_id"`
	ArticleAuthorID string  `json:"article_author_id"`
	ParentID        *string `json:"parent_id,omitempty"`
	ParentAuthorID  *string `json:"parent_author_id,omitempty"`
}

type reactionUpsertedPayload struct {
	PostID  string `json:"post_id"`
	UserID  string `json:"user_id"`
	Emotion string `json:"emotion"`
}

// ─── Pub/Sub event consumer ───────────────────────────────────────────────────

// HandleEvent processes a raw content-events Pub/Sub message.
// eventType is the value of the "event_type" Pub/Sub attribute.
func (s *NotificationService) HandleEvent(ctx context.Context, eventType string, data []byte) {
	var err error
	switch eventType {
	case "post.created":
		err = s.handlePostCreated(ctx, data)
	case "comment.created":
		err = s.handleCommentCreated(ctx, data)
	case "reaction.upserted":
		err = s.handleReactionUpserted(ctx, data)
	default:
		// reaction.removed and unknown types need no notification
		return
	}
	if err != nil {
		log.Error().Err(err).Str("event_type", eventType).Msg("handle notification event")
	}
}

func (s *NotificationService) handlePostCreated(ctx context.Context, data []byte) error {
	// data is the full Event envelope; payload is nested
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var p postCreatedPayload
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal post.created payload: %w", err)
	}

	actorID, err := uuid.Parse(p.AuthorID)
	if err != nil {
		return fmt.Errorf("parse author_id: %w", err)
	}
	entityID, err := uuid.Parse(p.PostID)
	if err != nil {
		return fmt.Errorf("parse post_id: %w", err)
	}

	// Notify the parent author on reply or reshare, but never notify yourself.
	if p.ParentAuthorID != nil && *p.ParentAuthorID != p.AuthorID {
		recipientID, err := uuid.Parse(*p.ParentAuthorID)
		if err != nil {
			return fmt.Errorf("parse parent_author_id: %w", err)
		}
		notifType := p.Kind // 'reply' or 'reshare'
		if err := s.db.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:     recipientID,
			Type:       notifType,
			ActorID:    actorID,
			EntityType: "post",
			EntityID:   entityID,
		}); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
	}
	return nil
}

func (s *NotificationService) handleCommentCreated(ctx context.Context, data []byte) error {
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var p commentCreatedPayload
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal comment.created payload: %w", err)
	}

	actorID, err := uuid.Parse(p.AuthorID)
	if err != nil {
		return fmt.Errorf("parse author_id: %w", err)
	}
	entityID, err := uuid.Parse(p.CommentID)
	if err != nil {
		return fmt.Errorf("parse comment_id: %w", err)
	}

	// Always notify the article author (unless commenting on own article).
	if p.ArticleAuthorID != "" && p.ArticleAuthorID != p.AuthorID {
		articleAuthorID, err := uuid.Parse(p.ArticleAuthorID)
		if err != nil {
			return fmt.Errorf("parse article_author_id: %w", err)
		}
		if err := s.db.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:     articleAuthorID,
			Type:       "comment",
			ActorID:    actorID,
			EntityType: "comment",
			EntityID:   entityID,
		}); err != nil {
			return fmt.Errorf("create article author notification: %w", err)
		}
	}

	// Also notify parent comment author when this is a threaded reply.
	if p.ParentAuthorID != nil && *p.ParentAuthorID != p.AuthorID && *p.ParentAuthorID != p.ArticleAuthorID {
		parentAuthorID, err := uuid.Parse(*p.ParentAuthorID)
		if err != nil {
			return fmt.Errorf("parse parent_author_id: %w", err)
		}
		if err := s.db.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:     parentAuthorID,
			Type:       "reply",
			ActorID:    actorID,
			EntityType: "comment",
			EntityID:   entityID,
		}); err != nil {
			return fmt.Errorf("create parent comment notification: %w", err)
		}
	}
	return nil
}

func (s *NotificationService) handleReactionUpserted(ctx context.Context, data []byte) error {
	// Reactions are high-volume; for now we skip individual reaction notifications
	// to avoid flooding. A future version could batch or deduplicate these.
	_ = data
	return nil
}

// ─── Query API ────────────────────────────────────────────────────────────────

func (s *NotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.db.CountUnread(ctx, userID)
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, limit int) ([]db.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.ListNotifications(ctx, userID, limit)
}

// EnqueueMarkAllRead schedules marking all of a user's unread notifications as
// read. The DB write happens asynchronously — callers should return 202 rather
// than 204 to reflect that the operation is accepted, not yet completed.
func (s *NotificationService) EnqueueMarkAllRead(userID uuid.UUID) {
	s.readQueue.Enqueue(userID, nil)
}

// EnqueueMarkRead schedules marking specific notifications as read.
func (s *NotificationService) EnqueueMarkRead(userID uuid.UUID, ids []uuid.UUID) {
	s.readQueue.Enqueue(userID, ids)
}

// StartReadWorker starts the background goroutine that drains the read queue.
// It blocks until ctx is cancelled and all queued jobs are flushed.
// Call it in a separate goroutine from main.
func (s *NotificationService) StartReadWorker(ctx context.Context) {
	s.readQueue.Run(ctx, s.db)
}
