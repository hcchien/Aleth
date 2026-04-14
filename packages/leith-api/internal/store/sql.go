package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SQLStore struct {
	db         *sql.DB
	driverName string
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{
		db:         db,
		driverName: "sqlite3",
	}
}

func NewSQLStoreWithDriver(db *sql.DB, driverName string) *SQLStore {
	return &SQLStore{
		db:         db,
		driverName: driverName,
	}
}

func OpenSQLStore(driverName, dsn string) (*SQLStore, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sql store: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sql store: %w", err)
	}
	store := NewSQLStoreWithDriver(db, driverName)
	if err := store.InitSchema(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLStore) InitSchema() error {
	return s.runMigrations()
}

func (s *SQLStore) CreateUser(user *User) error {
	if user == nil || user.DID == "" {
		return fmt.Errorf("invalid user")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	ph := s.placeholder
	insertUser := ""
	if s.isPostgres() {
		insertUser = fmt.Sprintf(`
INSERT INTO users (did, oauth_id, public_key, trust_tier, created_at)
VALUES (%s, %s, %s, %s, %s)
ON CONFLICT (did) DO NOTHING`, ph(1), ph(2), ph(3), ph(4), ph(5))
	} else {
		insertUser = fmt.Sprintf(`
INSERT OR IGNORE INTO users (did, oauth_id, public_key, trust_tier, created_at)
VALUES (%s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5))
	}

	if _, err := s.db.Exec(insertUser, user.DID, user.OAuthID, user.PublicKey, int(user.TrustTier), user.CreatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	getUser := fmt.Sprintf(`
SELECT id, did, oauth_id, public_key, trust_tier, created_at
FROM users
WHERE did = %s`, ph(1))
	var trustTier int
	if err := s.db.QueryRow(getUser, user.DID).Scan(
		&user.ID,
		&user.DID,
		&user.OAuthID,
		&user.PublicKey,
		&trustTier,
		&user.CreatedAt,
	); err != nil {
		return fmt.Errorf("load user after create: %w", err)
	}
	user.TrustTier = TrustTier(trustTier)
	return nil
}

func (s *SQLStore) GetUserByDID(did string) (*User, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, did, oauth_id, public_key, trust_tier, created_at
FROM users
WHERE did = %s`, ph(1))
	var (
		u         User
		trustTier int
	)
	err := s.db.QueryRow(q, did).Scan(
		&u.ID,
		&u.DID,
		&u.OAuthID,
		&u.PublicKey,
		&trustTier,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by did: %w", err)
	}
	u.TrustTier = TrustTier(trustTier)
	return &u, nil
}

func (s *SQLStore) CreatePost(post *Post) error {
	if post == nil {
		return fmt.Errorf("invalid post")
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}

	mediaHashes, err := json.Marshal(post.MediaHashes)
	if err != nil {
		return fmt.Errorf("marshal media hashes: %w", err)
	}

	ph := s.placeholder
	if s.isPostgres() {
		q := fmt.Sprintf(`
INSERT INTO posts (body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
		if err := s.db.QueryRow(
			q,
			post.Body,
			string(mediaHashes),
			nullInt64Value(post.ParentID),
			post.Timestamp,
			post.AuthorDID,
			post.Signature,
			post.VisibilityScore,
			post.CreatedAt,
		).Scan(&post.ID); err != nil {
			return fmt.Errorf("create post: %w", err)
		}
		return nil
	}

	q := fmt.Sprintf(`
INSERT INTO posts (body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
	res, err := s.db.Exec(
		q,
		post.Body,
		string(mediaHashes),
		nullInt64Value(post.ParentID),
		post.Timestamp,
		post.AuthorDID,
		post.Signature,
		post.VisibilityScore,
		post.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("fetch inserted post id: %w", err)
	}
	post.ID = id
	return nil
}

func (s *SQLStore) GetPosts(limit, offset int) ([]Post, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at
FROM posts
ORDER BY visibility_score DESC, created_at DESC
LIMIT %s OFFSET %s`, ph(1), ph(2))
	rows, err := s.db.Query(q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var (
			p             Post
			mediaHashesJS string
			parentID      sql.NullInt64
		)
		if err := rows.Scan(
			&p.ID,
			&p.Body,
			&mediaHashesJS,
			&parentID,
			&p.Timestamp,
			&p.AuthorDID,
			&p.Signature,
			&p.VisibilityScore,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		if err := json.Unmarshal([]byte(mediaHashesJS), &p.MediaHashes); err != nil {
			return nil, fmt.Errorf("decode media hashes: %w", err)
		}
		p.ParentID = parentID
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return posts, nil
}

func (s *SQLStore) UpdateVisibilityScore(postID int64, score float64) error {
	ph := s.placeholder
	q := fmt.Sprintf(`
UPDATE posts
SET visibility_score = visibility_score + %s
WHERE id = %s`, ph(1), ph(2))
	res, err := s.db.Exec(q, score, postID)
	if err != nil {
		return fmt.Errorf("update visibility score: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

func (s *SQLStore) CheckRateLimit(_ string, _ TrustTier) (bool, error) {
	return true, nil
}

func (s *SQLStore) isPostgres() bool {
	name := strings.ToLower(strings.TrimSpace(s.driverName))
	return strings.Contains(name, "pgx") || strings.Contains(name, "postgres")
}

func (s *SQLStore) placeholder(n int) string {
	if s.isPostgres() {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func nullInt64Value(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
