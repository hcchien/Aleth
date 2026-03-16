package graph

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	graphql "github.com/graph-gophers/graphql-go"
	"github.com/rs/zerolog/log"

	"github.com/aleth/content/internal/db"
	"github.com/aleth/content/internal/service"
)

//go:embed schema.graphqls
var schemaString string

// NewSchema builds the executable GraphQL schema wired to the content service.
func NewSchema(svc *service.ContentService) *graphql.Schema {
	return graphql.MustParseSchema(
		schemaString,
		&Resolver{svc: svc},
		graphql.UseStringDescriptions(),
	)
}

// NewSchemaWithFederation builds the schema with a federation notifier URL.
// After each top-level post creation, the federation service is notified.
func NewSchemaWithFederation(svc *service.ContentService, federationURL string) *graphql.Schema {
	return graphql.MustParseSchema(
		schemaString,
		&Resolver{svc: svc, federationURL: federationURL},
		graphql.UseStringDescriptions(),
	)
}

// ─── Context helpers ──────────────────────────────────────────────────────────

type contextKey string

const claimsKey contextKey = "claims"
const clientIPKey contextKey = "client_ip"

// UserClaims holds the validated JWT payload injected by the auth middleware.
type UserClaims struct {
	UserID     string
	Username   string
	TrustLevel int
}

// WithClaims injects user claims into the context.
func WithClaims(ctx context.Context, c UserClaims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext retrieves user claims (set by auth middleware).
func ClaimsFromContext(ctx context.Context) (UserClaims, bool) {
	c, ok := ctx.Value(claimsKey).(UserClaims)
	return c, ok
}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

func ClientIPFromContext(ctx context.Context) (string, bool) {
	ip, ok := ctx.Value(clientIPKey).(string)
	return ip, ok
}

// ─── Root resolver ────────────────────────────────────────────────────────────

type Resolver struct {
	svc           *service.ContentService
	federationURL string // empty = federation notifications disabled
}

// ─── Query resolvers ──────────────────────────────────────────────────────────

func (r *Resolver) Post(ctx context.Context, args struct{ ID graphql.ID }) (*PostResolver, error) {
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}
	claims, _ := ClaimsFromContext(ctx)
	viewerID := parseOptionalUUID(claims.UserID)

	post, err := r.svc.GetPost(ctx, id, viewerID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, nil
	}
	return &PostResolver{post: *post, svc: r.svc}, nil
}

func (r *Resolver) Posts(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*PostConnectionResolver, error) {
	claims, _ := ClaimsFromContext(ctx)
	viewerID := parseOptionalUUID(claims.UserID)

	var after *uuid.UUID
	if args.After != nil && *args.After != "" {
		id, err := uuid.Parse(*args.After)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor")
		}
		after = &id
	}

	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}

	posts, err := r.svc.ListPosts(ctx, after, limit, viewerID)
	if err != nil {
		return nil, err
	}
	return newPostConnection(posts, limit, r.svc), nil
}

func (r *Resolver) Notes(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*PostConnectionResolver, error) {
	claims, _ := ClaimsFromContext(ctx)
	viewerID := parseOptionalUUID(claims.UserID)

	var after *uuid.UUID
	if args.After != nil && *args.After != "" {
		id, err := uuid.Parse(*args.After)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor")
		}
		after = &id
	}

	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}

	notes, err := r.svc.ListNotes(ctx, after, limit, viewerID)
	if err != nil {
		return nil, err
	}
	return newPostConnection(notes, limit, r.svc), nil
}

type CreateNoteInput struct {
	Content     string
	NoteTitle   string
	NoteCover   *string
	NoteSummary *string
}

func (r *Resolver) CreateNote(ctx context.Context, args struct{ Input CreateNoteInput }) (*PostResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	authorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	note, err := r.svc.CreateNote(ctx, authorID, service.CreateNoteInput{
		Content:          args.Input.Content,
		NoteTitle:        args.Input.NoteTitle,
		NoteCover:        args.Input.NoteCover,
		NoteSummary:      args.Input.NoteSummary,
		AuthorTrustLevel: claims.TrustLevel,
	})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: note, svc: r.svc}, nil
}

