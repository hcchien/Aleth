package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aleth/content/internal/db"
	"github.com/aleth/content/internal/events"
)

type fakeContentStore struct {
	getBoardByOwnerIDFn      func(context.Context, uuid.UUID) (db.Board, error)
	getBoardByIDFn           func(context.Context, uuid.UUID) (db.Board, error)
	createBoardFn            func(context.Context, uuid.UUID, string) (db.Board, error)
	updateBoardFn            func(context.Context, db.UpdateBoardParams) (db.Board, error)
	subscribeBoardFn         func(context.Context, uuid.UUID, uuid.UUID) error
	unsubscribeBoardFn       func(context.Context, uuid.UUID, uuid.UUID) error
	isSubscribedFn           func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	countSubscribersFn       func(context.Context, uuid.UUID) (int64, error)
	createPostFn             func(context.Context, db.CreatePostParams) (db.Post, error)
	getPostByIDFn            func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error)
	listPostsFn              func(context.Context, db.ListPostsParams) ([]db.Post, error)
	listPostRepliesFn        func(context.Context, db.ListPostRepliesParams) ([]db.Post, error)
	softDeletePostFn         func(context.Context, uuid.UUID, uuid.UUID) error
	likePostFn               func(context.Context, uuid.UUID, uuid.UUID) error
	reactPostFn              func(context.Context, uuid.UUID, uuid.UUID, string, *string) error
	unlikePostFn             func(context.Context, uuid.UUID, uuid.UUID) error
	listPostReactionCountsFn func(context.Context, uuid.UUID) ([]db.ReactionCount, error)
	updatePostSignatureFn    func(context.Context, uuid.UUID, []byte) error
	resharePostFn            func(context.Context, db.ResharePostParams) (db.Post, error)
	createNoteFn             func(context.Context, db.CreateNoteParams) (db.Post, error)
	listNotesFn              func(context.Context, db.ListNotesParams) ([]db.Post, error)
	createArticleFn          func(context.Context, db.CreateArticleParams) (db.Article, error)
	getArticleByIDFn         func(context.Context, uuid.UUID) (db.Article, error)
	updateArticleFn          func(context.Context, db.UpdateArticleParams) (db.Article, error)
	publishArticleFn         func(context.Context, uuid.UUID) (db.Article, error)
	deleteArticleFn          func(context.Context, uuid.UUID, uuid.UUID) error
	listBoardArticlesFn      func(context.Context, db.ListArticlesParams) ([]db.Article, error)
	updateArticleSignatureFn func(context.Context, uuid.UUID, []byte) error
	createArticleCommentFn   func(context.Context, db.CreateArticleCommentParams) (db.ArticleComment, error)
	listArticleCommentsFn    func(context.Context, uuid.UUID, int) ([]db.ArticleComment, error)
	listCommentRepliesFn     func(context.Context, uuid.UUID, int) ([]db.ArticleComment, error)
	updateBoardVcPolicyFn    func(context.Context, db.UpdateBoardVcPolicyParams) (db.Board, error)
}

