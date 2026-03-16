package graph

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	graphql "github.com/graph-gophers/graphql-go"

	"github.com/aleth/content/internal/db"
	"github.com/aleth/content/internal/service"
)

func TestContextHelpers(t *testing.T) {
	c := UserClaims{UserID: uuid.New().String(), Username: "u", TrustLevel: 1}
	ctx := WithClaims(context.Background(), c)
	got, ok := ClaimsFromContext(ctx)
	if !ok || got.UserID != c.UserID {
		t.Fatalf("claims mismatch")
	}
}

func TestConnectionHelpers(t *testing.T) {
	svc := service.NewContentService(nil)
	p1 := db.Post{ID: uuid.New()}
	p2 := db.Post{ID: uuid.New()}
	pc := newPostConnection([]db.Post{p1, p2}, 2, svc)
	if !pc.HasMore() || pc.NextCursor() == nil || len(pc.Items()) != 2 {
		t.Fatalf("unexpected post connection")
	}

	a1 := db.Article{ID: uuid.New()}
	ac := newArticleConnection([]db.Article{a1}, 1, svc)
	if !ac.HasMore() || ac.NextCursor() == nil || len(ac.Items()) != 1 {
		t.Fatalf("unexpected article connection")
	}
}

func TestTypeResolvers(t *testing.T) {
	svc := service.NewContentService(nil)
	now := time.Now()
	postID := uuid.New()
	authorID := uuid.New()
	post := db.Post{
		ID:        postID,
		AuthorID:  authorID,
		Content:   "hi",
		CreatedAt: now,
	}
	pr := &PostResolver{post: post, svc: svc}
	if string(pr.ID()) != postID.String() || pr.Content() != "hi" {
		t.Fatalf("unexpected post resolver values")
	}

	articleID := uuid.New()
	boardID := uuid.New()
	ar := &ArticleResolver{article: db.Article{
		ID:           articleID,
		BoardID:      boardID,
		AuthorID:     authorID,
		Title:        "t",
		Slug:         "s",
		Status:       "published",
		AccessPolicy: "public",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, svc: svc}
	if string(ar.ID()) != articleID.String() || ar.Title() != "t" {
		t.Fatalf("unexpected article resolver values")
	}
}

func TestParseOptionalUUID(t *testing.T) {
	if parseOptionalUUID("") != nil {
		t.Fatalf("expected nil for empty input")
	}
	if parseOptionalUUID("invalid") != nil {
		t.Fatalf("expected nil for invalid uuid")
	}
	id := uuid.New().String()
	got := parseOptionalUUID(id)
	if got == nil || got.String() != id {
		t.Fatalf("expected parsed uuid")
	}
}

func TestResolverValidationBranches(t *testing.T) {
	r := &Resolver{}

	if _, err := r.Post(context.Background(), struct{ ID graphql.ID }{ID: graphql.ID("invalid")}); err == nil {
		t.Fatalf("expected invalid post id error")
	}
	if _, err := r.Article(context.Background(), struct{ ID graphql.ID }{ID: graphql.ID("invalid")}); err == nil {
		t.Fatalf("expected invalid article id error")
	}
	if _, err := r.BoardByID(context.Background(), struct{ ID graphql.ID }{ID: graphql.ID("invalid")}); err == nil {
		t.Fatalf("expected invalid board id error")
	}
	if _, err := r.Board(context.Background(), struct{ OwnerID graphql.ID }{OwnerID: graphql.ID("invalid")}); err == nil {
		t.Fatalf("expected invalid owner id error")
	}

	if _, err := r.CreatePost(context.Background(), struct{ Input CreatePostInput }{}); err == nil {
		t.Fatalf("expected not authenticated")
	}
	if _, err := r.ReplyPost(context.Background(), struct {
		PostId graphql.ID
		Input  CreatePostInput
	}{}); err == nil {
		t.Fatalf("expected not authenticated")
	}
}