type ResharePostInput struct {
	Content *string
}

func (r *Resolver) ResharePost(ctx context.Context, args struct {
	PostId graphql.ID
	Input  ResharePostInput
}) (*PostResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	authorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	originalID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}
	content := ""
	if args.Input.Content != nil {
		content = *args.Input.Content
	}
	post, err := r.svc.ResharePost(ctx, authorID, originalID, content)
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: post, svc: r.svc}, nil
}

func (r *Resolver) PostReplies(ctx context.Context, args struct {
	PostId graphql.ID
	Limit  *int32
}) ([]*PostResolver, error) {
	parentID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}
	claims, _ := ClaimsFromContext(ctx)
	viewerID := parseOptionalUUID(claims.UserID)

	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}

	posts, err := r.svc.GetPostReplies(ctx, parentID, viewerID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*PostResolver, len(posts))
	for i, p := range posts {
		out[i] = &PostResolver{post: p, svc: r.svc}
	}
	return out, nil
}

func (r *Resolver) Article(ctx context.Context, args struct{ ID graphql.ID }) (*ArticleResolver, error) {
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid article id")
	}
	// Use -1 to distinguish unauthenticated from L0 (both have TrustLevel==0).
	trustLevel := -1
	if claims, ok := ClaimsFromContext(ctx); ok {
		trustLevel = claims.TrustLevel
	}

	article, err := r.svc.GetArticle(ctx, id, trustLevel)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, nil
	}
	return &ArticleResolver{article: *article, svc: r.svc}, nil
}

func (r *Resolver) BoardByID(ctx context.Context, args struct{ ID graphql.ID }) (*BoardResolver, error) {
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid board id")
	}
	board, err := r.svc.GetBoardByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, nil
	}
	return &BoardResolver{board: *board, svc: r.svc}, nil
}

func (r *Resolver) Board(ctx context.Context, args struct{ OwnerID graphql.ID }) (*BoardResolver, error) {
	ownerID, err := uuid.Parse(string(args.OwnerID))
	if err != nil {
		return nil, fmt.Errorf("invalid owner id")
	}
	board, err := r.svc.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, nil
	}
	return &BoardResolver{board: *board, svc: r.svc}, nil
}

func (r *Resolver) BoardArticles(ctx context.Context, args struct {
	OwnerID graphql.ID
	After   *string
	Limit   *int32
}) (*ArticleConnectionResolver, error) {
	ownerID, err := uuid.Parse(string(args.OwnerID))
	if err != nil {
		return nil, fmt.Errorf("invalid owner id")
	}

	var after *uuid.UUID
	if args.After != nil && *args.After != "" {
		id, err := uuid.Parse(*args.After)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor")
		}
		after = &id
	}

	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}

	// Show drafts only to the board owner
	claims, _ := ClaimsFromContext(ctx)
	isOwner := claims.UserID == ownerID.String()

	articles, err := r.svc.ListBoardArticles(ctx, ownerID, after, limit, isOwner)
	if err != nil {
		return nil, err
	}
	return newArticleConnection(articles, limit, r.svc), nil
}

func (r *Resolver) ArticleComments(ctx context.Context, args struct {
	ArticleId graphql.ID
	Limit     *int32
}) ([]*ArticleCommentResolver, error) {
	articleID, err := uuid.Parse(string(args.ArticleId))
	if err != nil {
		return nil, fmt.Errorf("invalid article id")
	}
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	items, err := r.svc.ListArticleComments(ctx, articleID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*ArticleCommentResolver, 0, len(items))
	for _, item := range items {
		c := item
		out = append(out, &ArticleCommentResolver{comment: c})
	}
	return out, nil
}

// ─── Mutation resolvers ───────────────────────────────────────────────────────

type CreatePostInput struct {
	Content string
}