func newFakeContentStore() *fakeContentStore {
	return &fakeContentStore{
		getBoardByOwnerIDFn: func(context.Context, uuid.UUID) (db.Board, error) { return db.Board{}, pgx.ErrNoRows },
		getBoardByIDFn:      func(context.Context, uuid.UUID) (db.Board, error) { return db.Board{}, pgx.ErrNoRows },
		createBoardFn: func(_ context.Context, ownerID uuid.UUID, name string) (db.Board, error) {
			return db.Board{ID: uuid.New(), OwnerID: ownerID, Name: name, DefaultAccess: "public", CreatedAt: time.Now()}, nil
		},
		updateBoardFn: func(_ context.Context, p db.UpdateBoardParams) (db.Board, error) {
			name := "board"
			if p.Name != nil {
				name = *p.Name
			}
			return db.Board{ID: p.ID, Name: name, DefaultAccess: "public", CreatedAt: time.Now()}, nil
		},
		subscribeBoardFn:   func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		unsubscribeBoardFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		isSubscribedFn:     func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
		countSubscribersFn: func(context.Context, uuid.UUID) (int64, error) { return 3, nil },
		createPostFn: func(_ context.Context, p db.CreatePostParams) (db.Post, error) {
			return db.Post{ID: uuid.New(), AuthorID: p.AuthorID, ParentID: p.ParentID, RootID: p.RootID, Content: p.Content, CreatedAt: time.Now()}, nil
		},
		getPostByIDFn: func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
			return db.Post{ID: id, AuthorID: uuid.New(), Content: "x", CreatedAt: time.Now()}, nil
		},
		listPostsFn: func(context.Context, db.ListPostsParams) ([]db.Post, error) {
			return []db.Post{{ID: uuid.New(), Content: "p"}}, nil
		},
		listPostRepliesFn: func(_ context.Context, p db.ListPostRepliesParams) ([]db.Post, error) {
			return []db.Post{
				{ID: uuid.New(), ParentID: &p.ParentID, Content: "reply 1", CreatedAt: time.Now()},
				{ID: uuid.New(), ParentID: &p.ParentID, Content: "reply 2", CreatedAt: time.Now()},
			}, nil
		},
		softDeletePostFn:         func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		likePostFn:               func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		reactPostFn:              func(context.Context, uuid.UUID, uuid.UUID, string, *string) error { return nil },
		unlikePostFn:             func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		listPostReactionCountsFn: func(context.Context, uuid.UUID) ([]db.ReactionCount, error) { return nil, nil },
		updatePostSignatureFn: func(context.Context, uuid.UUID, []byte) error { return nil },
		resharePostFn: func(_ context.Context, p db.ResharePostParams) (db.Post, error) {
			resharedID := p.ResharedFromID
			return db.Post{ID: uuid.New(), AuthorID: p.AuthorID, Kind: "post", Content: p.Content, ResharedFromID: &resharedID, CreatedAt: time.Now()}, nil
		},
		createNoteFn: func(_ context.Context, p db.CreateNoteParams) (db.Post, error) {
			title := p.NoteTitle
			return db.Post{ID: uuid.New(), AuthorID: p.AuthorID, Kind: "note", NoteTitle: &title, Content: p.Content, CreatedAt: time.Now()}, nil
		},
		listNotesFn: func(context.Context, db.ListNotesParams) ([]db.Post, error) {
			title := "Test Note"
			return []db.Post{{ID: uuid.New(), Kind: "note", NoteTitle: &title, Content: "<p>body</p>"}}, nil
		},
		createArticleFn: func(_ context.Context, p db.CreateArticleParams) (db.Article, error) {
			return db.Article{ID: uuid.New(), BoardID: p.BoardID, AuthorID: p.AuthorID, Title: p.Title, Slug: p.Slug, AccessPolicy: p.AccessPolicy, Status: "draft"}, nil
		},
		getArticleByIDFn: func(_ context.Context, id uuid.UUID) (db.Article, error) {
			return db.Article{ID: id, AuthorID: uuid.New(), Status: "published", AccessPolicy: "public", MinTrustLevel: 0}, nil
		},
		updateArticleFn: func(_ context.Context, p db.UpdateArticleParams) (db.Article, error) {
			return db.Article{ID: p.ID, Title: "updated", Status: "draft", AccessPolicy: "public"}, nil
		},
		publishArticleFn: func(_ context.Context, id uuid.UUID) (db.Article, error) {
			return db.Article{ID: id, Status: "published"}, nil
		},
		deleteArticleFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		listBoardArticlesFn: func(context.Context, db.ListArticlesParams) ([]db.Article, error) {
			return []db.Article{{ID: uuid.New(), Status: "published"}}, nil
		},
		updateArticleSignatureFn: func(context.Context, uuid.UUID, []byte) error { return nil },
		createArticleCommentFn: func(_ context.Context, p db.CreateArticleCommentParams) (db.ArticleComment, error) {
			return db.ArticleComment{ID: uuid.New(), ArticleID: p.ArticleID, AuthorID: p.AuthorID, Content: p.Content, CreatedAt: time.Now()}, nil
		},
		listArticleCommentsFn: func(context.Context, uuid.UUID, int) ([]db.ArticleComment, error) {
			return nil, nil
		},
		listCommentRepliesFn: func(_ context.Context, parentID uuid.UUID, _ int) ([]db.ArticleComment, error) {
			return []db.ArticleComment{
				{ID: uuid.New(), ArticleID: uuid.New(), AuthorID: uuid.New(), ParentID: &parentID, Content: "reply 1", CreatedAt: time.Now()},
				{ID: uuid.New(), ArticleID: uuid.New(), AuthorID: uuid.New(), ParentID: &parentID, Content: "reply 2", CreatedAt: time.Now()},
			}, nil
		},
		updateBoardVcPolicyFn: func(_ context.Context, p db.UpdateBoardVcPolicyParams) (db.Board, error) {
			return db.Board{ID: p.ID}, nil
		},
	}
}

