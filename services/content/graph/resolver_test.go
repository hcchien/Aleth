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

// ─── Series resolver tests ────────────────────────────────────────────────────

func TestSeriesResolver_Fields(t *testing.T) {
	now := time.Now()
	desc := "A description"
	s := db.Series{
		ID:          uuid.New(),
		BoardID:     uuid.New(),
		Title:       "Test Series",
		Description: &desc,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc := service.NewContentService(nil)
	sr := &SeriesResolver{series: s, svc: svc}

	if string(sr.ID()) != s.ID.String() {
		t.Errorf("ID: got %s want %s", sr.ID(), s.ID)
	}
	if string(sr.BoardId()) != s.BoardID.String() {
		t.Errorf("BoardId: got %s want %s", sr.BoardId(), s.BoardID)
	}
	if sr.Title() != "Test Series" {
		t.Errorf("Title: got %q", sr.Title())
	}
	if sr.Description() == nil || *sr.Description() != desc {
		t.Errorf("Description: got %v", sr.Description())
	}
	if sr.CreatedAt() == "" || sr.UpdatedAt() == "" {
		t.Error("expected non-empty timestamps")
	}
}

func TestSeriesResolver_NilDescription(t *testing.T) {
	s := db.Series{
		ID:          uuid.New(),
		BoardID:     uuid.New(),
		Title:       "No Desc",
		Description: nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	svc := service.NewContentService(nil)
	sr := &SeriesResolver{series: s, svc: svc}
	if sr.Description() != nil {
		t.Error("expected nil description")
	}
}

func TestArticleResolver_SeriesId_Nil(t *testing.T) {
	svc := service.NewContentService(nil)
	ar := &ArticleResolver{article: db.Article{
		ID: uuid.New(), BoardID: uuid.New(), AuthorID: uuid.New(),
		Title: "t", Slug: "s", Status: "draft", AccessPolicy: "public",
		SeriesID:  nil,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, svc: svc}
	if ar.SeriesId() != nil {
		t.Error("expected nil SeriesId when no series assigned")
	}
}

func TestArticleResolver_SeriesId_Set(t *testing.T) {
	svc := service.NewContentService(nil)
	sid := uuid.New()
	ar := &ArticleResolver{article: db.Article{
		ID: uuid.New(), BoardID: uuid.New(), AuthorID: uuid.New(),
		Title: "t", Slug: "s", Status: "draft", AccessPolicy: "public",
		SeriesID:  &sid,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, svc: svc}
	got := ar.SeriesId()
	if got == nil {
		t.Fatal("expected non-nil SeriesId")
	}
	if string(*got) != sid.String() {
		t.Errorf("SeriesId: got %s want %s", *got, sid)
	}
}

func TestSeriesResolverValidationBranches(t *testing.T) {
	r := &Resolver{}

	// Series query: invalid ID
	if _, err := r.Series(context.Background(), struct{ ID graphql.ID }{ID: "bad"}); err == nil {
		t.Error("expected error for invalid series ID")
	}

	// BoardSeries query: invalid boardId
	if _, err := r.BoardSeries(context.Background(), struct{ BoardId graphql.ID }{BoardId: "bad"}); err == nil {
		t.Error("expected error for invalid boardId")
	}

	// All mutations require auth
	if _, err := r.CreateSeries(context.Background(), struct{ Input CreateSeriesInput }{}); err == nil {
		t.Error("expected not authenticated for CreateSeries")
	}
	if _, err := r.UpdateSeries(context.Background(), struct {
		ID    graphql.ID
		Input UpdateSeriesInput
	}{}); err == nil {
		t.Error("expected not authenticated for UpdateSeries")
	}
	if _, err := r.DeleteSeries(context.Background(), struct{ ID graphql.ID }{}); err == nil {
		t.Error("expected not authenticated for DeleteSeries")
	}
	if _, err := r.AddArticleToSeries(context.Background(), struct {
		ArticleId graphql.ID
		SeriesId  graphql.ID
	}{}); err == nil {
		t.Error("expected not authenticated for AddArticleToSeries")
	}
	if _, err := r.RemoveArticleFromSeries(context.Background(), struct{ ArticleId graphql.ID }{}); err == nil {
		t.Error("expected not authenticated for RemoveArticleFromSeries")
	}
}