func (r *Resolver) CreatePost(ctx context.Context, args struct{ Input CreatePostInput }) (*PostResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	authorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	post, err := r.svc.CreatePost(ctx, authorID, args.Input.Content, claims.TrustLevel)
	if err != nil {
		return nil, err
	}
	// Notify federation service asynchronously (fire-and-forget).
	if r.federationURL != "" {
		go r.notifyFederation(claims.Username, post)
	}
	return &PostResolver{post: post, svc: r.svc}, nil
}

func (r *Resolver) ReplyPost(ctx context.Context, args struct {
	PostId graphql.ID
	Input  CreatePostInput
}) (*PostResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	authorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}
	post, err := r.svc.ReplyPost(ctx, authorID, postID, args.Input.Content)
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: post, svc: r.svc}, nil
}

func (r *Resolver) DeletePost(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	postID, err := uuid.Parse(string(args.ID))
	if err != nil {
		return false, fmt.Errorf("invalid post id")
	}
	authorID, _ := uuid.Parse(claims.UserID)
	if err := r.svc.DeletePost(ctx, postID, authorID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Resolver) LikePost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return false, fmt.Errorf("invalid post id")
	}
	userID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.LikePost(ctx, postID, userID)
}

func (r *Resolver) UnlikePost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return false, fmt.Errorf("invalid post id")
	}
	userID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.UnlikePost(ctx, postID, userID)
}

func (r *Resolver) ReactPost(ctx context.Context, args struct {
	PostId  graphql.ID
	Emotion string
}) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return false, fmt.Errorf("invalid post id")
	}
	userID, _ := uuid.Parse(claims.UserID)
	var sourceIP *string
	if ip, ok := ClientIPFromContext(ctx); ok && ip != "" {
		sourceIP = &ip
	}
	return true, r.svc.ReactPost(ctx, postID, userID, args.Emotion, sourceIP)
}

func (r *Resolver) UnreactPost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return false, fmt.Errorf("invalid post id")
	}
	userID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.UnreactPost(ctx, postID, userID)
}

func (r *Resolver) CommentReplies(ctx context.Context, args struct {
	CommentId graphql.ID
	Limit     *int32
}) ([]*ArticleCommentResolver, error) {
	commentID, err := uuid.Parse(string(args.CommentId))
	if err != nil {
		return nil, fmt.Errorf("invalid comment id")
	}
	limit := 100
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	items, err := r.svc.GetCommentReplies(ctx, commentID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*ArticleCommentResolver, 0, len(items))
	for _, c := range items {
		c := c
		out = append(out, &ArticleCommentResolver{comment: c})
	}
	return out, nil
}

func (r *Resolver) CreateArticleComment(ctx context.Context, args struct {
	ArticleId       graphql.ID
	Content         string
	ParentCommentId *graphql.ID
}) (*ArticleCommentResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	articleID, err := uuid.Parse(string(args.ArticleId))
	if err != nil {
		return nil, fmt.Errorf("invalid article id")
	}
	authorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	var parentCommentID *uuid.UUID
	if args.ParentCommentId != nil {
		id, err := uuid.Parse(string(*args.ParentCommentId))
		if err != nil {
			return nil, fmt.Errorf("invalid parent comment id")
		}
		parentCommentID = &id
	}
	comment, err := r.svc.CreateArticleComment(ctx, articleID, authorID, args.Content, claims.TrustLevel, parentCommentID)
	if err != nil {
		return nil, err
	}
	return &ArticleCommentResolver{comment: comment}, nil
}

type CreateArticleInput struct {
	Title        string
	ContentMd    *string
	AccessPolicy string
}

func (r *Resolver) CreateArticle(ctx context.Context, args struct{ Input CreateArticleInput }) (*ArticleResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	authorID, _ := uuid.Parse(claims.UserID)
	article, err := r.svc.CreateArticle(ctx, authorID, service.CreateArticleInput{
		Title:        args.Input.Title,
		ContentMd:    args.Input.ContentMd,
		AccessPolicy: args.Input.AccessPolicy,
	})
	if err != nil {
		return nil, err
	}
	return &ArticleResolver{article: article, svc: r.svc}, nil
}