func (f *fakeContentStore) GetBoardByOwnerID(ctx context.Context, ownerID uuid.UUID) (db.Board, error) {
	return f.getBoardByOwnerIDFn(ctx, ownerID)
}
func (f *fakeContentStore) GetBoardByID(ctx context.Context, id uuid.UUID) (db.Board, error) {
	return f.getBoardByIDFn(ctx, id)
}
func (f *fakeContentStore) CreateBoard(ctx context.Context, ownerID uuid.UUID, name string) (db.Board, error) {
	return f.createBoardFn(ctx, ownerID, name)
}
func (f *fakeContentStore) UpdateBoard(ctx context.Context, p db.UpdateBoardParams) (db.Board, error) {
	return f.updateBoardFn(ctx, p)
}
func (f *fakeContentStore) SubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error {
	return f.subscribeBoardFn(ctx, boardID, userID)
}
func (f *fakeContentStore) UnsubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error {
	return f.unsubscribeBoardFn(ctx, boardID, userID)
}
func (f *fakeContentStore) IsSubscribed(ctx context.Context, boardID, userID uuid.UUID) (bool, error) {
	return f.isSubscribedFn(ctx, boardID, userID)
}
func (f *fakeContentStore) CountSubscribers(ctx context.Context, boardID uuid.UUID) (int64, error) {
	return f.countSubscribersFn(ctx, boardID)
}
func (f *fakeContentStore) CreatePost(ctx context.Context, p db.CreatePostParams) (db.Post, error) {
	return f.createPostFn(ctx, p)
}
func (f *fakeContentStore) GetPostByID(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (db.Post, error) {
	return f.getPostByIDFn(ctx, id, viewerID)
}
func (f *fakeContentStore) ListPosts(ctx context.Context, p db.ListPostsParams) ([]db.Post, error) {
	return f.listPostsFn(ctx, p)
}
func (f *fakeContentStore) ListPostReplies(ctx context.Context, p db.ListPostRepliesParams) ([]db.Post, error) {
	return f.listPostRepliesFn(ctx, p)
}
func (f *fakeContentStore) SoftDeletePost(ctx context.Context, id, authorID uuid.UUID) error {
	return f.softDeletePostFn(ctx, id, authorID)
}
func (f *fakeContentStore) LikePost(ctx context.Context, postID, userID uuid.UUID) error {
	return f.likePostFn(ctx, postID, userID)
}
func (f *fakeContentStore) ReactPost(ctx context.Context, postID, userID uuid.UUID, emotion string, sourceIP *string) error {
	return f.reactPostFn(ctx, postID, userID, emotion, sourceIP)
}
func (f *fakeContentStore) UnlikePost(ctx context.Context, postID, userID uuid.UUID) error {
	return f.unlikePostFn(ctx, postID, userID)
}
func (f *fakeContentStore) ListPostReactionCounts(ctx context.Context, postID uuid.UUID) ([]db.ReactionCount, error) {
	return f.listPostReactionCountsFn(ctx, postID)
}
func (f *fakeContentStore) UpdatePostSignature(ctx context.Context, id uuid.UUID, signature []byte) error {
	return f.updatePostSignatureFn(ctx, id, signature)
}
func (f *fakeContentStore) ResharePost(ctx context.Context, p db.ResharePostParams) (db.Post, error) {
	return f.resharePostFn(ctx, p)
}
func (f *fakeContentStore) CreateNote(ctx context.Context, p db.CreateNoteParams) (db.Post, error) {
	return f.createNoteFn(ctx, p)
}
func (f *fakeContentStore) ListNotes(ctx context.Context, p db.ListNotesParams) ([]db.Post, error) {
	return f.listNotesFn(ctx, p)
}
func (f *fakeContentStore) CreateArticle(ctx context.Context, p db.CreateArticleParams) (db.Article, error) {
	return f.createArticleFn(ctx, p)
}
func (f *fakeContentStore) GetArticleByID(ctx context.Context, id uuid.UUID) (db.Article, error) {
	return f.getArticleByIDFn(ctx, id)
}
func (f *fakeContentStore) UpdateArticle(ctx context.Context, p db.UpdateArticleParams) (db.Article, error) {
	return f.updateArticleFn(ctx, p)
}
func (f *fakeContentStore) PublishArticle(ctx context.Context, id uuid.UUID) (db.Article, error) {
	return f.publishArticleFn(ctx, id)
}
func (f *fakeContentStore) DeleteArticle(ctx context.Context, id, authorID uuid.UUID) error {
	return f.deleteArticleFn(ctx, id, authorID)
}
func (f *fakeContentStore) ListBoardArticles(ctx context.Context, p db.ListArticlesParams) ([]db.Article, error) {
	return f.listBoardArticlesFn(ctx, p)
}
func (f *fakeContentStore) UpdateArticleSignature(ctx context.Context, id uuid.UUID, signature []byte) error {
	return f.updateArticleSignatureFn(ctx, id, signature)
}
func (f *fakeContentStore) CreateArticleComment(ctx context.Context, params db.CreateArticleCommentParams) (db.ArticleComment, error) {
	return f.createArticleCommentFn(ctx, params)
}
func (f *fakeContentStore) ListArticleComments(ctx context.Context, articleID uuid.UUID, limit int) ([]db.ArticleComment, error) {
	return f.listArticleCommentsFn(ctx, articleID, limit)
}
func (f *fakeContentStore) ListCommentReplies(ctx context.Context, parentID uuid.UUID, limit int) ([]db.ArticleComment, error) {
	return f.listCommentRepliesFn(ctx, parentID, limit)
}
func (f *fakeContentStore) UpdateBoardVcPolicy(ctx context.Context, p db.UpdateBoardVcPolicyParams) (db.Board, error) {
	return f.updateBoardVcPolicyFn(ctx, p)
}

func TestBoardAndPostFlows(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	owner := uuid.New()

	board, err := s.GetOrCreateBoard(ctx, owner, "My Board")
	if err != nil {
		t.Fatalf("GetOrCreateBoard error: %v", err)
	}
	if board.OwnerID != owner {
		t.Fatalf("owner mismatch")
	}

	if _, err := s.GetBoardByOwnerID(ctx, owner); err != nil {
		t.Fatalf("GetBoardByOwnerID error: %v", err)
	}
	if _, err := s.GetBoardByID(ctx, board.ID); err != nil {
		t.Fatalf("GetBoardByID error: %v", err)
	}

	if _, err := s.CreatePost(ctx, owner, "  hello ", 0); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}
	if _, err := s.ListPosts(ctx, nil, 20, nil); err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}
	if err := s.DeletePost(ctx, uuid.New(), owner); err != nil {
		t.Fatalf("DeletePost error: %v", err)
	}
	if err := s.LikePost(ctx, uuid.New(), owner); err != nil {
		t.Fatalf("LikePost error: %v", err)
	}
	if err := s.UnlikePost(ctx, uuid.New(), owner); err != nil {
		t.Fatalf("UnlikePost error: %v", err)
	}
	if err := s.ReactPost(ctx, uuid.New(), owner, "love", nil); err != nil {
		t.Fatalf("ReactPost error: %v", err)
	}
	if err := s.UnreactPost(ctx, uuid.New(), owner); err != nil {
		t.Fatalf("UnreactPost error: %v", err)
	}
}

func TestBoardValidationBranches(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	owner := uuid.New()

	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{}, pgx.ErrNoRows
	}
	if _, err := s.UpdateBoardSettings(ctx, owner, UpdateBoardInput{}); err == nil {
		t.Fatalf("expected board not found")
	}

	policy := "private"
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{ID: uuid.New(), OwnerID: owner}, nil
	}
	if _, err := s.UpdateBoardSettings(ctx, owner, UpdateBoardInput{DefaultAccess: &policy}); err == nil {
		t.Fatalf("expected invalid access policy")
	}
}

func TestValidationAndReplyBranches(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	uid := uuid.New()

	if _, err := s.CreatePost(ctx, uid, "   ", 0); err == nil {
		t.Fatalf("expected empty content error")
	}
	if _, err := s.CreatePost(ctx, uid, strings.Repeat("a", 501), 0); err == nil {
		t.Fatalf("expected max length error")
	}

	postID := uuid.New()
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{}, pgx.ErrNoRows
	}
	if _, err := s.ReplyPost(ctx, uid, postID, "reply"); err == nil {
		t.Fatalf("expected not found error")
	}

	now := time.Now()
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{ID: postID, DeletedAt: &now}, nil
	}
	if _, err := s.ReplyPost(ctx, uid, postID, "reply"); err == nil {
		t.Fatalf("expected deleted parent error")
	}
}

