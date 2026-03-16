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

// Pool wraps pgxpool and provides all DB operations for the content service.
type Pool struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
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

func (p *Pool) Close() {
	p.pool.Close()
}

// ─── Models ───────────────────────────────────────────────────────────────────

// VcRequirement is one entry in a board's VC gate list.
type VcRequirement struct {
	VcType string `json:"vc_type"`
	Issuer string `json:"issuer"`
}

type Board struct {
	ID                uuid.UUID
	OwnerID           uuid.UUID
	Name              string
	Description       *string
	DefaultAccess     string
	MinTrustLevel     int16
	CommentPolicy     string
	MinCommentTrust   int16
	RequireVcs        []VcRequirement // post-write gate
	RequireCommentVcs []VcRequirement // comment-write gate
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Post struct {
	ID             uuid.UUID
	AuthorID       uuid.UUID
	ParentID       *uuid.UUID
	RootID         *uuid.UUID
	Kind           string
	Content        string
	NoteTitle      *string
	NoteCover      *string
	NoteSummary    *string
	ResharedFromID *uuid.UUID
	ReachScore     float64
	Signature      []byte
	CreatedAt      time.Time
	DeletedAt      *time.Time
	// Computed via subqueries
	LikeCount     int64
	ReplyCount    int64
	IsLiked       bool
	ViewerEmotion *string
}

type ReactionCount struct {
	Emotion string
	Count   int64
}

type Article struct {
	ID            uuid.UUID
	BoardID       uuid.UUID
	AuthorID      uuid.UUID
	Title         string
	Slug          string
	ContentMd     *string
	ContentJSON   []byte
	Status        string
	AccessPolicy  string
	MinTrustLevel int16
	ReachScore    float64
	Signature     []byte
	PublishedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ArticleComment struct {
	ID        uuid.UUID
	ArticleID uuid.UUID
	AuthorID  uuid.UUID
	ParentID  *uuid.UUID
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// ─── Board queries ────────────────────────────────────────────────────────────

const boardCols = `id, owner_id, name, description, default_access, min_trust_level,
		          comment_policy, min_comment_trust, require_vcs, require_comment_vcs, created_at, updated_at`

func (p *Pool) CreateBoard(ctx context.Context, ownerID uuid.UUID, name string) (Board, error) {
	q := `INSERT INTO boards (owner_id, name) VALUES ($1, $2) RETURNING ` + boardCols
	return scanBoard(p.pool.QueryRow(ctx, q, ownerID, name))
}

func (p *Pool) GetBoardByOwnerID(ctx context.Context, ownerID uuid.UUID) (Board, error) {
	q := `SELECT ` + boardCols + ` FROM boards WHERE owner_id = $1`
	return scanBoard(p.pool.QueryRow(ctx, q, ownerID))
}

func (p *Pool) GetBoardByID(ctx context.Context, id uuid.UUID) (Board, error) {
	q := `SELECT ` + boardCols + ` FROM boards WHERE id = $1`
	return scanBoard(p.pool.QueryRow(ctx, q, id))
}

type UpdateBoardParams struct {
	ID            uuid.UUID
	Name          *string
	Description   *string
	DefaultAccess *string
}

func (p *Pool) UpdateBoard(ctx context.Context, params UpdateBoardParams) (Board, error) {
	q := `
		UPDATE boards SET
			name           = COALESCE($2, name),
			description    = COALESCE($3, description),
			default_access = COALESCE($4, default_access),
			updated_at     = now()
		WHERE id = $1
		RETURNING ` + boardCols
	return scanBoard(p.pool.QueryRow(ctx, q, params.ID, params.Name, params.Description, params.DefaultAccess))
}

// UpdateBoardVcPolicy sets the trust-level gates and VC requirement lists.
type UpdateBoardVcPolicyParams struct {
	ID                uuid.UUID
	MinTrustLevel     int16
	MinCommentTrust   int16
	RequireVcs        []VcRequirement
	RequireCommentVcs []VcRequirement
}

func (p *Pool) UpdateBoardVcPolicy(ctx context.Context, params UpdateBoardVcPolicyParams) (Board, error) {
	rvRaw, err := json.Marshal(params.RequireVcs)
	if err != nil {
		return Board{}, fmt.Errorf("marshal require_vcs: %w", err)
	}
	rcRaw, err := json.Marshal(params.RequireCommentVcs)
	if err != nil {
		return Board{}, fmt.Errorf("marshal require_comment_vcs: %w", err)
	}
	q := `
		UPDATE boards SET
			min_trust_level     = $2,
			min_comment_trust   = $3,
			require_vcs         = $4,
			require_comment_vcs = $5,
			updated_at          = now()
		WHERE id = $1
		RETURNING ` + boardCols
	return scanBoard(p.pool.QueryRow(ctx, q,
		params.ID, params.MinTrustLevel, params.MinCommentTrust, rvRaw, rcRaw,
	))
}

// ─── Board subscriber queries ─────────────────────────────────────────────────

func (p *Pool) SubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO board_subscribers (board_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		boardID, userID,
	)
	return err
}

func (p *Pool) UnsubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`DELETE FROM board_subscribers WHERE board_id = $1 AND user_id = $2`,
		boardID, userID,
	)
	return err
}

