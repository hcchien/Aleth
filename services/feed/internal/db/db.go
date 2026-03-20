package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Auth DB ──────────────────────────────────────────────────────────────────

// AuthUser is a minimal user record from the auth DB.
type AuthUser struct {
	ID          uuid.UUID
	Username    string
	DisplayName *string
	TrustLevel  int32
}

// AuthStore queries the auth database (user_follows, users).
type AuthStore struct {
	pool *pgxpool.Pool
}

func NewAuthStore(pool *pgxpool.Pool) *AuthStore { return &AuthStore{pool: pool} }

// GetFolloweeIDs returns the IDs of users that followerID is following.
func (s *AuthStore) GetFolloweeIDs(ctx context.Context, followerID uuid.UUID) ([]uuid.UUID, error) {
	const q = `SELECT followee_id FROM user_follows WHERE follower_id = $1`
	rows, err := s.pool.Query(ctx, q, followerID)
	if err != nil {
		return nil, fmt.Errorf("get followee ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan followee id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetUsersByIDs returns basic user info for a batch of IDs.
func (s *AuthStore) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]AuthUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const q = `SELECT id, username, display_name, trust_level FROM users WHERE id = ANY($1)`
	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}
	defer rows.Close()

	var users []AuthUser
	for rows.Next() {
		var u AuthUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.TrustLevel); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ─── Content DB ───────────────────────────────────────────────────────────────

// FeedPost is a post as returned by feed queries.
type FeedPost struct {
	ID             uuid.UUID
	AuthorID       uuid.UUID
	Content        string
	Kind           string
	NoteTitle      *string
	NoteSummary    *string
	ResharedFromID *uuid.UUID       // nil if this is an original post
	CommentCount   int32
	ReactionCounts map[string]int32 // per-emotion counts {"like":5,"love":2,...}
	MyEmotion      *string          // viewer's reaction; nil if not reacted
	CreatedAt      time.Time
	AuthorTrustLevel int32          // trust level from auth DB
	IsSigned       bool             // true if post has a non-nil signature
}

// ContentStore queries the content database (posts, post_likes).
type ContentStore struct {
	pool *pgxpool.Pool
}

func NewContentStore(pool *pgxpool.Pool) *ContentStore { return &ContentStore{pool: pool} }

// GetFollowedPageIDs returns the IDs of fan pages that viewerID follows.
func (s *ContentStore) GetFollowedPageIDs(ctx context.Context, viewerID uuid.UUID) ([]uuid.UUID, error) {
	const q = `SELECT page_id FROM page_followers WHERE user_id = $1`
	rows, err := s.pool.Query(ctx, q, viewerID)
	if err != nil {
		return nil, fmt.Errorf("get followed page ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan page id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FeedPostsParams controls a personalized feed query.
type FeedPostsParams struct {
	FolloweeIDs     []uuid.UUID // authors to include (should include viewer's own ID)
	FollowedPageIDs []uuid.UUID // fan pages the viewer follows
	ViewerID        *uuid.UUID  // for my_emotion join; nil for anonymous
	Cursor          *uuid.UUID  // last seen post ID
	Limit           int
}

// ListFeedPosts returns posts from the given followee IDs and followed page IDs,
// newest first.
func (s *ContentStore) ListFeedPosts(ctx context.Context, p FeedPostsParams) ([]FeedPost, error) {
	if len(p.FolloweeIDs) == 0 && len(p.FollowedPageIDs) == 0 {
		return nil, nil
	}
	const q = `
		SELECT p.id, p.author_id, p.content, p.kind,
		       p.note_title, p.note_summary, p.reshared_from_id,
		       (SELECT COUNT(*) FROM posts c WHERE c.parent_id = p.id AND c.deleted_at IS NULL)::int AS comment_count,
		       p.reaction_counts,
		       pl.emotion AS my_emotion,
		       p.created_at,
		       (p.signature IS NOT NULL) AS is_signed
		FROM posts p
		LEFT JOIN post_likes pl
		       ON pl.post_id = p.id AND pl.user_id = $1
		WHERE p.deleted_at IS NULL
		  AND p.parent_id IS NULL
		  AND (p.author_id = ANY($2) OR (p.page_id IS NOT NULL AND p.page_id = ANY($5)))
		  AND ($3::uuid IS NULL OR p.created_at < (SELECT created_at FROM posts WHERE id = $3))
		ORDER BY p.created_at DESC
		LIMIT $4
	`
	rows, err := s.pool.Query(ctx, q, p.ViewerID, p.FolloweeIDs, p.Cursor, p.Limit, p.FollowedPageIDs)
	if err != nil {
		return nil, fmt.Errorf("list feed posts: %w", err)
	}
	defer rows.Close()
	return collectFeedPosts(rows)
}

// ExplorePostsParams controls an explore feed query.
// Cursor-based pagination is not supported for the explore feed because
// the ranking score changes continuously as posts age.
type ExplorePostsParams struct {
	ViewerID *uuid.UUID // for my_emotion join; nil for anonymous
	Limit    int
}

// ListExplorePosts returns public top-level posts ranked by trust-weighted,
// time-decayed reach score. Uses a Hacker News–style gravity formula:
//
//	score = (reach_score + 1) / ((age_hours + 2) ^ 1.5)
//
// Cursor-based pagination is not used for explore because the ranking changes
// continuously as posts age; always returns the top-N posts by score.
func (s *ContentStore) ListExplorePosts(ctx context.Context, p ExplorePostsParams) ([]FeedPost, error) {
	const q = `
		SELECT p.id, p.author_id, p.content, p.kind,
		       p.note_title, p.note_summary, p.reshared_from_id,
		       (SELECT COUNT(*) FROM posts c WHERE c.parent_id = p.id AND c.deleted_at IS NULL)::int AS comment_count,
		       p.reaction_counts,
		       pl.emotion AS my_emotion,
		       p.created_at,
		       (p.signature IS NOT NULL) AS is_signed
		FROM posts p
		LEFT JOIN post_likes pl
		       ON pl.post_id = p.id AND pl.user_id = $1
		WHERE p.deleted_at IS NULL
		  AND p.parent_id IS NULL
		ORDER BY (p.reach_score + 1.0) /
		         POWER(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600.0 + 2.0, 1.5) DESC,
		         p.created_at DESC
		LIMIT $2
	`
	rows, err := s.pool.Query(ctx, q, p.ViewerID, p.Limit)
	if err != nil {
		return nil, fmt.Errorf("list explore posts: %w", err)
	}
	defer rows.Close()
	return collectFeedPosts(rows)
}

// ─── Friend reactor queries ───────────────────────────────────────────────────

// PostReactor is a friend who reacted to a post, enriched with user info.
type PostReactor struct {
	UserID      uuid.UUID
	Username    string
	DisplayName *string
	Emotion     string
}

// GetPostReactorEmotions returns (userID → emotion) for postID where the
// reactor is in candidateIDs, ordered most-recent first up to limit.
// The service layer then batch-fetches user profiles from the auth DB.
func (s *ContentStore) GetPostReactorEmotions(
	ctx context.Context,
	postID uuid.UUID,
	candidateIDs []uuid.UUID,
	limit int,
) (map[uuid.UUID]string, error) {
	if len(candidateIDs) == 0 {
		return nil, nil
	}
	const q = `
		SELECT pl.user_id, pl.emotion
		FROM post_likes pl
		WHERE pl.post_id = $1
		  AND pl.user_id = ANY($2)
		ORDER BY pl.updated_at DESC
		LIMIT $3
	`
	rows, err := s.pool.Query(ctx, q, postID, candidateIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("get post reactor emotions: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]string)
	for rows.Next() {
		var uid uuid.UUID
		var emotion string
		if err := rows.Scan(&uid, &emotion); err != nil {
			return nil, fmt.Errorf("scan reactor emotion: %w", err)
		}
		out[uid] = emotion
	}
	return out, rows.Err()
}

// ─── collectFeedPosts helper ──────────────────────────────────────────────────

func collectFeedPosts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]FeedPost, error) {
	var posts []FeedPost
	for rows.Next() {
		var fp FeedPost
		// Scan reaction_counts JSONB as raw bytes, then unmarshal.
		var reactionCountsRaw json.RawMessage
		if err := rows.Scan(
			&fp.ID, &fp.AuthorID, &fp.Content, &fp.Kind,
			&fp.NoteTitle, &fp.NoteSummary, &fp.ResharedFromID,
			&fp.CommentCount, &reactionCountsRaw,
			&fp.MyEmotion,
			&fp.CreatedAt,
			&fp.IsSigned,
		); err != nil {
			return nil, fmt.Errorf("scan feed post: %w", err)
		}
		fp.ReactionCounts = make(map[string]int32)
		if len(reactionCountsRaw) > 0 {
			if err := json.Unmarshal(reactionCountsRaw, &fp.ReactionCounts); err != nil {
				return nil, fmt.Errorf("unmarshal reaction_counts: %w", err)
			}
		}
		posts = append(posts, fp)
	}
	return posts, rows.Err()
}
