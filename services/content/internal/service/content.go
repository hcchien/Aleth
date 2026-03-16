package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/aleth/content/internal/db"
	"github.com/aleth/content/internal/events"
)

// ContentService provides business logic for posts, articles, and boards.
type ContentService struct {
	db         contentStore
	signingKey []byte
	publisher  events.Publisher
}

type contentStore interface {
	GetBoardByOwnerID(ctx context.Context, ownerID uuid.UUID) (db.Board, error)
	GetBoardByID(ctx context.Context, id uuid.UUID) (db.Board, error)
	CreateBoard(ctx context.Context, ownerID uuid.UUID, name string) (db.Board, error)
	UpdateBoard(ctx context.Context, params db.UpdateBoardParams) (db.Board, error)
	UpdateBoardVcPolicy(ctx context.Context, params db.UpdateBoardVcPolicyParams) (db.Board, error)
	SubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error
	UnsubscribeBoard(ctx context.Context, boardID, userID uuid.UUID) error
	IsSubscribed(ctx context.Context, boardID, userID uuid.UUID) (bool, error)
	CountSubscribers(ctx context.Context, boardID uuid.UUID) (int64, error)

	CreatePost(ctx context.Context, params db.CreatePostParams) (db.Post, error)
	GetPostByID(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (db.Post, error)
	ListPosts(ctx context.Context, params db.ListPostsParams) ([]db.Post, error)
	ListPostReplies(ctx context.Context, params db.ListPostRepliesParams) ([]db.Post, error)
	SoftDeletePost(ctx context.Context, id, authorID uuid.UUID) error
	CreateNote(ctx context.Context, params db.CreateNoteParams) (db.Post, error)
	ListNotes(ctx context.Context, params db.ListNotesParams) ([]db.Post, error)
	ResharePost(ctx context.Context, params db.ResharePostParams) (db.Post, error)
	LikePost(ctx context.Context, postID, userID uuid.UUID) error
	ReactPost(ctx context.Context, postID, userID uuid.UUID, emotion string, sourceIP *string) error
	UnlikePost(ctx context.Context, postID, userID uuid.UUID) error
	ListPostReactionCounts(ctx context.Context, postID uuid.UUID) ([]db.ReactionCount, error)
	UpdatePostSignature(ctx context.Context, id uuid.UUID, signature []byte) error

	CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.Article, error)
	GetArticleByID(ctx context.Context, id uuid.UUID) (db.Article, error)
	UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.Article, error)
	PublishArticle(ctx context.Context, id uuid.UUID) (db.Article, error)
	DeleteArticle(ctx context.Context, id, authorID uuid.UUID) error
	ListBoardArticles(ctx context.Context, params db.ListArticlesParams) ([]db.Article, error)
	UpdateArticleSignature(ctx context.Context, id uuid.UUID, signature []byte) error
	CreateArticleComment(ctx context.Context, params db.CreateArticleCommentParams) (db.ArticleComment, error)
	ListArticleComments(ctx context.Context, articleID uuid.UUID, limit int) ([]db.ArticleComment, error)
	ListCommentReplies(ctx context.Context, parentID uuid.UUID, limit int) ([]db.ArticleComment, error)
}

func NewContentService(store contentStore) *ContentService {
	return &ContentService{db: store, publisher: &events.DirectPublisher{}}
}

func (s *ContentService) SetSigningSecret(secret string) {
	s.signingKey = []byte(strings.TrimSpace(secret))
}

// SetPublisher replaces the publisher used to emit domain events after writes.
// Call this during startup to switch between DirectPublisher (local) and PubSubPublisher (production).
func (s *ContentService) SetPublisher(p events.Publisher) {
	s.publisher = p
}

// publishEvent emits an event after a successful write. Failures are logged but
// never returned — the write itself has already succeeded and must not be rolled back.
func (s *ContentService) publishEvent(ctx context.Context, eventType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("type", eventType).Msg("failed to marshal event payload")
		return
	}
	evt := events.Event{
		ID:         uuid.New().String(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    data,
	}
	if err := s.publisher.Publish(ctx, evt); err != nil {
		log.Error().Err(err).Str("type", eventType).Str("id", evt.ID).Msg("failed to publish event")
	}
}

