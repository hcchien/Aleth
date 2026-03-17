// Package contentdb provides a read-only client for the content service
// database. The notification service uses it to look up page followers for
// fan-out notifications when a page publishes new content.
package contentdb

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is a thin read-only wrapper around the content service's Postgres pool.
type Pool struct {
	pool *pgxpool.Pool
}

// New opens a connection pool to the content database.
func New(ctx context.Context, dsn string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create content db pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping content db: %w", err)
	}
	return &Pool{pool: pool}, nil
}

// Close releases the pool's connections.
func (p *Pool) Close() {
	p.pool.Close()
}

// ListPageFollowers returns the user IDs of all followers of the given page.
// Returns an empty (non-nil) slice when the page has no followers.
func (p *Pool) ListPageFollowers(ctx context.Context, pageID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT user_id FROM page_followers WHERE page_id = $1`,
		pageID,
	)
	if err != nil {
		return nil, fmt.Errorf("list page followers: %w", err)
	}
	defer rows.Close()

	out := []uuid.UUID{}
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		out = append(out, userID)
	}
	return out, rows.Err()
}