func TestArticleFlows(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	author := uuid.New()

	_, err := s.CreateArticle(ctx, author, CreateArticleInput{Title: "  ", AccessPolicy: "public"})
	if err == nil {
		t.Fatalf("expected title error")
	}

	article, err := s.CreateArticle(ctx, author, CreateArticleInput{Title: "Hello World", AccessPolicy: "public"})
	if err != nil {
		t.Fatalf("CreateArticle error: %v", err)
	}
	if article.Slug == "" {
		t.Fatalf("expected slug")
	}

	st.getArticleByIDFn = func(_ context.Context, id uuid.UUID) (db.Article, error) {
		return db.Article{ID: id, AuthorID: author, Status: "published", AccessPolicy: "public", MinTrustLevel: 0}, nil
	}
	if _, err := s.UpdateArticle(ctx, article.ID, author, UpdateArticleInput{}); err != nil {
		t.Fatalf("UpdateArticle error: %v", err)
	}
	if _, err := s.PublishArticle(ctx, article.ID, author); err != nil {
		t.Fatalf("PublishArticle error: %v", err)
	}
	if _, err := s.GetArticle(ctx, article.ID, 0); err != nil {
		t.Fatalf("GetArticle error: %v", err)
	}
	if err := s.DeleteArticle(ctx, article.ID, author); err != nil {
		t.Fatalf("DeleteArticle error: %v", err)
	}
	if _, err := s.ListBoardArticles(ctx, author, nil, 20, false); err != nil {
		t.Fatalf("ListBoardArticles error: %v", err)
	}
}

func TestArticleValidationBranches(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	author := uuid.New()
	articleID := uuid.New()

	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{}, pgx.ErrNoRows
	}
	if _, err := s.UpdateArticle(ctx, articleID, author, UpdateArticleInput{}); err == nil {
		t.Fatalf("expected article not found")
	}
	if _, err := s.PublishArticle(ctx, articleID, author); err == nil {
		t.Fatalf("expected article not found")
	}

	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, AuthorID: uuid.New()}, nil
	}
	if _, err := s.UpdateArticle(ctx, articleID, author, UpdateArticleInput{}); err == nil {
		t.Fatalf("expected not authorized")
	}
	if _, err := s.PublishArticle(ctx, articleID, author); err == nil {
		t.Fatalf("expected not authorized")
	}

	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, AuthorID: author}, nil
	}
	status := "invalid"
	if _, err := s.UpdateArticle(ctx, articleID, author, UpdateArticleInput{Status: &status}); err == nil {
		t.Fatalf("expected invalid status")
	}

	article := db.Article{
		ID:            articleID,
		AuthorID:      author,
		Status:        "draft",
		AccessPolicy:  "members",
		MinTrustLevel: 3,
	}
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return article, nil
	}
	got, err := s.GetArticle(ctx, articleID, 2)
	if err != nil {
		t.Fatalf("GetArticle draft should not error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for draft article")
	}

	article.Status = "published"
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return article, nil
	}
	if _, err := s.GetArticle(ctx, articleID, -1); err == nil {
		t.Fatalf("expected members-only error")
	}
	article.AccessPolicy = "public"
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return article, nil
	}
	if _, err := s.GetArticle(ctx, articleID, 1); err == nil {
		t.Fatalf("expected insufficient trust error")
	}

	// Public article with min_trust_level=0 is accessible to unauthenticated users.
	article.MinTrustLevel = 0
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return article, nil
	}
	if got, err := s.GetArticle(ctx, articleID, -1); err != nil || got == nil {
		t.Fatalf("expected unauthenticated user to read public article with min_trust_level=0, got err=%v got=%v", err, got)
	}
}

func TestHelperFunctions(t *testing.T) {
	if got := slugify("  Hello, World!  "); got != "hello-world" {
		t.Fatalf("unexpected slug %q", got)
	}
	if err := validateAccessPolicy("private"); err == nil {
		t.Fatalf("expected invalid access policy error")
	}
	if err := validateArticleStatus("x"); err == nil {
		t.Fatalf("expected invalid status error")
	}
	if isSupportedEmotion("xxx") {
		t.Fatalf("unexpected supported emotion")
	}
}

func TestContentSigningAndVerification(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	s.SetSigningSecret("signing-secret")

	author := uuid.New()
	post, err := s.CreatePost(context.Background(), author, "hello signed world", 0)
	if err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}
	if len(post.Signature) == 0 {
		t.Fatalf("expected post signature")
	}
	info := s.BuildPostSignatureInfo(post)
	if !info.IsSigned || !info.IsVerified {
		t.Fatalf("expected signed+verified post info, got %+v", info)
	}
}

func TestContentSigningFailsAfterTamper(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	s.SetSigningSecret("signing-secret")

	author := uuid.New()
	post, err := s.CreatePost(context.Background(), author, "original", 0)
	if err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}
	post.Content = "tampered"
	info := s.BuildPostSignatureInfo(post)
	if info.IsVerified {
		t.Fatalf("expected verification failure after content tamper")
	}
}

func TestReactPostValidation(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	err := s.ReactPost(context.Background(), uuid.New(), uuid.New(), "invalid-emotion", nil)
	if err == nil {
		t.Fatalf("expected unsupported emotion error")
	}
}

func TestArticleCommentFlow(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	author := uuid.New()
	articleID := uuid.New()

	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "public", MinTrustLevel: 0}, nil
	}
	c, err := s.CreateArticleComment(context.Background(), articleID, author, "hello comment", 0, nil)
	if err != nil {
		t.Fatalf("CreateArticleComment error: %v", err)
	}
	if c.Content == "" {
		t.Fatalf("expected comment content")
	}

	if _, err := s.ListArticleComments(context.Background(), articleID, 20); err != nil {
		t.Fatalf("ListArticleComments error: %v", err)
	}
}