// ─── Board ────────────────────────────────────────────────────────────────────

// GetOrCreateBoard returns the user's board, creating it with a default name if
// it doesn't exist yet.
func (s *ContentService) GetOrCreateBoard(ctx context.Context, ownerID uuid.UUID, defaultName string) (db.Board, error) {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err == nil {
		return board, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Board{}, fmt.Errorf("get board: %w", err)
	}
	// Board does not exist — create it
	return s.db.CreateBoard(ctx, ownerID, defaultName)
}

// GetBoardByID returns a board by its UUID.
func (s *ContentService) GetBoardByID(ctx context.Context, id uuid.UUID) (*db.Board, error) {
	board, err := s.db.GetBoardByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get board: %w", err)
	}
	return &board, nil
}

// GetBoardByOwnerID returns a board by owner UUID.
func (s *ContentService) GetBoardByOwnerID(ctx context.Context, ownerID uuid.UUID) (*db.Board, error) {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get board: %w", err)
	}
	return &board, nil
}

type UpdateBoardInput struct {
	Name          *string
	Description   *string
	DefaultAccess *string
}

// UpdateBoardSettings updates the calling user's board metadata.
func (s *ContentService) UpdateBoardSettings(ctx context.Context, ownerID uuid.UUID, input UpdateBoardInput) (db.Board, error) {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Board{}, fmt.Errorf("board not found")
		}
		return db.Board{}, fmt.Errorf("get board: %w", err)
	}

	if input.DefaultAccess != nil {
		if err := validateAccessPolicy(*input.DefaultAccess); err != nil {
			return db.Board{}, err
		}
	}

	return s.db.UpdateBoard(ctx, db.UpdateBoardParams{
		ID:            board.ID,
		Name:          input.Name,
		Description:   input.Description,
		DefaultAccess: input.DefaultAccess,
	})
}

// UpdateBoardVcPolicy sets the trust-level gates and VC requirement lists.
// Only the board owner should call this.
func (s *ContentService) UpdateBoardVcPolicy(ctx context.Context, boardID uuid.UUID, minTrust, minCommentTrust int16, requireVcs, requireCommentVcs []db.VcRequirement) (db.Board, error) {
	return s.db.UpdateBoardVcPolicy(ctx, db.UpdateBoardVcPolicyParams{
		ID:                boardID,
		MinTrustLevel:     minTrust,
		MinCommentTrust:   minCommentTrust,
		RequireVcs:        requireVcs,
		RequireCommentVcs: requireCommentVcs,
	})
}

// SubscribeBoard subscribes the given user to a board identified by its owner.
// Subscribing to your own board is allowed but has no practical effect.
func (s *ContentService) SubscribeBoard(ctx context.Context, ownerID, subscriberID uuid.UUID) error {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("board not found")
		}
		return fmt.Errorf("get board: %w", err)
	}
	return s.db.SubscribeBoard(ctx, board.ID, subscriberID)
}

// UnsubscribeBoard removes the subscription of the given user from a board.
func (s *ContentService) UnsubscribeBoard(ctx context.Context, ownerID, subscriberID uuid.UUID) error {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("board not found")
		}
		return fmt.Errorf("get board: %w", err)
	}
	return s.db.UnsubscribeBoard(ctx, board.ID, subscriberID)
}

// IsSubscribedToBoard reports whether viewerID is subscribed to the board owned by ownerID.
func (s *ContentService) IsSubscribedToBoard(ctx context.Context, boardID, viewerID uuid.UUID) (bool, error) {
	return s.db.IsSubscribed(ctx, boardID, viewerID)
}

// CountBoardSubscribers returns the total subscriber count for the given board.
func (s *ContentService) CountBoardSubscribers(ctx context.Context, boardID uuid.UUID) (int64, error) {
	return s.db.CountSubscribers(ctx, boardID)
}

// ─── Posts ────────────────────────────────────────────────────────────────────