type UpdateArticleInput struct {
	Title        *string
	ContentMd    *string
	AccessPolicy *string
	Status       *string
}

func (r *Resolver) UpdateArticle(ctx context.Context, args struct {
	ID    graphql.ID
	Input UpdateArticleInput
}) (*ArticleResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid article id")
	}
	authorID, _ := uuid.Parse(claims.UserID)
	article, err := r.svc.UpdateArticle(ctx, id, authorID, service.UpdateArticleInput{
		Title:        args.Input.Title,
		ContentMd:    args.Input.ContentMd,
		AccessPolicy: args.Input.AccessPolicy,
		Status:       args.Input.Status,
	})
	if err != nil {
		return nil, err
	}
	return &ArticleResolver{article: article, svc: r.svc}, nil
}

func (r *Resolver) PublishArticle(ctx context.Context, args struct{ ID graphql.ID }) (*ArticleResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid article id")
	}
	authorID, _ := uuid.Parse(claims.UserID)
	article, err := r.svc.PublishArticle(ctx, id, authorID)
	if err != nil {
		return nil, err
	}
	return &ArticleResolver{article: article, svc: r.svc}, nil
}

func (r *Resolver) DeleteArticle(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return false, fmt.Errorf("invalid article id")
	}
	authorID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.DeleteArticle(ctx, id, authorID)
}

type BoardSettingsInput struct {
	Name          *string
	Description   *string
	DefaultAccess *string
}

func (r *Resolver) UpdateBoardSettings(ctx context.Context, args struct{ Input BoardSettingsInput }) (*BoardResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	ownerID, _ := uuid.Parse(claims.UserID)
	board, err := r.svc.UpdateBoardSettings(ctx, ownerID, service.UpdateBoardInput{
		Name:          args.Input.Name,
		Description:   args.Input.Description,
		DefaultAccess: args.Input.DefaultAccess,
	})
	if err != nil {
		return nil, err
	}
	return &BoardResolver{board: board, svc: r.svc}, nil
}

type VcRequirementInput struct {
	VcType string
	Issuer string
}

type BoardVcPolicyInput struct {
	MinTrustLevel     int32
	RequireVcs        []VcRequirementInput
	MinCommentTrust   int32
	RequireCommentVcs []VcRequirementInput
}

func (r *Resolver) UpdateBoardVcPolicy(ctx context.Context, args struct{ Input BoardVcPolicyInput }) (*BoardResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	ownerID, _ := uuid.Parse(claims.UserID)
	board, err := r.svc.GetBoardByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("board not found: %w", err)
	}

	requireVcs := make([]db.VcRequirement, len(args.Input.RequireVcs))
	for i, v := range args.Input.RequireVcs {
		requireVcs[i] = db.VcRequirement{VcType: v.VcType, Issuer: v.Issuer}
	}
	requireCommentVcs := make([]db.VcRequirement, len(args.Input.RequireCommentVcs))
	for i, v := range args.Input.RequireCommentVcs {
		requireCommentVcs[i] = db.VcRequirement{VcType: v.VcType, Issuer: v.Issuer}
	}

	updated, err := r.svc.UpdateBoardVcPolicy(ctx, board.ID,
		int16(args.Input.MinTrustLevel), int16(args.Input.MinCommentTrust),
		requireVcs, requireCommentVcs,
	)
	if err != nil {
		return nil, err
	}
	return &BoardResolver{board: updated, svc: r.svc}, nil
}