func (p *Pool) IsSubscribed(ctx context.Context, boardID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM board_subscribers WHERE board_id = $1 AND user_id = $2)`,
		boardID, userID,
	).Scan(&exists)
	return exists, err
}

func (p *Pool) CountSubscribers(ctx context.Context, boardID uuid.UUID) (int64, error) {
	var count int64
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM board_subscribers WHERE board_id = $1`,
		boardID,
	).Scan(&count)
	return count, err
}

func scanBoard(row pgx.Row) (Board, error) {
	var b Board
	var rvRaw, rcRaw []byte
	err := row.Scan(
		&b.ID, &b.OwnerID, &b.Name, &b.Description,
		&b.DefaultAccess, &b.MinTrustLevel,
		&b.CommentPolicy, &b.MinCommentTrust,
		&rvRaw, &rcRaw,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return Board{}, fmt.Errorf("scan board: %w", err)
	}
	if err := json.Unmarshal(rvRaw, &b.RequireVcs); err != nil {
		b.RequireVcs = nil
	}
	if err := json.Unmarshal(rcRaw, &b.RequireCommentVcs); err != nil {
		b.RequireCommentVcs = nil
	}
	return b, nil
}

// ─── Post queries ─────────────────────────────────────────────────────────────

// TrustMultiplier returns the reach-score weight for a given trust level.
// L0=1.0, L1=1.5, L2=2.5, L3=4.0, L4+=6.0
func TrustMultiplier(trustLevel int) float64 {
	switch {
	case trustLevel >= 4:
		return 6.0
	case trustLevel == 3:
		return 4.0
	case trustLevel == 2:
		return 2.5
	case trustLevel == 1:
		return 1.5
	default:
		return 1.0
	}
}

type CreatePostParams struct {
	AuthorID         uuid.UUID
	ParentID         *uuid.UUID
	RootID           *uuid.UUID
	Content          string
	AuthorTrustLevel int
}

func (p *Pool) CreatePost(ctx context.Context, params CreatePostParams) (Post, error) {
	const q = `
		INSERT INTO posts (author_id, parent_id, root_id, content, reach_score)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, author_id, parent_id, root_id, kind, content,
		          note_title, note_cover, note_summary, reshared_from_id,
		          reach_score, signature, created_at, deleted_at,
		          0::bigint AS like_count, 0::bigint AS reply_count, false AS is_liked,
		          NULL::text AS viewer_emotion
	`
	initialReachScore := TrustMultiplier(params.AuthorTrustLevel)
	return scanPost(p.pool.QueryRow(ctx, q,
		params.AuthorID, params.ParentID, params.RootID, params.Content, initialReachScore))
}