// CreatePost creates a new root-level post.
func (s *ContentService) CreatePost(ctx context.Context, authorID uuid.UUID, content string, authorTrustLevel int) (db.Post, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return db.Post{}, fmt.Errorf("content cannot be empty")
	}
	if len([]rune(content)) > 500 {
		return db.Post{}, fmt.Errorf("content exceeds 500 characters")
	}

	post, err := s.db.CreatePost(ctx, db.CreatePostParams{
		AuthorID:         authorID,
		Content:          content,
		AuthorTrustLevel: authorTrustLevel,
	})
	if err != nil {
		return db.Post{}, err
	}
	if err := s.signPost(ctx, &post); err != nil {
		return db.Post{}, err
	}
	s.publishEvent(ctx, events.TypePostCreated, events.PostCreatedPayload{
		PostID:   post.ID.String(),
		AuthorID: authorID.String(),
		Kind:     "post",
	})
	return post, nil
}

// ─── Notes ────────────────────────────────────────────────────────────────────

// CreateNoteInput holds the fields required to create a note.
type CreateNoteInput struct {
	Content          string
	NoteTitle        string
	NoteCover        *string
	NoteSummary      *string
	AuthorTrustLevel int
}

// CreateNote creates a new long-form note post.
func (s *ContentService) CreateNote(ctx context.Context, authorID uuid.UUID, input CreateNoteInput) (db.Post, error) {
	input.NoteTitle = strings.TrimSpace(input.NoteTitle)
	if input.NoteTitle == "" {
		return db.Post{}, fmt.Errorf("note title cannot be empty")
	}
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		return db.Post{}, fmt.Errorf("note content cannot be empty")
	}

	note, err := s.db.CreateNote(ctx, db.CreateNoteParams{
		AuthorID:         authorID,
		Content:          input.Content,
		NoteTitle:        input.NoteTitle,
		NoteCover:        input.NoteCover,
		NoteSummary:      input.NoteSummary,
		AuthorTrustLevel: input.AuthorTrustLevel,
	})
	if err != nil {
		return db.Post{}, err
	}
	if err := s.signPost(ctx, &note); err != nil {
		return db.Post{}, err
	}
	s.publishEvent(ctx, events.TypePostCreated, events.PostCreatedPayload{
		PostID:   note.ID.String(),
		AuthorID: authorID.String(),
		Kind:     "note",
	})
	return note, nil
}

// ListNotes returns a paginated list of notes.
func (s *ContentService) ListNotes(ctx context.Context, after *uuid.UUID, limit int, viewerID *uuid.UUID) ([]db.Post, error) {
	return s.db.ListNotes(ctx, db.ListNotesParams{
		After:    after,
		Limit:    limit,
		ViewerID: viewerID,
	})
}

// ResharePost creates a new post that reshares an existing post with an optional comment.
func (s *ContentService) ResharePost(ctx context.Context, authorID uuid.UUID, originalID uuid.UUID, content string) (db.Post, error) {
	original, err := s.db.GetPostByID(ctx, originalID, nil)
	if err != nil {
		return db.Post{}, fmt.Errorf("original post not found: %w", err)
	}
	if original.ResharedFromID != nil {
		return db.Post{}, fmt.Errorf("cannot reshare a reshare")
	}

	content = strings.TrimSpace(content)

	post, err := s.db.ResharePost(ctx, db.ResharePostParams{
		AuthorID:       authorID,
		Content:        content,
		ResharedFromID: originalID,
	})
	if err != nil {
		return db.Post{}, err
	}
	if err := s.signPost(ctx, &post); err != nil {
		return db.Post{}, err
	}
	originalIDStr := originalID.String()
	originalAuthorIDStr := original.AuthorID.String()
	s.publishEvent(ctx, events.TypePostCreated, events.PostCreatedPayload{
		PostID:         post.ID.String(),
		AuthorID:       authorID.String(),
		Kind:           "reshare",
		ParentID:       &originalIDStr,
		ParentAuthorID: &originalAuthorIDStr,
	})
	return post, nil
}