func TestCreateArticleComment_ValidationBranches(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	articleID := uuid.New()
	author := uuid.New()

	// Empty content
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "public"}, nil
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "  ", 0, nil); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected 'cannot be empty' error, got %v", err)
	}

	// Too long content
	if _, err := s.CreateArticleComment(ctx, articleID, author, strings.Repeat("x", 1001), 0, nil); err == nil || !strings.Contains(err.Error(), "exceeds 1000") {
		t.Fatalf("expected 'exceeds 1000' error, got %v", err)
	}

	// Article not found
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{}, pgx.ErrNoRows
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "hello", 0, nil); err == nil || !strings.Contains(err.Error(), "article not found") {
		t.Fatalf("expected 'article not found' error, got %v", err)
	}

	// Draft article not available for comments
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "draft", AccessPolicy: "public"}, nil
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "hello", 0, nil); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected 'not available' error, got %v", err)
	}

	// Members-only policy, unauthenticated viewer (trust level -1)
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "members", MinTrustLevel: 0}, nil
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "hello", -1, nil); err == nil || !strings.Contains(err.Error(), "members-only") {
		t.Fatalf("expected 'members-only' error, got %v", err)
	}

	// Insufficient trust level
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "public", MinTrustLevel: 5}, nil
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "hello", 2, nil); err == nil || !strings.Contains(err.Error(), "insufficient trust") {
		t.Fatalf("expected 'insufficient trust level' error, got %v", err)
	}

	// Members-only but authenticated (trust level 0) should succeed
	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "members", MinTrustLevel: 0}, nil
	}
	if _, err := s.CreateArticleComment(ctx, articleID, author, "hello", 0, nil); err != nil {
		t.Fatalf("unexpected error for authenticated members article: %v", err)
	}
}

func TestSignatureNonFatal(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	s.SetSigningSecret("test-key")

	// Make UpdatePostSignature fail
	st.updatePostSignatureFn = func(context.Context, uuid.UUID, []byte) error {
		return errors.New("db unavailable")
	}
	// CreatePost should succeed even if signature update fails
	post, err := s.CreatePost(context.Background(), uuid.New(), "content", 0)
	if err != nil {
		t.Fatalf("expected no error on non-fatal signature failure, got %v", err)
	}
	if post.ID == uuid.Nil {
		t.Error("expected valid post ID")
	}

	// Same for articles
	st.updateArticleSignatureFn = func(context.Context, uuid.UUID, []byte) error {
		return errors.New("db unavailable")
	}
	ctx := context.Background()
	owner := uuid.New()
	boardID := uuid.New()
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{ID: boardID, OwnerID: owner}, nil
	}
	article, err := s.CreateArticle(ctx, owner, CreateArticleInput{Title: "Test", AccessPolicy: "public"})
	if err != nil {
		t.Fatalf("expected no error on non-fatal article signature failure, got %v", err)
	}
	if article.ID == uuid.Nil {
		t.Error("expected valid article ID")
	}
}

func TestGetCommentReplies(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	parentID := uuid.New()

	// Happy path: returns stub replies
	replies, err := s.GetCommentReplies(ctx, parentID, 50)
	if err != nil {
		t.Fatalf("GetCommentReplies error: %v", err)
	}
	if len(replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(replies))
	}
	for _, r := range replies {
		if r.ParentID == nil || *r.ParentID != parentID {
			t.Errorf("reply ParentID mismatch: got %v", r.ParentID)
		}
	}

	// DB error propagation
	st.listCommentRepliesFn = func(context.Context, uuid.UUID, int) ([]db.ArticleComment, error) {
		return nil, errors.New("db down")
	}
	if _, err := s.GetCommentReplies(ctx, parentID, 50); err == nil {
		t.Fatal("expected error from DB, got nil")
	}

	// Empty result
	st.listCommentRepliesFn = func(context.Context, uuid.UUID, int) ([]db.ArticleComment, error) {
		return nil, nil
	}
	empty, err := s.GetCommentReplies(ctx, parentID, 50)
	if err != nil {
		t.Fatalf("unexpected error on empty result: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 replies, got %d", len(empty))
	}
}

func TestCreateArticleComment_Threading(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	author := uuid.New()
	articleID := uuid.New()
	parentID := uuid.New()

	st.getArticleByIDFn = func(context.Context, uuid.UUID) (db.Article, error) {
		return db.Article{ID: articleID, Status: "published", AccessPolicy: "public", MinTrustLevel: 0}, nil
	}

	// Successfully create a reply with parentCommentID
	st.createArticleCommentFn = func(_ context.Context, p db.CreateArticleCommentParams) (db.ArticleComment, error) {
		return db.ArticleComment{
			ID:        uuid.New(),
			ArticleID: p.ArticleID,
			AuthorID:  p.AuthorID,
			ParentID:  p.ParentID,
			Content:   p.Content,
			CreatedAt: time.Now(),
		}, nil
	}
	reply, err := s.CreateArticleComment(ctx, articleID, author, "threaded reply", 0, &parentID)
	if err != nil {
		t.Fatalf("CreateArticleComment reply error: %v", err)
	}
	if reply.ParentID == nil || *reply.ParentID != parentID {
		t.Fatalf("expected ParentID %v, got %v", parentID, reply.ParentID)
	}

	// Top-level comment still works (nil parentCommentID)
	top, err := s.CreateArticleComment(ctx, articleID, author, "top-level", 0, nil)
	if err != nil {
		t.Fatalf("CreateArticleComment top-level error: %v", err)
	}
	if top.ParentID != nil {
		t.Fatalf("expected nil ParentID for top-level comment, got %v", top.ParentID)
	}
}