// GetPostByID returns a post with computed like/reply counts and isLiked for viewerID.
// Pass nil for viewerID when not authenticated.
func (p *Pool) GetPostByID(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (Post, error) {
	const q = `
		SELECT p.id, p.author_id, p.parent_id, p.root_id, p.kind, p.content,
		       p.note_title, p.note_cover, p.note_summary, p.reshared_from_id,
		       p.reach_score, p.signature, p.created_at, p.deleted_at,
		       (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) AS like_count,
		       (SELECT COUNT(*) FROM posts WHERE parent_id = p.id AND deleted_at IS NULL) AS reply_count,
		       EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2 AND emotion = 'like') AS is_liked,
		       (SELECT emotion FROM post_likes WHERE post_id = p.id AND user_id = $2 LIMIT 1) AS viewer_emotion
		FROM posts p
		WHERE p.id = $1
	`
	return scanPost(p.pool.QueryRow(ctx, q, id, viewerID))
}

type ListPostsParams struct {
	After    *uuid.UUID // cursor: last seen post ID
	Limit    int
	ViewerID *uuid.UUID
}

func (p *Pool) ListPosts(ctx context.Context, params ListPostsParams) ([]Post, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	const q = `
		SELECT p.id, p.author_id, p.parent_id, p.root_id, p.kind, p.content,
		       p.note_title, p.note_cover, p.note_summary, p.reshared_from_id,
		       p.reach_score, p.signature, p.created_at, p.deleted_at,
		       (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) AS like_count,
		       (SELECT COUNT(*) FROM posts WHERE parent_id = p.id AND deleted_at IS NULL) AS reply_count,
		       EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $3 AND emotion = 'like') AS is_liked,
		       (SELECT emotion FROM post_likes WHERE post_id = p.id AND user_id = $3 LIMIT 1) AS viewer_emotion
		FROM posts p
		WHERE p.deleted_at IS NULL
		  AND p.parent_id IS NULL
		  AND p.kind = 'post'
		  AND ($1::uuid IS NULL OR p.created_at < (SELECT created_at FROM posts WHERE id = $1))
		ORDER BY p.created_at DESC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, params.After, limit, params.ViewerID)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

type ListPostRepliesParams struct {
	ParentID uuid.UUID
	ViewerID *uuid.UUID
	Limit    int
}

func (p *Pool) ListPostReplies(ctx context.Context, params ListPostRepliesParams) ([]Post, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const q = `
		SELECT p.id, p.author_id, p.parent_id, p.root_id, p.kind, p.content,
		       p.note_title, p.note_cover, p.note_summary, p.reshared_from_id,
		       p.reach_score, p.signature, p.created_at, p.deleted_at,
		       (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) AS like_count,
		       (SELECT COUNT(*) FROM posts WHERE parent_id = p.id AND deleted_at IS NULL) AS reply_count,
		       EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $3 AND emotion = 'like') AS is_liked,
		       (SELECT emotion FROM post_likes WHERE post_id = p.id AND user_id = $3 LIMIT 1) AS viewer_emotion
		FROM posts p
		WHERE p.deleted_at IS NULL
		  AND p.parent_id = $1
		ORDER BY p.created_at ASC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, params.ParentID, limit, params.ViewerID)
	if err != nil {
		return nil, fmt.Errorf("list post replies: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

func (p *Pool) SoftDeletePost(ctx context.Context, id, authorID uuid.UUID) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE posts SET deleted_at = now() WHERE id = $1 AND author_id = $2 AND deleted_at IS NULL`,
		id, authorID,
	)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("post not found or not owned by user")
	}
	return nil
}

func (p *Pool) LikePost(ctx context.Context, postID, userID uuid.UUID) error {
	return p.ReactPost(ctx, postID, userID, "like", nil)
}

func (p *Pool) ReactPost(ctx context.Context, postID, userID uuid.UUID, emotion string, sourceIP *string) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO post_likes (post_id, user_id, emotion, source_ip) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (post_id, user_id) DO UPDATE
		 SET emotion = EXCLUDED.emotion,
		     source_ip = EXCLUDED.source_ip,
		     updated_at = now()`,
		postID, userID, emotion,
		sourceIP,
	)
	return err
}