// ReplyPost creates a reply to an existing post.
func (s *ContentService) ReplyPost(ctx context.Context, authorID, postID uuid.UUID, content string) (db.Post, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return db.Post{}, fmt.Errorf("content cannot be empty")
	}
	if len([]rune(content)) > 500 {
		return db.Post{}, fmt.Errorf("content exceeds 500 characters")
	}

	parent, err := s.db.GetPostByID(ctx, postID, nil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Post{}, fmt.Errorf("post not found")
		}
		return db.Post{}, fmt.Errorf("get parent post: %w", err)
	}
	if parent.DeletedAt != nil {
		return db.Post{}, fmt.Errorf("cannot reply to a deleted post")
	}

	// rootID: if the parent is itself a reply, propagate its root; otherwise the parent is the root.
	rootID := parent.RootID
	if rootID == nil {
		rootID = &parent.ID
	}

	post, err := s.db.CreatePost(ctx, db.CreatePostParams{
		AuthorID: authorID,
		ParentID: &parent.ID,
		RootID:   rootID,
		Content:  content,
	})
	if err != nil {
		return db.Post{}, err
	}
	if err := s.signPost(ctx, &post); err != nil {
		return db.Post{}, err
	}
	parentIDStr := postID.String()
	parentAuthorIDStr := parent.AuthorID.String()
	s.publishEvent(ctx, events.TypePostCreated, events.PostCreatedPayload{
		PostID:         post.ID.String(),
		AuthorID:       authorID.String(),
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &parentAuthorIDStr,
	})
	return post, nil
}

// GetPost returns a post by ID.
func (s *ContentService) GetPost(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*db.Post, error) {
	post, err := s.db.GetPostByID(ctx, id, viewerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	return &post, nil
}

// ListPosts returns a paginated list of public root posts.
func (s *ContentService) ListPosts(ctx context.Context, after *uuid.UUID, limit int, viewerID *uuid.UUID) ([]db.Post, error) {
	return s.db.ListPosts(ctx, db.ListPostsParams{
		After:    after,
		Limit:    limit,
		ViewerID: viewerID,
	})
}

// GetPostReplies returns direct replies to a post in chronological order.
func (s *ContentService) GetPostReplies(ctx context.Context, parentID uuid.UUID, viewerID *uuid.UUID, limit int) ([]db.Post, error) {
	return s.db.ListPostReplies(ctx, db.ListPostRepliesParams{
		ParentID: parentID,
		ViewerID: viewerID,
		Limit:    limit,
	})
}

// DeletePost soft-deletes a post, verifying ownership.
func (s *ContentService) DeletePost(ctx context.Context, id, authorID uuid.UUID) error {
	return s.db.SoftDeletePost(ctx, id, authorID)
}

// LikePost records a like from userID on postID.
func (s *ContentService) LikePost(ctx context.Context, postID, userID uuid.UUID) error {
	if err := s.db.LikePost(ctx, postID, userID); err != nil {
		return err
	}
	s.publishEvent(ctx, events.TypeReactionUpserted, events.ReactionUpsertedPayload{
		PostID:  postID.String(),
		UserID:  userID.String(),
		Emotion: "like",
	})
	return nil
}

func (s *ContentService) ReactPost(ctx context.Context, postID, userID uuid.UUID, emotion string, sourceIP *string) error {
	if !isSupportedEmotion(emotion) {
		return fmt.Errorf("unsupported emotion")
	}
	if err := s.db.ReactPost(ctx, postID, userID, emotion, sourceIP); err != nil {
		return err
	}
	s.publishEvent(ctx, events.TypeReactionUpserted, events.ReactionUpsertedPayload{
		PostID:  postID.String(),
		UserID:  userID.String(),
		Emotion: emotion,
	})
	return nil
}

// UnlikePost removes a like from userID on postID.
func (s *ContentService) UnlikePost(ctx context.Context, postID, userID uuid.UUID) error {
	if err := s.db.UnlikePost(ctx, postID, userID); err != nil {
		return err
	}
	s.publishEvent(ctx, events.TypeReactionRemoved, events.ReactionRemovedPayload{
		PostID: postID.String(),
		UserID: userID.String(),
	})
	return nil
}

func (s *ContentService) UnreactPost(ctx context.Context, postID, userID uuid.UUID) error {
	if err := s.db.UnlikePost(ctx, postID, userID); err != nil {
		return err
	}
	s.publishEvent(ctx, events.TypeReactionRemoved, events.ReactionRemovedPayload{
		PostID: postID.String(),
		UserID: userID.String(),
	})
	return nil
}

func (s *ContentService) ListPostReactionCounts(ctx context.Context, postID uuid.UUID) ([]db.ReactionCount, error) {
	return s.db.ListPostReactionCounts(ctx, postID)
}

// ─── Articles ─────────────────────────────────────────────────────────────────

type CreateArticleInput struct {
	Title        string
	ContentMd    *string
	AccessPolicy string
}

// CreateArticle creates a draft article in the author's board.
// The board is created automatically if it doesn't exist.
func (s *ContentService) CreateArticle(ctx context.Context, authorID uuid.UUID, input CreateArticleInput) (db.Article, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return db.Article{}, fmt.Errorf("title cannot be empty")
	}
	if err := validateAccessPolicy(input.AccessPolicy); err != nil {
		return db.Article{}, err
	}

	board, err := s.GetOrCreateBoard(ctx, authorID, "My Board")
	if err != nil {
		return db.Article{}, fmt.Errorf("get or create board: %w", err)
	}

	slug := slugify(input.Title)

	article, err := s.db.CreateArticle(ctx, db.CreateArticleParams{
		BoardID:      board.ID,
		AuthorID:     authorID,
		Title:        input.Title,
		Slug:         slug,
		ContentMd:    input.ContentMd,
		AccessPolicy: input.AccessPolicy,
	})
	if err != nil {
		return db.Article{}, err
	}
	if err := s.signArticle(ctx, &article); err != nil {
		return db.Article{}, err
	}
	return article, nil
}