func TestBoardSubscriptions(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	owner := uuid.New()
	boardID := uuid.New()
	subscriber := uuid.New()

	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{ID: boardID, OwnerID: owner}, nil
	}

	// SubscribeBoard
	if err := s.SubscribeBoard(ctx, owner, subscriber); err != nil {
		t.Fatalf("SubscribeBoard error: %v", err)
	}

	// SubscribeBoard — board not found
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{}, pgx.ErrNoRows
	}
	if err := s.SubscribeBoard(ctx, owner, subscriber); err == nil || !strings.Contains(err.Error(), "board not found") {
		t.Fatalf("expected 'board not found', got %v", err)
	}

	// UnsubscribeBoard
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{ID: boardID, OwnerID: owner}, nil
	}
	if err := s.UnsubscribeBoard(ctx, owner, subscriber); err != nil {
		t.Fatalf("UnsubscribeBoard error: %v", err)
	}

	// UnsubscribeBoard — board not found
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{}, pgx.ErrNoRows
	}
	if err := s.UnsubscribeBoard(ctx, owner, subscriber); err == nil || !strings.Contains(err.Error(), "board not found") {
		t.Fatalf("expected 'board not found', got %v", err)
	}

	// IsSubscribedToBoard
	ok, err := s.IsSubscribedToBoard(ctx, boardID, subscriber)
	if err != nil {
		t.Fatalf("IsSubscribedToBoard error: %v", err)
	}
	if !ok {
		t.Fatal("expected subscribed=true")
	}

	// CountBoardSubscribers
	count, err := s.CountBoardSubscribers(ctx, boardID)
	if err != nil {
		t.Fatalf("CountBoardSubscribers error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 subscribers, got %d", count)
	}
}

func TestGetPostAndReplies(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	postID := uuid.New()

	// GetPost — found
	post, err := s.GetPost(ctx, postID, nil)
	if err != nil {
		t.Fatalf("GetPost error: %v", err)
	}
	if post == nil || post.ID != postID {
		t.Fatalf("expected post with ID %v", postID)
	}

	// GetPost — not found returns nil, nil
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{}, pgx.ErrNoRows
	}
	missing, err := s.GetPost(ctx, postID, nil)
	if err != nil {
		t.Fatalf("GetPost not-found should return nil error, got %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing post")
	}

	// GetPost — other DB error
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{}, errors.New("db failure")
	}
	if _, err := s.GetPost(ctx, postID, nil); err == nil {
		t.Fatal("expected DB error propagation")
	}

	// GetPostReplies — happy path
	st.listPostRepliesFn = func(_ context.Context, p db.ListPostRepliesParams) ([]db.Post, error) {
		return []db.Post{{ID: uuid.New(), ParentID: &p.ParentID, Content: "r"}}, nil
	}
	replies, err := s.GetPostReplies(ctx, postID, nil, 20)
	if err != nil {
		t.Fatalf("GetPostReplies error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected at least one reply")
	}

	// ListPostReactionCounts
	counts, err := s.ListPostReactionCounts(ctx, postID)
	if err != nil {
		t.Fatalf("ListPostReactionCounts error: %v", err)
	}
	_ = counts

	// ReplyPost — happy path (parent exists, not deleted)
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{ID: postID, Content: "parent"}, nil
	}
	reply, err := s.ReplyPost(ctx, uuid.New(), postID, "child reply")
	if err != nil {
		t.Fatalf("ReplyPost happy path error: %v", err)
	}
	if reply.ParentID == nil || *reply.ParentID != postID {
		t.Fatalf("expected ParentID %v, got %v", postID, reply.ParentID)
	}
}

func TestBuildArticleSignatureInfo(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	s.SetSigningSecret("sig-key")

	owner := uuid.New()
	boardID := uuid.New()
	st.getBoardByOwnerIDFn = func(context.Context, uuid.UUID) (db.Board, error) {
		return db.Board{ID: boardID, OwnerID: owner}, nil
	}

	article, err := s.CreateArticle(context.Background(), owner, CreateArticleInput{
		Title:        "Signed Article",
		AccessPolicy: "public",
	})
	if err != nil {
		t.Fatalf("CreateArticle error: %v", err)
	}

	info := s.BuildArticleSignatureInfo(article)
	if !info.IsSigned {
		t.Fatal("expected article to be signed")
	}
	if !info.IsVerified {
		t.Fatalf("expected article signature to verify, got: %s", info.Explanation)
	}

	// Tamper with content — should fail verification
	article.Title = "tampered"
	tampered := s.BuildArticleSignatureInfo(article)
	if tampered.IsVerified {
		t.Fatal("expected tampered article to fail verification")
	}
}

func TestCreateNote(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	author := uuid.New()

	// Success
	note, err := s.CreateNote(ctx, author, CreateNoteInput{
		Content:   "<p>Hello Notes</p>",
		NoteTitle: "My First Note",
	})
	if err != nil {
		t.Fatalf("CreateNote error: %v", err)
	}
	if note.Kind != "note" {
		t.Fatalf("expected kind='note', got %q", note.Kind)
	}
	if note.NoteTitle == nil || *note.NoteTitle != "My First Note" {
		t.Fatalf("unexpected NoteTitle: %v", note.NoteTitle)
	}

	// Empty title → error
	if _, err := s.CreateNote(ctx, author, CreateNoteInput{Content: "<p>x</p>", NoteTitle: "   "}); err == nil {
		t.Fatal("expected empty title error")
	}

	// Empty content → error
	if _, err := s.CreateNote(ctx, author, CreateNoteInput{Content: "", NoteTitle: "Title"}); err == nil {
		t.Fatal("expected empty content error")
	}
}

func TestListNotes(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()

	notes, err := s.ListNotes(ctx, nil, 10, nil)
	if err != nil {
		t.Fatalf("ListNotes error: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected at least one note")
	}
	for _, n := range notes {
		if n.Kind != "note" {
			t.Fatalf("expected kind='note', got %q", n.Kind)
		}
	}
}