func (p *Pool) UnlikePost(ctx context.Context, postID, userID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`,
		postID, userID,
	)
	return err
}

func (p *Pool) ListPostReactionCounts(ctx context.Context, postID uuid.UUID) ([]ReactionCount, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT emotion, COUNT(*) FROM post_likes WHERE post_id = $1 GROUP BY emotion ORDER BY COUNT(*) DESC, emotion ASC`,
		postID,
	)
	if err != nil {
		return nil, fmt.Errorf("list post reaction counts: %w", err)
	}
	defer rows.Close()

	var out []ReactionCount
	for rows.Next() {
		var rc ReactionCount
		if err := rows.Scan(&rc.Emotion, &rc.Count); err != nil {
			return nil, fmt.Errorf("scan reaction count: %w", err)
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (p *Pool) UpdatePostSignature(ctx context.Context, id uuid.UUID, signature []byte) error {
	_, err := p.pool.Exec(ctx, `UPDATE posts SET signature = $2 WHERE id = $1`, id, signature)
	if err != nil {
		return fmt.Errorf("update post signature: %w", err)
	}
	return nil
}

func scanPost(row pgx.Row) (Post, error) {
	var post Post
	err := row.Scan(
		&post.ID, &post.AuthorID, &post.ParentID, &post.RootID, &post.Kind, &post.Content,
		&post.NoteTitle, &post.NoteCover, &post.NoteSummary, &post.ResharedFromID,
		&post.ReachScore, &post.Signature, &post.CreatedAt, &post.DeletedAt,
		&post.LikeCount, &post.ReplyCount, &post.IsLiked, &post.ViewerEmotion,
	)
	if err != nil {
		return Post{}, fmt.Errorf("scan post: %w", err)
	}
	return post, nil
}

func collectPosts(rows pgx.Rows) ([]Post, error) {
	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(
			&post.ID, &post.AuthorID, &post.ParentID, &post.RootID, &post.Kind, &post.Content,
			&post.NoteTitle, &post.NoteCover, &post.NoteSummary, &post.ResharedFromID,
			&post.ReachScore, &post.Signature, &post.CreatedAt, &post.DeletedAt,
			&post.LikeCount, &post.ReplyCount, &post.IsLiked, &post.ViewerEmotion,
		)
		if err != nil {
			return nil, fmt.Errorf("scan post row: %w", err)
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// ─── Reshare queries ──────────────────────────────────────────────────────────

type ResharePostParams struct {
	AuthorID       uuid.UUID
	Content        string // optional comment
	ResharedFromID uuid.UUID
}

func (p *Pool) ResharePost(ctx context.Context, params ResharePostParams) (Post, error) {
	const q = `
		INSERT INTO posts (author_id, content, kind, reshared_from_id)
		VALUES ($1, $2, 'post', $3)
		RETURNING id, author_id, parent_id, root_id, kind, content,
		          note_title, note_cover, note_summary, reshared_from_id,
		          reach_score, signature, created_at, deleted_at,
		          0::bigint AS like_count, 0::bigint AS reply_count, false AS is_liked,
		          NULL::text AS viewer_emotion
	`
	return scanPost(p.pool.QueryRow(ctx, q,
		params.AuthorID, params.Content, params.ResharedFromID))
}

// ─── Note queries ─────────────────────────────────────────────────────────────

type CreateNoteParams struct {
	AuthorID         uuid.UUID
	Content          string // Tiptap HTML
	NoteTitle        string
	NoteCover        *string
	NoteSummary      *string
	AuthorTrustLevel int
}

func (p *Pool) CreateNote(ctx context.Context, params CreateNoteParams) (Post, error) {
	const q = `
		INSERT INTO posts (author_id, content, kind, note_title, note_cover, note_summary, reach_score)
		VALUES ($1, $2, 'note', $3, $4, $5, $6)
		RETURNING id, author_id, parent_id, root_id, kind, content,
		          note_title, note_cover, note_summary, reshared_from_id,
		          reach_score, signature, created_at, deleted_at,
		          0::bigint AS like_count, 0::bigint AS reply_count, false AS is_liked,
		          NULL::text AS viewer_emotion
	`
	initialReachScore := TrustMultiplier(params.AuthorTrustLevel)
	return scanPost(p.pool.QueryRow(ctx, q,
		params.AuthorID, params.Content, params.NoteTitle,
		params.NoteCover, params.NoteSummary, initialReachScore))
}

type ListNotesParams struct {
	After    *uuid.UUID
	Limit    int
	ViewerID *uuid.UUID
}

func (p *Pool) ListNotes(ctx context.Context, params ListNotesParams) ([]Post, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	const q = `
		SELECT p.id, p.author_id, p.parent_id, p.root_id, p.kind, p.content,
		       p.note_title, p.note_cover, p.note_summary, p.reshared_from_id,
		       p.reach_score, p.signature, p.created_at, p.deleted_at,
		       (SELECT COUNT(*) FROM post_likes WHERE post_id = p.id) AS like_count,
		       (SELECT COUNT(*) FROM posts WHERE parent_id = p.id AND deleted_at IS NULL) AS reply_count,
		       EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $3 AND emotion = 'like') AS is_liked,
		       (SELECT emotion FROM post_likes WHERE post_id = p.id AND user_id = $3 LIMIT 1) AS viewer_emotion
		FROM posts p
		WHERE p.deleted_at IS NULL
		  AND p.parent_id IS NULL
		  AND p.kind = 'note'
		  AND ($1::uuid IS NULL OR p.created_at < (SELECT created_at FROM posts WHERE id = $1))
		ORDER BY p.created_at DESC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, params.After, limit, params.ViewerID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

// ─── Federation / internal queries ───────────────────────────────────────────

// ListPublicPostsByAuthor returns non-deleted, top-level posts (kind='post')
// ordered by created_at DESC. Pass before as a time cursor for pagination.
func (p *Pool) ListPublicPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit int, before *time.Time) ([]Post, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	const q = `
		SELECT id, author_id, parent_id, root_id, kind, content,
		       note_title, note_cover, note_summary, reshared_from_id,
		       reach_score, signature, created_at, deleted_at,
		       0::bigint AS like_count, 0::bigint AS reply_count,
		       false AS is_liked, NULL::text AS viewer_emotion
		FROM posts
		WHERE author_id = $1
		  AND deleted_at IS NULL
		  AND parent_id IS NULL
		  AND kind = 'post'
		  AND ($3::timestamptz IS NULL OR created_at < $3)
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, authorID, limit, before)
	if err != nil {
		return nil, fmt.Errorf("list public posts by author: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

// ─── Article queries ──────────────────────────────────────────────────────────

type CreateArticleParams struct {
	BoardID      uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Slug         string
	ContentMd    *string
	AccessPolicy string
}

func (p *Pool) CreateArticle(ctx context.Context, params CreateArticleParams) (Article, error) {
	const q = `
		INSERT INTO articles (board_id, author_id, title, slug, content_md, access_policy)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, board_id, author_id, title, slug, content_md, content_json,
		          status, access_policy, min_trust_level, reach_score, signature,
		          published_at, created_at, updated_at
	`
	return scanArticle(p.pool.QueryRow(ctx, q,
		params.BoardID, params.AuthorID, params.Title, params.Slug,
		params.ContentMd, params.AccessPolicy))
}

func (p *Pool) GetArticleByID(ctx context.Context, id uuid.UUID) (Article, error) {
	const q = `
		SELECT id, board_id, author_id, title, slug, content_md, content_json,
		       status, access_policy, min_trust_level, reach_score, signature,
		       published_at, created_at, updated_at
		FROM articles WHERE id = $1
	`
	return scanArticle(p.pool.QueryRow(ctx, q, id))
}

func (p *Pool) GetArticleBySlug(ctx context.Context, boardID uuid.UUID, slug string) (Article, error) {
	const q = `
		SELECT id, board_id, author_id, title, slug, content_md, content_json,
		       status, access_policy, min_trust_level, reach_score, signature,
		       published_at, created_at, updated_at
		FROM articles WHERE board_id = $1 AND slug = $2
	`
	return scanArticle(p.pool.QueryRow(ctx, q, boardID, slug))
}

type UpdateArticleParams struct {
	ID           uuid.UUID
	Title        *string
	ContentMd    *string
	AccessPolicy *string
	Status       *string
}

func (p *Pool) UpdateArticle(ctx context.Context, params UpdateArticleParams) (Article, error) {
	const q = `
		UPDATE articles SET
			title         = COALESCE($2, title),
			content_md    = COALESCE($3, content_md),
			access_policy = COALESCE($4, access_policy),
			status        = COALESCE($5, status),
			updated_at    = now()
		WHERE id = $1
		RETURNING id, board_id, author_id, title, slug, content_md, content_json,
		          status, access_policy, min_trust_level, reach_score, signature,
		          published_at, created_at, updated_at
	`
	return scanArticle(p.pool.QueryRow(ctx, q,
		params.ID, params.Title, params.ContentMd, params.AccessPolicy, params.Status))
}

func (p *Pool) PublishArticle(ctx context.Context, id uuid.UUID) (Article, error) {
	const q = `
		UPDATE articles SET
			status       = 'published',
			published_at = COALESCE(published_at, now()),
			updated_at   = now()
		WHERE id = $1
		RETURNING id, board_id, author_id, title, slug, content_md, content_json,
		          status, access_policy, min_trust_level, reach_score, signature,
		          published_at, created_at, updated_at
	`
	return scanArticle(p.pool.QueryRow(ctx, q, id))
}

func (p *Pool) DeleteArticle(ctx context.Context, id, authorID uuid.UUID) error {
	tag, err := p.pool.Exec(ctx,
		`DELETE FROM articles WHERE id = $1 AND author_id = $2`,
		id, authorID,
	)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("article not found or not owned by user")
	}
	return nil
}

func (p *Pool) UpdateArticleSignature(ctx context.Context, id uuid.UUID, signature []byte) error {
	_, err := p.pool.Exec(ctx, `UPDATE articles SET signature = $2, updated_at = now() WHERE id = $1`, id, signature)
	if err != nil {
		return fmt.Errorf("update article signature: %w", err)
	}
	return nil
}

type ListArticlesParams struct {
	BoardID uuid.UUID
	After   *uuid.UUID // cursor: last seen article ID
	Limit   int
	// If true, include drafts (only for the board owner)
	IncludeDrafts bool
}

func (p *Pool) ListBoardArticles(ctx context.Context, params ListArticlesParams) ([]Article, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	const q = `
		SELECT id, board_id, author_id, title, slug, content_md, content_json,
		       status, access_policy, min_trust_level, reach_score, signature,
		       published_at, created_at, updated_at
		FROM articles
		WHERE board_id = $1
		  AND ($2 OR status = 'published')
		  AND ($3::uuid IS NULL OR published_at < (SELECT published_at FROM articles WHERE id = $3))
		ORDER BY published_at DESC NULLS LAST, created_at DESC
		LIMIT $4
	`
	rows, err := p.pool.Query(ctx, q, params.BoardID, params.IncludeDrafts, params.After, limit)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	return collectArticles(rows)
}

func scanArticle(row pgx.Row) (Article, error) {
	var a Article
	err := row.Scan(
		&a.ID, &a.BoardID, &a.AuthorID, &a.Title, &a.Slug,
		&a.ContentMd, &a.ContentJSON, &a.Status, &a.AccessPolicy,
		&a.MinTrustLevel, &a.ReachScore, &a.Signature,
		&a.PublishedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return Article{}, fmt.Errorf("scan article: %w", err)
	}
	return a, nil
}

func collectArticles(rows pgx.Rows) ([]Article, error) {
	var articles []Article
	for rows.Next() {
		var a Article
		err := rows.Scan(
			&a.ID, &a.BoardID, &a.AuthorID, &a.Title, &a.Slug,
			&a.ContentMd, &a.ContentJSON, &a.Status, &a.AccessPolicy,
			&a.MinTrustLevel, &a.ReachScore, &a.Signature,
			&a.PublishedAt, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan article row: %w", err)
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// ─── Article comment queries ──────────────────────────────────────────────────

type CreateArticleCommentParams struct {
	ArticleID uuid.UUID
	AuthorID  uuid.UUID
	ParentID  *uuid.UUID
	Content   string
}

func (p *Pool) CreateArticleComment(ctx context.Context, params CreateArticleCommentParams) (ArticleComment, error) {
	const q = `
		INSERT INTO article_comments (article_id, author_id, parent_id, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, article_id, author_id, parent_id, content, created_at, updated_at, deleted_at
	`
	var c ArticleComment
	err := p.pool.QueryRow(ctx, q, params.ArticleID, params.AuthorID, params.ParentID, params.Content).Scan(
		&c.ID, &c.ArticleID, &c.AuthorID, &c.ParentID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if err != nil {
		return ArticleComment{}, fmt.Errorf("create article comment: %w", err)
	}
	return c, nil
}

func (p *Pool) ListArticleComments(ctx context.Context, articleID uuid.UUID, limit int) ([]ArticleComment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT id, article_id, author_id, parent_id, content, created_at, updated_at, deleted_at
		FROM article_comments
		WHERE article_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, articleID, limit)
	if err != nil {
		return nil, fmt.Errorf("list article comments: %w", err)
	}
	defer rows.Close()

	var out []ArticleComment
	for rows.Next() {
		var c ArticleComment
		if err := rows.Scan(
			&c.ID, &c.ArticleID, &c.AuthorID, &c.ParentID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan article comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Pool) ListCommentReplies(ctx context.Context, parentID uuid.UUID, limit int) ([]ArticleComment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT id, article_id, author_id, parent_id, content, created_at, updated_at, deleted_at
		FROM article_comments
		WHERE parent_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := p.pool.Query(ctx, q, parentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list comment replies: %w", err)
	}
	defer rows.Close()

	var out []ArticleComment
	for rows.Next() {
		var c ArticleComment
		if err := rows.Scan(
			&c.ID, &c.ArticleID, &c.AuthorID, &c.ParentID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment reply: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
