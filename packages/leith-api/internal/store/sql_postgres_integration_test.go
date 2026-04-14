//go:build integration

package store

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSQLStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("LEITH_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("set LEITH_PG_TEST_DSN to run PostgreSQL integration tests")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown driver") {
			t.Skip("postgres driver not registered; install and import github.com/jackc/pgx/v5/stdlib")
		}
		t.Fatalf("open pg connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping pg connection: %v", err)
	}

	s := NewSQLStoreWithDriver(db, "pgx")
	if err := s.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	if err := resetPostgresTables(db); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	t.Run("create and get user", func(t *testing.T) {
		user := &User{
			DID:       "did:vflow:pg-user-1",
			OAuthID:   "",
			PublicKey: []byte{1, 2, 3},
			TrustTier: L2_SOCIAL,
			CreatedAt: time.Now(),
		}
		if err := s.CreateUser(user); err != nil {
			t.Fatalf("create user: %v", err)
		}
		if user.ID == 0 {
			t.Fatalf("expected non-zero user id")
		}

		got, err := s.GetUserByDID(user.DID)
		if err != nil {
			t.Fatalf("get user: %v", err)
		}
		if got == nil {
			t.Fatalf("expected user")
		}
		if got.DID != user.DID || got.TrustTier != user.TrustTier {
			t.Fatalf("unexpected user payload: %+v", got)
		}
	})

	t.Run("create, update, and list posts", func(t *testing.T) {
		author := &User{
			DID:       "did:vflow:pg-author-1",
			TrustTier: L1_DEVICE,
			CreatedAt: time.Now(),
		}
		if err := s.CreateUser(author); err != nil {
			t.Fatalf("create author: %v", err)
		}

		p1 := &Post{
			Body:            "first",
			MediaHashes:     []string{"a"},
			Timestamp:       time.Now().Unix(),
			AuthorDID:       author.DID,
			Signature:       "sig1",
			VisibilityScore: 1,
			CreatedAt:       time.Now().Add(-1 * time.Minute),
		}
		if err := s.CreatePost(p1); err != nil {
			t.Fatalf("create p1: %v", err)
		}

		p2 := &Post{
			Body:            "second",
			MediaHashes:     []string{"b"},
			Timestamp:       time.Now().Unix(),
			AuthorDID:       author.DID,
			Signature:       "sig2",
			VisibilityScore: 1,
			CreatedAt:       time.Now(),
		}
		if err := s.CreatePost(p2); err != nil {
			t.Fatalf("create p2: %v", err)
		}

		if err := s.UpdateVisibilityScore(p1.ID, 5); err != nil {
			t.Fatalf("update visibility: %v", err)
		}

		posts, err := s.GetPosts(10, 0)
		if err != nil {
			t.Fatalf("get posts: %v", err)
		}
		if len(posts) < 2 {
			t.Fatalf("expected at least 2 posts, got %d", len(posts))
		}
		if posts[0].ID != p1.ID {
			t.Fatalf("expected boosted p1 first, got id=%d", posts[0].ID)
		}
	})
}

func resetPostgresTables(db *sql.DB) error {
	queries := []string{
		"TRUNCATE TABLE posts RESTART IDENTITY",
		"TRUNCATE TABLE users RESTART IDENTITY",
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q, err)
		}
	}
	return nil
}
