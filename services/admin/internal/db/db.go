package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Pool constructors ───────────────────────────────────────────────────────

// AuthPool is a connection pool to the auth DB.
// Used for admin_users, audit_log, and user management.
type AuthPool struct{ pool *pgxpool.Pool }

// ContentPool is a read+write connection pool to the content DB.
// Capped at 5 connections so admin queries never starve the main services.
type ContentPool struct{ pool *pgxpool.Pool }

func NewAuthPool(ctx context.Context, url string) (*AuthPool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse auth db url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create auth pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping auth db: %w", err)
	}
	return &AuthPool{pool: p}, nil
}

func NewContentPool(ctx context.Context, url string) (*ContentPool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse content db url: %w", err)
	}
	cfg.MaxConns = 5
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create content pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping content db: %w", err)
	}
	return &ContentPool{pool: p}, nil
}

func (p *AuthPool) Close()    { p.pool.Close() }
func (p *ContentPool) Close() { p.pool.Close() }

// ─── Models ──────────────────────────────────────────────────────────────────

type AdminUser struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash []byte
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

type AuditEntry struct {
	ID           uuid.UUID
	AdminID      uuid.UUID
	AdminUsername string
	Action       string
	TargetType   string
	TargetID     uuid.UUID
	Note         *string
	Metadata     *string
	CreatedAt    time.Time
}

type User struct {
	ID          uuid.UUID
	Username    string
	DisplayName *string
	Email       *string
	TrustLevel  int16
	IsSuspended bool
	CreatedAt   time.Time
}

type Report struct {
	ID              uuid.UUID
	PostID          uuid.UUID
	ReporterID      uuid.UUID
	ReporterUsername string
	Reason          string
	Note            *string
	CreatedAt       time.Time
}

type ReportGroup struct {
	PostID          uuid.UUID
	PostContent     string
	PostDeleted     bool
	AuthorID        uuid.UUID
	AuthorUsername  string
	ReportCount     int
	OldestReport    time.Time
	Reports         []Report
}

type Post struct {
	ID         uuid.UUID
	Content    string
	AuthorID   uuid.UUID
	ReplyCount int
	LikeCount  int
	OpenReports int
	CreatedAt  time.Time
	DeletedAt  *time.Time
}

// ─── Admin user queries ───────────────────────────────────────────────────────

func (p *AuthPool) GetAdminByUsername(ctx context.Context, username string) (*AdminUser, error) {
	const q = `SELECT id, username, email, password_hash, role, is_active, created_at, last_login_at
	           FROM admin_users WHERE username = $1 AND is_active = true`
	row := p.pool.QueryRow(ctx, q, username)
	a := &AdminUser{}
	if err := row.Scan(&a.ID, &a.Username, &a.Email, &a.PasswordHash,
		&a.Role, &a.IsActive, &a.CreatedAt, &a.LastLoginAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (p *AuthPool) GetAdminByID(ctx context.Context, id uuid.UUID) (*AdminUser, error) {
	const q = `SELECT id, username, email, password_hash, role, is_active, created_at, last_login_at
	           FROM admin_users WHERE id = $1`
	row := p.pool.QueryRow(ctx, q, id)
	a := &AdminUser{}
	if err := row.Scan(&a.ID, &a.Username, &a.Email, &a.PasswordHash,
		&a.Role, &a.IsActive, &a.CreatedAt, &a.LastLoginAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (p *AuthPool) TouchAdminLogin(ctx context.Context, id uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE admin_users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// ─── Audit log ────────────────────────────────────────────────────────────────

type CreateAuditParams struct {
	AdminID    uuid.UUID
	Action     string
	TargetType string
	TargetID   uuid.UUID
	Note       *string
	Metadata   *string // JSON string
}

func (p *AuthPool) CreateAuditEntry(ctx context.Context, params CreateAuditParams) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO audit_log (admin_id, action, target_type, target_id, note, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		params.AdminID, params.Action, params.TargetType, params.TargetID,
		params.Note, params.Metadata,
	)
	return err
}

func (p *AuthPool) ListAuditLog(ctx context.Context, adminID *uuid.UUID, targetType *string, targetID *uuid.UUID, after *time.Time, limit int) ([]AuditEntry, error) {
	q := `SELECT al.id, al.admin_id, au.username, al.action, al.target_type, al.target_id,
	             al.note, al.metadata::text, al.created_at
	      FROM audit_log al
	      JOIN admin_users au ON au.id = al.admin_id
	      WHERE ($1::uuid IS NULL OR al.admin_id = $1)
	        AND ($2::text IS NULL OR al.target_type = $2)
	        AND ($3::uuid IS NULL OR al.target_id = $3)
	        AND ($4::timestamptz IS NULL OR al.created_at < $4)
	      ORDER BY al.created_at DESC LIMIT $5`
	rows, err := p.pool.Query(ctx, q, adminID, targetType, targetID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.AdminID, &e.AdminUsername, &e.Action,
			&e.TargetType, &e.TargetID, &e.Note, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ─── User management (auth DB) ────────────────────────────────────────────────

func (p *AuthPool) ListUsers(ctx context.Context, search *string, trustLevel *int16, suspended *bool, after *time.Time, limit int) ([]User, error) {
	q := `SELECT id, username, display_name, email, trust_level, is_suspended, created_at
	      FROM users
	      WHERE deleted_at IS NULL
	        AND ($1::text IS NULL OR username ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
	        AND ($2::smallint IS NULL OR trust_level = $2)
	        AND ($3::boolean IS NULL OR is_suspended = $3)
	        AND ($4::timestamptz IS NULL OR created_at < $4)
	      ORDER BY created_at DESC LIMIT $5`
	rows, err := p.pool.Query(ctx, q, search, trustLevel, suspended, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email,
			&u.TrustLevel, &u.IsSuspended, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (p *AuthPool) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `SELECT id, username, display_name, email, trust_level, is_suspended, created_at
	           FROM users WHERE id = $1 AND deleted_at IS NULL`
	row := p.pool.QueryRow(ctx, q, id)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email,
		&u.TrustLevel, &u.IsSuspended, &u.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (p *AuthPool) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, username, display_name, email, trust_level, is_suspended, created_at
		 FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uuid.UUID]User, len(ids))
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email,
			&u.TrustLevel, &u.IsSuspended, &u.CreatedAt); err != nil {
			return nil, err
		}
		m[u.ID] = u
	}
	return m, rows.Err()
}

func (p *AuthPool) SetTrustLevel(ctx context.Context, userID uuid.UUID, level int16) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE users SET trust_level = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		userID, level)
	return err
}

func (p *AuthPool) SetSuspended(ctx context.Context, userID uuid.UUID, suspended bool) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE users SET is_suspended = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		userID, suspended)
	return err
}