func TestResharePost(t *testing.T) {
	st := newFakeContentStore()
	s := NewContentService(st)
	ctx := context.Background()
	author := uuid.New()
	originalID := uuid.New()

	// Success: reshare with comment
	post, err := s.ResharePost(ctx, author, originalID, "my comment")
	if err != nil {
		t.Fatalf("ResharePost error: %v", err)
	}
	if post.ResharedFromID == nil || *post.ResharedFromID != originalID {
		t.Fatalf("expected ResharedFromID=%s, got %v", originalID, post.ResharedFromID)
	}

	// Success: reshare without comment (empty string)
	post2, err := s.ResharePost(ctx, author, originalID, "")
	if err != nil {
		t.Fatalf("ResharePost (no comment) error: %v", err)
	}
	if post2.ResharedFromID == nil {
		t.Fatal("expected ResharedFromID to be set")
	}

	// Error: reshare of a reshare
	resharedID := uuid.New()
	st.getPostByIDFn = func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return db.Post{ID: id, AuthorID: uuid.New(), Content: "x", ResharedFromID: &resharedID, CreatedAt: time.Now()}, nil
	}
	if _, err := s.ResharePost(ctx, author, originalID, ""); err == nil {
		t.Fatal("expected error when resharing a reshare")
	} else if !strings.Contains(err.Error(), "cannot reshare a reshare") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Error: original post not found
	st.getPostByIDFn = func(context.Context, uuid.UUID, *uuid.UUID) (db.Post, error) {
		return db.Post{}, errors.New("not found")
	}
	if _, err := s.ResharePost(ctx, author, originalID, ""); err == nil {
		t.Fatal("expected error when original post not found")
	}
}

// ─── capturePublisher ─────────────────────────────────────────────────────────

// capturePublisher is a test-only Publisher that records every published event.
type capturePublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (c *capturePublisher) Publish(_ context.Context, e events.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func (c *capturePublisher) all() []events.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]events.Event, len(c.events))
	copy(out, c.events)
	return out
}

func (c *capturePublisher) first() (events.Event, bool) {
	evts := c.all()
	if len(evts) == 0 {
		return events.Event{}, false
	}
	return evts[0], true
}

// ─── Event publishing tests ───────────────────────────────────────────────────

func newServiceWithCapture(st contentStore) (*ContentService, *capturePublisher) {
	s := NewContentService(st)
	s.signingKey = []byte("test-secret")
	pub := &capturePublisher{}
	s.SetPublisher(pub)
	return s, pub
}

func TestCreatePost_PublishesPostCreatedEvent(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	authorID := uuid.New()

	post, err := s.CreatePost(context.Background(), authorID, "hello world", 0)
	if err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected post.created event to be published")
	}
	if evt.Type != events.TypePostCreated {
		t.Errorf("event type: got %s want %s", evt.Type, events.TypePostCreated)
	}

	var payload events.PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PostID != post.ID.String() {
		t.Errorf("payload PostID: got %s want %s", payload.PostID, post.ID)
	}
	if payload.AuthorID != authorID.String() {
		t.Errorf("payload AuthorID: got %s want %s", payload.AuthorID, authorID)
	}
	if payload.Kind != "post" {
		t.Errorf("payload Kind: got %s want post", payload.Kind)
	}
	if payload.ParentID != nil {
		t.Errorf("payload ParentID: expected nil for root post, got %s", *payload.ParentID)
	}
}

func TestCreateNote_PublishesPostCreatedEventWithKindNote(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	authorID := uuid.New()

	_, err := s.CreateNote(context.Background(), authorID, CreateNoteInput{
		NoteTitle: "My Note",
		Content:   "body content",
	})
	if err != nil {
		t.Fatalf("CreateNote error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected post.created event to be published")
	}

	var payload events.PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Kind != "note" {
		t.Errorf("payload Kind: got %s want note", payload.Kind)
	}
}

func TestReplyPost_PublishesPostCreatedEventWithKindReplyAndParent(t *testing.T) {
	st := newFakeContentStore()
	parentID := uuid.New()
	parentAuthorID := uuid.New()
	st.getPostByIDFn = func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return db.Post{ID: parentID, AuthorID: parentAuthorID, Content: "parent", CreatedAt: time.Now()}, nil
	}

	s, pub := newServiceWithCapture(st)
	authorID := uuid.New()

	_, err := s.ReplyPost(context.Background(), authorID, parentID, "my reply")
	if err != nil {
		t.Fatalf("ReplyPost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected post.created event to be published")
	}

	var payload events.PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Kind != "reply" {
		t.Errorf("payload Kind: got %s want reply", payload.Kind)
	}
	if payload.ParentID == nil || *payload.ParentID != parentID.String() {
		t.Errorf("payload ParentID: got %v want %s", payload.ParentID, parentID)
	}
	if payload.ParentAuthorID == nil || *payload.ParentAuthorID != parentAuthorID.String() {
		t.Errorf("payload ParentAuthorID: got %v want %s", payload.ParentAuthorID, parentAuthorID)
	}
}

func TestResharePost_PublishesPostCreatedEventWithKindReshare(t *testing.T) {
	st := newFakeContentStore()
	originalID := uuid.New()
	originalAuthorID := uuid.New()
	st.getPostByIDFn = func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return db.Post{ID: originalID, AuthorID: originalAuthorID, Content: "original", CreatedAt: time.Now()}, nil
	}

	s, pub := newServiceWithCapture(st)
	authorID := uuid.New()

	_, err := s.ResharePost(context.Background(), authorID, originalID, "nice post")
	if err != nil {
		t.Fatalf("ResharePost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected post.created event to be published")
	}

	var payload events.PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Kind != "reshare" {
		t.Errorf("payload Kind: got %s want reshare", payload.Kind)
	}
	if payload.ParentID == nil || *payload.ParentID != originalID.String() {
		t.Errorf("payload ParentID: got %v want %s", payload.ParentID, originalID)
	}
	if payload.ParentAuthorID == nil || *payload.ParentAuthorID != originalAuthorID.String() {
		t.Errorf("payload ParentAuthorID: got %v want %s", payload.ParentAuthorID, originalAuthorID)
	}
}

