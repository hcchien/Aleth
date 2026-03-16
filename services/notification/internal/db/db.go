package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification represents a single notification row.
type Notification struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Type       string    `json:"type"` // 'reply', 'reshare', 'comment', 'reaction'
	ActorID    uuid.UUID `json:"actor_id"`
	EntityType string    `json:"entity_type"` // 'post', 'comment'
	EntityID   uuid.UUID `json:"entity_id"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

// Store wraps a pgxpool and provides notification queries.
type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// CreateNotificationParams are the fields needed to insert a notification.
type CreateNotificationParams struct {
	UserID     uuid.UUID
	Type       string
	ActorID    uuid.UUID
	EntityType string
	EntityID   uuid.UUID
}

// CreateNotification inserts a single notification row.
func (s *Store) CreateNotification(ctx context.Context, p CreateNotificationParams) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, actor_id, entity_type, entity_id)
		VALUES ($1, $2, $3, $4, $5)
	`, p.UserID, p.Type, p.ActorID, p.EntityType, p.EntityID)
	return err
}

// CountUnread returns the number of unread notifications for a user.
func (s *Store) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

// ListNotifications returns the most recent notifications for a user, newest first.
func (s *Store) ListNotifications(ctx context.Context, userID uuid.UUID, limit int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, type, actor_id, entity_type, entity_id, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.EntityType, &n.EntityID, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkAllRead marks all unread notifications for a user as read.
func (s *Store) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`,
		userID,
	)
	return err
}

// MarkRead marks specific notifications as read.
func (s *Store) MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND id = ANY($2)`,
		userID, ids,
	)
	return err
}