func (r *Resolver) SubscribeBoard(ctx context.Context, args struct{ OwnerID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	ownerID, err := uuid.Parse(string(args.OwnerID))
	if err != nil {
		return false, fmt.Errorf("invalid owner id")
	}
	subscriberID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.SubscribeBoard(ctx, ownerID, subscriberID)
}

func (r *Resolver) UnsubscribeBoard(ctx context.Context, args struct{ OwnerID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	ownerID, err := uuid.Parse(string(args.OwnerID))
	if err != nil {
		return false, fmt.Errorf("invalid owner id")
	}
	subscriberID, _ := uuid.Parse(claims.UserID)
	return true, r.svc.UnsubscribeBoard(ctx, ownerID, subscriberID)
}

// ─── Type resolvers ───────────────────────────────────────────────────────────

type PostResolver struct {
	post db.Post
	svc  *service.ContentService
}

func (r *PostResolver) ID() graphql.ID       { return graphql.ID(r.post.ID.String()) }
func (r *PostResolver) AuthorId() graphql.ID { return graphql.ID(r.post.AuthorID.String()) }
func (r *PostResolver) ParentId() *graphql.ID {
	if r.post.ParentID == nil {
		return nil
	}
	s := graphql.ID(r.post.ParentID.String())
	return &s
}
func (r *PostResolver) RootId() *graphql.ID {
	if r.post.RootID == nil {
		return nil
	}
	s := graphql.ID(r.post.RootID.String())
	return &s
}
func (r *PostResolver) Kind() string         { return r.post.Kind }
func (r *PostResolver) Content() string      { return r.post.Content }
func (r *PostResolver) NoteTitle() *string   { return r.post.NoteTitle }
func (r *PostResolver) NoteCover() *string   { return r.post.NoteCover }
func (r *PostResolver) NoteSummary() *string { return r.post.NoteSummary }
func (r *PostResolver) ResharedFromId() *graphql.ID {
	if r.post.ResharedFromID == nil {
		return nil
	}
	s := graphql.ID(r.post.ResharedFromID.String())
	return &s
}
func (r *PostResolver) ReplyCount() int32    { return int32(r.post.ReplyCount) }
func (r *PostResolver) LikeCount() int32       { return int32(r.post.LikeCount) }
func (r *PostResolver) IsLiked() bool          { return r.post.IsLiked }
func (r *PostResolver) ViewerEmotion() *string { return r.post.ViewerEmotion }
func (r *PostResolver) ReactionCounts(ctx context.Context) ([]*ReactionCountResolver, error) {
	counts, err := r.svc.ListPostReactionCounts(ctx, r.post.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*ReactionCountResolver, 0, len(counts))
	for _, c := range counts {
		rc := c
		out = append(out, &ReactionCountResolver{item: rc})
	}
	return out, nil
}
func (r *PostResolver) CreatedAt() string { return r.post.CreatedAt.UTC().Format(time.RFC3339) }
func (r *PostResolver) DeletedAt() *string {
	if r.post.DeletedAt == nil {
		return nil
	}
	s := r.post.DeletedAt.UTC().Format(time.RFC3339)
	return &s
}
func (r *PostResolver) SignatureInfo() *SignatureInfoResolver {
	info := r.svc.BuildPostSignatureInfo(r.post)
	return &SignatureInfoResolver{
		isSigned:    info.IsSigned,
		isVerified:  info.IsVerified,
		contentHash: info.ContentHash,
		signature:   info.Signature,
		algorithm:   info.Algorithm,
		explanation: info.Explanation,
	}
}

type ArticleResolver struct {
	article db.Article
	svc     *service.ContentService
}

func (r *ArticleResolver) ID() graphql.ID       { return graphql.ID(r.article.ID.String()) }
func (r *ArticleResolver) BoardId() graphql.ID  { return graphql.ID(r.article.BoardID.String()) }
func (r *ArticleResolver) AuthorId() graphql.ID { return graphql.ID(r.article.AuthorID.String()) }
func (r *ArticleResolver) Title() string        { return r.article.Title }
func (r *ArticleResolver) Slug() string         { return r.article.Slug }
func (r *ArticleResolver) ContentMd() *string   { return r.article.ContentMd }
func (r *ArticleResolver) Status() string       { return r.article.Status }
func (r *ArticleResolver) AccessPolicy() string { return r.article.AccessPolicy }
func (r *ArticleResolver) PublishedAt() *string {
	if r.article.PublishedAt == nil {
		return nil
	}
	s := r.article.PublishedAt.UTC().Format(time.RFC3339)
	return &s
}
func (r *ArticleResolver) CreatedAt() string { return r.article.CreatedAt.UTC().Format(time.RFC3339) }
func (r *ArticleResolver) UpdatedAt() string { return r.article.UpdatedAt.UTC().Format(time.RFC3339) }
func (r *ArticleResolver) SignatureInfo() *SignatureInfoResolver {
	info := r.svc.BuildArticleSignatureInfo(r.article)
	return &SignatureInfoResolver{
		isSigned:    info.IsSigned,
		isVerified:  info.IsVerified,
		contentHash: info.ContentHash,
		signature:   info.Signature,
		algorithm:   info.Algorithm,
		explanation: info.Explanation,
	}
}

type SignatureInfoResolver struct {
	isSigned    bool
	isVerified  bool
	contentHash *string
	signature   *string
	algorithm   *string
	explanation string
}

func (r *SignatureInfoResolver) IsSigned() bool       { return r.isSigned }
func (r *SignatureInfoResolver) IsVerified() bool     { return r.isVerified }
func (r *SignatureInfoResolver) ContentHash() *string { return r.contentHash }
func (r *SignatureInfoResolver) Signature() *string   { return r.signature }
func (r *SignatureInfoResolver) Algorithm() *string   { return r.algorithm }
func (r *SignatureInfoResolver) Explanation() string  { return r.explanation }

type ReactionCountResolver struct {
	item db.ReactionCount
}

func (r *ReactionCountResolver) Emotion() string { return r.item.Emotion }
func (r *ReactionCountResolver) Count() int32    { return int32(r.item.Count) }

type ArticleCommentResolver struct {
	comment db.ArticleComment
}

func (r *ArticleCommentResolver) ID() graphql.ID { return graphql.ID(r.comment.ID.String()) }
func (r *ArticleCommentResolver) ArticleId() graphql.ID {
	return graphql.ID(r.comment.ArticleID.String())
}
func (r *ArticleCommentResolver) AuthorId() graphql.ID {
	return graphql.ID(r.comment.AuthorID.String())
}
func (r *ArticleCommentResolver) ParentId() *graphql.ID {
	if r.comment.ParentID == nil {
		return nil
	}
	id := graphql.ID(r.comment.ParentID.String())
	return &id
}
func (r *ArticleCommentResolver) Content() string { return r.comment.Content }
func (r *ArticleCommentResolver) CreatedAt() string {
	return r.comment.CreatedAt.UTC().Format(time.RFC3339)
}

type BoardResolver struct {
	board db.Board
	svc   *service.ContentService
}

func (r *BoardResolver) ID() graphql.ID          { return graphql.ID(r.board.ID.String()) }
func (r *BoardResolver) OwnerId() graphql.ID     { return graphql.ID(r.board.OwnerID.String()) }
func (r *BoardResolver) Name() string            { return r.board.Name }
func (r *BoardResolver) Description() *string    { return r.board.Description }
func (r *BoardResolver) DefaultAccess() string   { return r.board.DefaultAccess }
func (r *BoardResolver) MinTrustLevel() int32    { return int32(r.board.MinTrustLevel) }
func (r *BoardResolver) CommentPolicy() string   { return r.board.CommentPolicy }
func (r *BoardResolver) MinCommentTrust() int32  { return int32(r.board.MinCommentTrust) }
func (r *BoardResolver) CreatedAt() string       { return r.board.CreatedAt.UTC().Format(time.RFC3339) }

func (r *BoardResolver) RequireVcs() []*VcRequirementResolver {
	out := make([]*VcRequirementResolver, len(r.board.RequireVcs))
	for i, v := range r.board.RequireVcs {
		v := v
		out[i] = &VcRequirementResolver{req: v}
	}
	return out
}

func (r *BoardResolver) RequireCommentVcs() []*VcRequirementResolver {
	out := make([]*VcRequirementResolver, len(r.board.RequireCommentVcs))
	for i, v := range r.board.RequireCommentVcs {
		v := v
		out[i] = &VcRequirementResolver{req: v}
	}
	return out
}

type VcRequirementResolver struct{ req db.VcRequirement }

func (r *VcRequirementResolver) VcType() string { return r.req.VcType }
func (r *VcRequirementResolver) Issuer() string { return r.req.Issuer }

func (r *BoardResolver) SubscriberCount(ctx context.Context) (int32, error) {
	count, err := r.svc.CountBoardSubscribers(ctx, r.board.ID)
	return int32(count), err
}

func (r *BoardResolver) IsSubscribed(ctx context.Context) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, nil
	}
	viewerID := parseOptionalUUID(claims.UserID)
	if viewerID == nil {
		return false, nil
	}
	return r.svc.IsSubscribedToBoard(ctx, r.board.ID, *viewerID)
}