func TestLikePost_PublishesReactionUpsertedEvent(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	postID, userID := uuid.New(), uuid.New()

	if err := s.LikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("LikePost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected reaction.upserted event to be published")
	}
	if evt.Type != events.TypeReactionUpserted {
		t.Errorf("event type: got %s want %s", evt.Type, events.TypeReactionUpserted)
	}

	var payload events.ReactionUpsertedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PostID != postID.String() {
		t.Errorf("PostID: got %s want %s", payload.PostID, postID)
	}
	if payload.Emotion != "like" {
		t.Errorf("Emotion: got %s want like", payload.Emotion)
	}
}

func TestReactPost_PublishesReactionUpsertedEvent(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	postID, userID := uuid.New(), uuid.New()

	if err := s.ReactPost(context.Background(), postID, userID, "love", nil); err != nil {
		t.Fatalf("ReactPost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected reaction.upserted event to be published")
	}

	var payload events.ReactionUpsertedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Emotion != "love" {
		t.Errorf("Emotion: got %s want love", payload.Emotion)
	}
}

func TestUnlikePost_PublishesReactionRemovedEvent(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	postID, userID := uuid.New(), uuid.New()

	if err := s.UnlikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("UnlikePost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected reaction.removed event to be published")
	}
	if evt.Type != events.TypeReactionRemoved {
		t.Errorf("event type: got %s want %s", evt.Type, events.TypeReactionRemoved)
	}

	var payload events.ReactionRemovedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PostID != postID.String() || payload.UserID != userID.String() {
		t.Errorf("payload mismatch: got %+v", payload)
	}
}

func TestUnreactPost_PublishesReactionRemovedEvent(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)
	postID, userID := uuid.New(), uuid.New()

	if err := s.UnreactPost(context.Background(), postID, userID); err != nil {
		t.Fatalf("UnreactPost error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected reaction.removed event to be published")
	}
	if evt.Type != events.TypeReactionRemoved {
		t.Errorf("event type: got %s want %s", evt.Type, events.TypeReactionRemoved)
	}
}

func TestCreateArticleComment_PublishesCommentCreatedEvent(t *testing.T) {
	st := newFakeContentStore()
	articleID, authorID, articleAuthorID := uuid.New(), uuid.New(), uuid.New()
	st.getArticleByIDFn = func(_ context.Context, id uuid.UUID) (db.Article, error) {
		return db.Article{ID: id, AuthorID: articleAuthorID, Status: "published", AccessPolicy: "public", MinTrustLevel: 0}, nil
	}

	s, pub := newServiceWithCapture(st)

	comment, err := s.CreateArticleComment(context.Background(), articleID, authorID, "great article", 0, nil)
	if err != nil {
		t.Fatalf("CreateArticleComment error: %v", err)
	}

	evt, ok := pub.first()
	if !ok {
		t.Fatal("expected comment.created event to be published")
	}
	if evt.Type != events.TypeCommentCreated {
		t.Errorf("event type: got %s want %s", evt.Type, events.TypeCommentCreated)
	}

	var payload events.CommentCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.CommentID != comment.ID.String() {
		t.Errorf("CommentID: got %s want %s", payload.CommentID, comment.ID)
	}
	if payload.ArticleID != articleID.String() {
		t.Errorf("ArticleID: got %s want %s", payload.ArticleID, articleID)
	}
	if payload.AuthorID != authorID.String() {
		t.Errorf("AuthorID: got %s want %s", payload.AuthorID, authorID)
	}
	if payload.ArticleAuthorID != articleAuthorID.String() {
		t.Errorf("ArticleAuthorID: got %s want %s", payload.ArticleAuthorID, articleAuthorID)
	}
	if payload.ParentID != nil {
		t.Errorf("ParentID: expected nil for root comment, got %s", *payload.ParentID)
	}
}

func TestCreateArticleComment_WithParent_PublishesParentID(t *testing.T) {
	st := newFakeContentStore()
	parentID := uuid.New()
	st.createArticleCommentFn = func(_ context.Context, p db.CreateArticleCommentParams) (db.ArticleComment, error) {
		return db.ArticleComment{ID: uuid.New(), ArticleID: p.ArticleID, AuthorID: p.AuthorID, ParentID: p.ParentID, Content: p.Content, CreatedAt: time.Now()}, nil
	}

	s, pub := newServiceWithCapture(st)
	articleID, authorID := uuid.New(), uuid.New()

	_, err := s.CreateArticleComment(context.Background(), articleID, authorID, "reply", 0, &parentID)
	if err != nil {
		t.Fatalf("CreateArticleComment error: %v", err)
	}

	evt, _ := pub.first()
	var payload events.CommentCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ParentID == nil || *payload.ParentID != parentID.String() {
		t.Errorf("ParentID: got %v want %s", payload.ParentID, parentID)
	}
}

func TestCreatePost_DBError_NoEventPublished(t *testing.T) {
	st := newFakeContentStore()
	st.createPostFn = func(context.Context, db.CreatePostParams) (db.Post, error) {
		return db.Post{}, errors.New("db error")
	}

	s, pub := newServiceWithCapture(st)
	_, err := s.CreatePost(context.Background(), uuid.New(), "hello", 0)
	if err == nil {
		t.Fatal("expected error from DB")
	}
	if evts := pub.all(); len(evts) != 0 {
		t.Errorf("expected no events on DB error, got %d", len(evts))
	}
}

func TestReactPost_UnsupportedEmotion_NoEventPublished(t *testing.T) {
	st := newFakeContentStore()
	s, pub := newServiceWithCapture(st)

	err := s.ReactPost(context.Background(), uuid.New(), uuid.New(), "thumbsup", nil)
	if err == nil {
		t.Fatal("expected error for unsupported emotion")
	}
	if evts := pub.all(); len(evts) != 0 {
		t.Errorf("expected no events for unsupported emotion, got %d", len(evts))
	}
}