type UpdateArticleInput struct {
	Title        *string
	ContentMd    *string
	AccessPolicy *string
	Status       *string
}

// UpdateArticle updates article fields, verifying ownership.
func (s *ContentService) UpdateArticle(ctx context.Context, id, authorID uuid.UUID, input UpdateArticleInput) (db.Article, error) {
	existing, err := s.db.GetArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Article{}, fmt.Errorf("article not found")
		}
		return db.Article{}, fmt.Errorf("get article: %w", err)
	}
	if existing.AuthorID != authorID {
		return db.Article{}, fmt.Errorf("not authorized")
	}

	if input.AccessPolicy != nil {
		if err := validateAccessPolicy(*input.AccessPolicy); err != nil {
			return db.Article{}, err
		}
	}
	if input.Status != nil {
		if err := validateArticleStatus(*input.Status); err != nil {
			return db.Article{}, err
		}
	}

	article, err := s.db.UpdateArticle(ctx, db.UpdateArticleParams{
		ID:           id,
		Title:        input.Title,
		ContentMd:    input.ContentMd,
		AccessPolicy: input.AccessPolicy,
		Status:       input.Status,
	})
	if err != nil {
		return db.Article{}, err
	}
	if err := s.signArticle(ctx, &article); err != nil {
		return db.Article{}, err
	}
	return article, nil
}

// PublishArticle sets an article's status to published, verifying ownership.
func (s *ContentService) PublishArticle(ctx context.Context, id, authorID uuid.UUID) (db.Article, error) {
	existing, err := s.db.GetArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Article{}, fmt.Errorf("article not found")
		}
		return db.Article{}, fmt.Errorf("get article: %w", err)
	}
	if existing.AuthorID != authorID {
		return db.Article{}, fmt.Errorf("not authorized")
	}

	article, err := s.db.PublishArticle(ctx, id)
	if err != nil {
		return db.Article{}, err
	}
	if err := s.signArticle(ctx, &article); err != nil {
		return db.Article{}, err
	}
	return article, nil
}

// GetArticle returns an article by ID, enforcing access policy.
// viewerTrustLevel is -1 for unauthenticated users, 0 for L0, 1+ for higher trust.
func (s *ContentService) GetArticle(ctx context.Context, id uuid.UUID, viewerTrustLevel int) (*db.Article, error) {
	article, err := s.db.GetArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get article: %w", err)
	}
	if article.Status != "published" && article.Status != "unlisted" {
		// Drafts are not visible to non-authors via this path
		return nil, nil
	}
	// viewerTrustLevel == -1 means unauthenticated; members policy requires login (L0+)
	if article.AccessPolicy == "members" && viewerTrustLevel < 0 {
		return nil, fmt.Errorf("members-only content")
	}
	// For public articles, unauthenticated visitors are treated as L0 (trust 0) so they
	// can read content with min_trust_level == 0 without having to log in.
	effectiveTrust := viewerTrustLevel
	if article.AccessPolicy == "public" && effectiveTrust < 0 {
		effectiveTrust = 0
	}
	if effectiveTrust < int(article.MinTrustLevel) {
		return nil, fmt.Errorf("insufficient trust level")
	}
	return &article, nil
}