// ─── Connection resolvers ─────────────────────────────────────────────────────

type PostConnectionResolver struct {
	posts      []db.Post
	hasMore    bool
	nextCursor *string
	svc        *service.ContentService
}

func newPostConnection(posts []db.Post, limit int, svc *service.ContentService) *PostConnectionResolver {
	hasMore := len(posts) == limit
	var nextCursor *string
	if hasMore && len(posts) > 0 {
		s := posts[len(posts)-1].ID.String()
		nextCursor = &s
	}
	return &PostConnectionResolver{posts: posts, hasMore: hasMore, nextCursor: nextCursor, svc: svc}
}

func (r *PostConnectionResolver) Items() []*PostResolver {
	res := make([]*PostResolver, len(r.posts))
	for i := range r.posts {
		res[i] = &PostResolver{post: r.posts[i], svc: r.svc}
	}
	return res
}
func (r *PostConnectionResolver) NextCursor() *string { return r.nextCursor }
func (r *PostConnectionResolver) HasMore() bool       { return r.hasMore }

type ArticleConnectionResolver struct {
	articles   []db.Article
	hasMore    bool
	nextCursor *string
	svc        *service.ContentService
}

func newArticleConnection(articles []db.Article, limit int, svc *service.ContentService) *ArticleConnectionResolver {
	hasMore := len(articles) == limit
	var nextCursor *string
	if hasMore && len(articles) > 0 {
		s := articles[len(articles)-1].ID.String()
		nextCursor = &s
	}
	return &ArticleConnectionResolver{articles: articles, hasMore: hasMore, nextCursor: nextCursor, svc: svc}
}

func (r *ArticleConnectionResolver) Items() []*ArticleResolver {
	res := make([]*ArticleResolver, len(r.articles))
	for i := range r.articles {
		res[i] = &ArticleResolver{article: r.articles[i], svc: r.svc}
	}
	return res
}
func (r *ArticleConnectionResolver) NextCursor() *string { return r.nextCursor }
func (r *ArticleConnectionResolver) HasMore() bool       { return r.hasMore }

// ─── Federation notification ──────────────────────────────────────────────────

// notifyFederation calls POST {federationURL}/internal/post-created in a goroutine.
// Failures are logged but do not affect the caller.
func (r *Resolver) notifyFederation(username string, post db.Post) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := struct {
		AuthorUsername string `json:"authorUsername"`
		PostID         string `json:"postId"`
		Content        string `json:"content"`
		CreatedAt      string `json:"createdAt"`
	}{
		AuthorUsername: username,
		PostID:         post.ID.String(),
		Content:        post.Content,
		CreatedAt:      post.CreatedAt.UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("federation notify: marshal failed")
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.federationURL+"/internal/post-created", bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Msg("federation notify: build request failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("username", username).Msg("federation notify: request failed")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Str("username", username).Msg("federation notify: unexpected status")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func parseOptionalUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}
