package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store executes counter SQL against PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store connected to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// IncrPostCommentCount atomically increments posts.comment_count.
func (s *Store) IncrPostCommentCount(ctx context.Context, postID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE posts SET comment_count = comment_count + 1 WHERE id = $1`,
		postID,
	)
	return err
}

// IncrArticleCommentCount atomically increments articles.comment_count.
func (s *Store) IncrArticleCommentCount(ctx context.Context, articleID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE articles SET comment_count = comment_count + 1 WHERE id = $1`,
		articleID,
	)
	return err
}

// UpdatePostReactionCounts recomputes posts.reaction_counts JSONB by
// aggregating the current post_likes rows for the given post.
//
// This replaces the old ±1 INT approach: instead of tracking a total, we
// store a per-emotion map {"like":5,"love":2,...} that the feed service can
// return directly. Running this on every reaction event is idempotent —
// duplicate Pub/Sub deliveries produce the same result — and the nightly
// reconciliation job remains a no-op for already-correct rows.
func (s *Store) UpdatePostReactionCounts(ctx context.Context, postID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE posts
		SET reaction_counts = (
			SELECT COALESCE(jsonb_object_agg(emotion, cnt), '{}')
			FROM (
				SELECT emotion, COUNT(*) AS cnt
				FROM post_likes
				WHERE post_id = $1
				GROUP BY emotion
			) t
		)
		WHERE id = $1
	`, postID)
	return err
}