// ─── Dashboard stats (auth DB) ────────────────────────────────────────────────

func (p *AuthPool) CountNewUsers(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE created_at >= $1 AND deleted_at IS NULL`, since).Scan(&n)
	return n, err
}

func (p *AuthPool) CountTotalUsers(ctx context.Context) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// ─── Report queue (content DB) ────────────────────────────────────────────────

// ListReportGroups returns posts that have unresolved (or resolved) reports,
// grouped in memory after a single efficient query.
func (p *ContentPool) ListReportGroups(ctx context.Context, resolved bool, after *time.Time, limit int) ([]ReportGroup, error) {
	// Fetch the top-N most-recently-reported posts, then their reports.
	// The idx_reports_post_unresolved / idx_reports_open index covers this.
	// Two compile-time constant queries are used to avoid any fmt.Sprintf on
	// SQL strings; neither contains user input.
	const qUnresolved = `
		WITH ranked AS (
			SELECT r.post_id,
			       COUNT(*)                          AS report_count,
			       MIN(r.created_at)                 AS oldest_report
			FROM reports r
			WHERE r.resolved_at IS NULL
			  AND ($1::timestamptz IS NULL OR r.created_at < $1)
			GROUP BY r.post_id
			ORDER BY oldest_report ASC
			LIMIT $2
		)
		SELECT r.id, r.post_id, r.reporter_id, r.reason, r.note, r.created_at,
		       ranked.report_count, ranked.oldest_report,
		       p.content,
		       p.deleted_at IS NOT NULL AS post_deleted,
		       p.author_id
		FROM ranked
		JOIN reports r ON r.post_id = ranked.post_id AND r.resolved_at IS NULL
		JOIN posts   p ON p.id      = ranked.post_id
		ORDER BY ranked.oldest_report ASC, r.created_at ASC
	`
	const qResolved = `
		WITH ranked AS (
			SELECT r.post_id,
			       COUNT(*)                          AS report_count,
			       MIN(r.created_at)                 AS oldest_report
			FROM reports r
			WHERE r.resolved_at IS NOT NULL
			  AND ($1::timestamptz IS NULL OR r.created_at < $1)
			GROUP BY r.post_id
			ORDER BY oldest_report ASC
			LIMIT $2
		)
		SELECT r.id, r.post_id, r.reporter_id, r.reason, r.note, r.created_at,
		       ranked.report_count, ranked.oldest_report,
		       p.content,
		       p.deleted_at IS NOT NULL AS post_deleted,
		       p.author_id
		FROM ranked
		JOIN reports r ON r.post_id = ranked.post_id AND r.resolved_at IS NOT NULL
		JOIN posts   p ON p.id      = ranked.post_id
		ORDER BY ranked.oldest_report ASC, r.created_at ASC
	`

	var q string
	if resolved {
		q = qResolved
	} else {
		q = qUnresolved
	}

	rows, err := p.pool.Query(ctx, q, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect raw rows, build groups keyed by post_id.
	type rawRow struct {
		reportID    uuid.UUID
		postID      uuid.UUID
		reporterID  uuid.UUID
		reason      string
		note        *string
		createdAt   time.Time
		reportCount int
		oldestReport time.Time
		content     string
		deleted     bool
		authorID    uuid.UUID
	}
	groupMap := make(map[uuid.UUID]*ReportGroup)
	var order []uuid.UUID

	for rows.Next() {
		var rr rawRow
		if err := rows.Scan(&rr.reportID, &rr.postID, &rr.reporterID, &rr.reason,
			&rr.note, &rr.createdAt, &rr.reportCount, &rr.oldestReport,
			&rr.content, &rr.deleted, &rr.authorID); err != nil {
			return nil, err
		}
		if _, ok := groupMap[rr.postID]; !ok {
			groupMap[rr.postID] = &ReportGroup{
				PostID:      rr.postID,
				PostContent: rr.content,
				PostDeleted: rr.deleted,
				AuthorID:    rr.authorID,
				ReportCount: rr.reportCount,
				OldestReport: rr.oldestReport,
			}
			order = append(order, rr.postID)
		}
		groupMap[rr.postID].Reports = append(groupMap[rr.postID].Reports, Report{
			ID:         rr.reportID,
			PostID:     rr.postID,
			ReporterID: rr.reporterID,
			Reason:     rr.reason,
			Note:       rr.note,
			CreatedAt:  rr.createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	groups := make([]ReportGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *groupMap[id])
	}
	return groups, nil
}

func (p *ContentPool) ResolveReportsByPost(ctx context.Context, postID, resolverID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE reports SET resolved_at = now(), resolved_by = $2
		 WHERE post_id = $1 AND resolved_at IS NULL`,
		postID, resolverID)
	return err
}