// DeleteArticle removes an article, verifying ownership.
func (s *ContentService) DeleteArticle(ctx context.Context, id, authorID uuid.UUID) error {
	return s.db.DeleteArticle(ctx, id, authorID)
}

// ListBoardArticles returns paginated published articles for a board.
func (s *ContentService) ListBoardArticles(ctx context.Context, ownerID uuid.UUID, after *uuid.UUID, limit int, includeOwnerDrafts bool) ([]db.Article, error) {
	board, err := s.db.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get board: %w", err)
	}

	return s.db.ListBoardArticles(ctx, db.ListArticlesParams{
		BoardID:       board.ID,
		After:         after,
		Limit:         limit,
		IncludeDrafts: includeOwnerDrafts,
	})
}

// CreateArticleComment creates a comment on an article.
// viewerTrustLevel must match the article's min_trust_level; pass -1 for unauthenticated (always rejected).
// parentCommentID is optional; when set, the comment is a reply to the given comment.
func (s *ContentService) CreateArticleComment(ctx context.Context, articleID, authorID uuid.UUID, content string, viewerTrustLevel int, parentCommentID *uuid.UUID) (db.ArticleComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return db.ArticleComment{}, fmt.Errorf("comment content cannot be empty")
	}
	if len([]rune(content)) > 1000 {
		return db.ArticleComment{}, fmt.Errorf("comment content exceeds 1000 characters")
	}

	article, err := s.db.GetArticleByID(ctx, articleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ArticleComment{}, fmt.Errorf("article not found")
		}
		return db.ArticleComment{}, fmt.Errorf("get article: %w", err)
	}
	if article.Status != "published" && article.Status != "unlisted" {
		return db.ArticleComment{}, fmt.Errorf("article not available for comments")
	}
	if article.AccessPolicy == "members" && viewerTrustLevel < 0 {
		return db.ArticleComment{}, fmt.Errorf("members-only content")
	}
	if viewerTrustLevel < int(article.MinTrustLevel) {
		return db.ArticleComment{}, fmt.Errorf("insufficient trust level")
	}

	comment, err := s.db.CreateArticleComment(ctx, db.CreateArticleCommentParams{
		ArticleID: articleID,
		AuthorID:  authorID,
		ParentID:  parentCommentID,
		Content:   content,
	})
	if err != nil {
		return db.ArticleComment{}, err
	}
	payload := events.CommentCreatedPayload{
		CommentID:       comment.ID.String(),
		ArticleID:       articleID.String(),
		AuthorID:        authorID.String(),
		ArticleAuthorID: article.AuthorID.String(),
	}
	if parentCommentID != nil {
		parentIDStr := parentCommentID.String()
		payload.ParentID = &parentIDStr
		// ParentAuthorID would require fetching the parent comment; omit for now.
		// The notification service can look up the parent comment author separately if needed.
	}
	s.publishEvent(ctx, events.TypeCommentCreated, payload)
	return comment, nil
}

func (s *ContentService) ListArticleComments(ctx context.Context, articleID uuid.UUID, limit int) ([]db.ArticleComment, error) {
	return s.db.ListArticleComments(ctx, articleID, limit)
}

func (s *ContentService) GetCommentReplies(ctx context.Context, parentID uuid.UUID, limit int) ([]db.ArticleComment, error) {
	return s.db.ListCommentReplies(ctx, parentID, limit)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = uuid.New().String()[:8]
	}
	return s
}

func validateAccessPolicy(policy string) error {
	switch policy {
	case "public", "members":
		return nil
	}
	return fmt.Errorf("invalid access policy %q: must be 'public' or 'members'", policy)
}

func validateArticleStatus(status string) error {
	switch status {
	case "draft", "published", "unlisted":
		return nil
	}
	return fmt.Errorf("invalid status %q: must be 'draft', 'published', or 'unlisted'", status)
}

func isSupportedEmotion(emotion string) bool {
	switch emotion {
	case "like", "love", "haha", "wow", "sad", "angry":
		return true
	default:
		return false
	}
}