func (p *ContentPool) SoftDeletePost(ctx context.Context, postID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE posts SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		postID)
	return err
}

// ─── Post list (content DB) ───────────────────────────────────────────────────

func (p *ContentPool) ListPosts(ctx context.Context, authorID *uuid.UUID, after *time.Time, limit int) ([]Post, error) {
	q := `SELECT p.id, p.content, p.author_id,
	             COALESCE((SELECT COUNT(*) FROM posts r WHERE r.parent_id = p.id AND r.deleted_at IS NULL), 0) AS reply_count,
	             COALESCE((SELECT COUNT(*) FROM post_reactions pr WHERE pr.post_id = p.id), 0) AS like_count,
	             COALESCE((SELECT COUNT(*) FROM reports rp WHERE rp.post_id = p.id AND rp.resolved_at IS NULL), 0) AS open_reports,
	             p.created_at, p.deleted_at
	      FROM posts p
	      WHERE ($1::uuid IS NULL OR p.author_id = $1)
	        AND ($2::timestamptz IS NULL OR p.created_at < $2)
	      ORDER BY p.created_at DESC LIMIT $3`
	rows, err := p.pool.Query(ctx, q, authorID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.ID, &post.Content, &post.AuthorID,
			&post.ReplyCount, &post.LikeCount, &post.OpenReports,
			&post.CreatedAt, &post.DeletedAt); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// ─── Dashboard stats (content DB) ─────────────────────────────────────────────

func (p *ContentPool) CountNewPosts(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE created_at >= $1 AND deleted_at IS NULL AND parent_id IS NULL`, since).Scan(&n)
	return n, err
}

func (p *ContentPool) CountTotalPosts(ctx context.Context) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND parent_id IS NULL`).Scan(&n)
	return n, err
}

func (p *ContentPool) CountOpenReports(ctx context.Context) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT post_id) FROM reports WHERE resolved_at IS NULL`).Scan(&n)
	return n, err
}

func (p *ContentPool) CountUserPosts(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE author_id = $1 AND deleted_at IS NULL AND parent_id IS NULL`, userID).Scan(&n)
	return n, err
}

func (p *ContentPool) CountUserReports(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reports WHERE reporter_id = $1`, userID).Scan(&n)
	return n, err
}