type SignatureInfo struct {
	IsSigned    bool
	IsVerified  bool
	ContentHash *string
	Signature   *string
	Algorithm   *string
	Explanation string
}

func (s *ContentService) BuildPostSignatureInfo(post db.Post) SignatureInfo {
	return s.buildSignatureInfo(post.AuthorID, post.Signature, post.Content)
}

func (s *ContentService) BuildArticleSignatureInfo(article db.Article) SignatureInfo {
	return s.buildSignatureInfo(article.AuthorID, article.Signature, articleSigningContent(article))
}

func (s *ContentService) signPost(ctx context.Context, post *db.Post) error {
	if len(s.signingKey) == 0 {
		return nil
	}
	sig, err := s.makeSignaturePayload(post.AuthorID, post.Content)
	if err != nil {
		return fmt.Errorf("sign post: %w", err)
	}
	if err := s.db.UpdatePostSignature(ctx, post.ID, sig); err != nil {
		// Post is already committed; signature update is best-effort.
		log.Error().Err(err).Str("post_id", post.ID.String()).Msg("failed to persist post signature")
		return nil
	}
	post.Signature = sig
	return nil
}

func (s *ContentService) signArticle(ctx context.Context, article *db.Article) error {
	if len(s.signingKey) == 0 {
		return nil
	}
	sig, err := s.makeSignaturePayload(article.AuthorID, articleSigningContent(*article))
	if err != nil {
		return fmt.Errorf("sign article: %w", err)
	}
	if err := s.db.UpdateArticleSignature(ctx, article.ID, sig); err != nil {
		// Article is already committed; signature update is best-effort.
		log.Error().Err(err).Str("article_id", article.ID.String()).Msg("failed to persist article signature")
		return nil
	}
	article.Signature = sig
	return nil
}

func (s *ContentService) makeSignaturePayload(authorID uuid.UUID, content string) ([]byte, error) {
	hash := contentHash(content)
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write([]byte(authorID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(hash))
	signature := hex.EncodeToString(mac.Sum(nil))

	payload := map[string]any{
		"version":     1,
		"algorithm":   "HMAC-SHA256",
		"contentHash": hash,
		"signature":   signature,
		"signedAt":    time.Now().UTC().Format(time.RFC3339),
		"signer":      "did:aleth:" + authorID.String(),
	}
	return json.Marshal(payload)
}

func (s *ContentService) buildSignatureInfo(authorID uuid.UUID, raw []byte, content string) SignatureInfo {
	hash := contentHash(content)
	info := SignatureInfo{
		IsSigned:    false,
		IsVerified:  false,
		ContentHash: &hash,
		Explanation: "Hash 是內容指紋；只要內容被改動，hash 就會不同。",
	}
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		info.Explanation = "內容尚未簽章。"
		return info
	}
	info.IsSigned = true

	var payload struct {
		Algorithm   string `json:"algorithm"`
		ContentHash string `json:"contentHash"`
		Signature   string `json:"signature"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		v := strings.TrimSpace(string(raw))
		info.Signature = &v
		info.Explanation = "簽章格式無法解析。"
		return info
	}
	if payload.Algorithm != "" {
		info.Algorithm = &payload.Algorithm
	}
	if payload.ContentHash != "" {
		info.ContentHash = &payload.ContentHash
	}
	if payload.Signature != "" {
		info.Signature = &payload.Signature
	}
	if len(s.signingKey) == 0 {
		info.Explanation = "伺服器未配置驗章金鑰。"
		return info
	}

	if payload.ContentHash == "" || payload.Signature == "" {
		info.Explanation = "簽章資料不完整。"
		return info
	}

	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write([]byte(authorID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(payload.ContentHash))
	expected := hex.EncodeToString(mac.Sum(nil))
	info.IsVerified = strings.EqualFold(payload.ContentHash, hash) && hmac.Equal([]byte(expected), []byte(payload.Signature))
	if info.IsVerified {
		info.Explanation = "已簽章驗證：作者身份已驗證，內容未被竄改。"
	} else {
		info.Explanation = "簽章驗證失敗：內容或簽章不一致。"
	}
	return info
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func articleSigningContent(article db.Article) string {
	body := ""
	if article.ContentMd != nil {
		body = *article.ContentMd
	}
	return article.Title + "\n" + body
}
