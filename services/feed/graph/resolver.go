package graph

import (
	"context"
	_ "embed"

	graphql "github.com/graph-gophers/graphql-go"
	"github.com/google/uuid"

	"github.com/aleth/feed/internal/db"
	"github.com/aleth/feed/internal/service"
)

//go:embed schema.graphqls
var schemaString string

// NewSchema builds the executable GraphQL schema for the feed service.
func NewSchema(svc *service.FeedService) *graphql.Schema {
	r := &Resolver{svc: svc}
	return graphql.MustParseSchema(schemaString, r, graphql.UseStringDescriptions())
}

// ─── Context ──────────────────────────────────────────────────────────────────

type ctxKey int

const ctxViewerID ctxKey = iota

// WithViewerID stores the authenticated viewer UUID in the context.
func WithViewerID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxViewerID, id)
}

func viewerIDFromCtx(ctx context.Context) *uuid.UUID {
	id, ok := ctx.Value(ctxViewerID).(uuid.UUID)
	if !ok {
		return nil
	}
	return &id
}

// ─── Root Resolver ────────────────────────────────────────────────────────────

// Resolver is the root GraphQL resolver for the feed service.
type Resolver struct {
	svc *service.FeedService
}

func (r *Resolver) Feed(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*FeedConnectionResolver, error) {
	viewerID := viewerIDFromCtx(ctx)
	limit := 0
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	result, err := r.svc.GetFeed(ctx, viewerID, args.After, limit)
	if err != nil {
		return &FeedConnectionResolver{}, nil // degrade gracefully
	}
	return newFeedConnectionResolver(result), nil
}

func (r *Resolver) PostFriendReactors(ctx context.Context, args struct {
	PostId graphql.ID
	Limit  *int32
}) ([]*FriendReactorResolver, error) {
	viewerID := viewerIDFromCtx(ctx)
	if viewerID == nil {
		return []*FriendReactorResolver{}, nil // unauthenticated → empty
	}
	postID, err := uuid.Parse(string(args.PostId))
	if err != nil {
		return []*FriendReactorResolver{}, nil
	}
	limit := 0
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	reactors, err := r.svc.GetFriendReactors(ctx, *viewerID, postID, limit)
	if err != nil {
		return []*FriendReactorResolver{}, nil // degrade gracefully
	}
	out := make([]*FriendReactorResolver, len(reactors))
	for i, rc := range reactors {
		rc := rc
		out[i] = &FriendReactorResolver{item: rc}
	}
	return out, nil
}

func (r *Resolver) ExploreFeed(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*FeedConnectionResolver, error) {
	viewerID := viewerIDFromCtx(ctx)
	limit := 0
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	result, err := r.svc.GetExploreFeed(ctx, viewerID, args.After, limit)
	if err != nil {
		return &FeedConnectionResolver{}, nil
	}
	return newFeedConnectionResolver(result), nil
}

// ─── Connection resolver ──────────────────────────────────────────────────────

type FeedConnectionResolver struct {
	items      []*FeedItemResolver
	nextCursor *string
	hasMore    bool
}

func newFeedConnectionResolver(result *service.FeedResult) *FeedConnectionResolver {
	items := make([]*FeedItemResolver, len(result.Posts))
	for i := range result.Posts {
		p := result.Posts[i]
		author := result.Authors[p.AuthorID]
		items[i] = &FeedItemResolver{
			id:    "post:" + p.ID.String(),
			typ:   "post",
			score: float64(len(result.Posts) - i),
			post:  newFeedPostResolver(p, author),
		}
	}

	var cursor *string
	if result.NextCursor != nil {
		s := result.NextCursor.String()
		cursor = &s
	}

	return &FeedConnectionResolver{
		items:      items,
		nextCursor: cursor,
		hasMore:    result.HasMore,
	}
}

func (r *FeedConnectionResolver) Items() []*FeedItemResolver { return r.items }
func (r *FeedConnectionResolver) NextCursor() *string        { return r.nextCursor }
func (r *FeedConnectionResolver) HasMore() bool              { return r.hasMore }

// ─── Item resolver ────────────────────────────────────────────────────────────

type FeedItemResolver struct {
	id      string
	typ     string
	post    *FeedPostResolver
	article *FeedArticleResolver
	score   float64
}

func (r *FeedItemResolver) ID() graphql.ID               { return graphql.ID(r.id) }
func (r *FeedItemResolver) Type() string                 { return r.typ }
func (r *FeedItemResolver) Post() *FeedPostResolver      { return r.post }
func (r *FeedItemResolver) Article() *FeedArticleResolver { return r.article }
func (r *FeedItemResolver) Score() float64               { return r.score }

// ─── Post resolver ────────────────────────────────────────────────────────────

type FeedPostResolver struct {
	post   db.FeedPost
	author db.AuthUser
}

func newFeedPostResolver(p db.FeedPost, author db.AuthUser) *FeedPostResolver {
	return &FeedPostResolver{post: p, author: author}
}

func (r *FeedPostResolver) ID() graphql.ID             { return graphql.ID(r.post.ID.String()) }
func (r *FeedPostResolver) AuthorId() graphql.ID       { return graphql.ID(r.post.AuthorID.String()) }
func (r *FeedPostResolver) AuthorUsername() string     { return r.author.Username }
func (r *FeedPostResolver) AuthorDisplayName() *string { return r.author.DisplayName }
func (r *FeedPostResolver) AuthorTrustLevel() int32    { return r.author.TrustLevel }
func (r *FeedPostResolver) IsSigned() bool             { return r.post.IsSigned }
func (r *FeedPostResolver) Content() string            { return r.post.Content }
func (r *FeedPostResolver) Kind() string               { return r.post.Kind }
func (r *FeedPostResolver) NoteTitle() *string         { return r.post.NoteTitle }
func (r *FeedPostResolver) NoteSummary() *string       { return r.post.NoteSummary }
func (r *FeedPostResolver) CommentCount() int32 { return r.post.CommentCount }
func (r *FeedPostResolver) ReactionCounts() []*FeedReactionCountResolver {
	out := make([]*FeedReactionCountResolver, 0, len(r.post.ReactionCounts))
	for emotion, count := range r.post.ReactionCounts {
		e, c := emotion, count
		out = append(out, &FeedReactionCountResolver{emotion: e, count: c})
	}
	return out
}
func (r *FeedPostResolver) ResharedFromId() *graphql.ID {
	if r.post.ResharedFromID == nil {
		return nil
	}
	id := graphql.ID(r.post.ResharedFromID.String())
	return &id
}
func (r *FeedPostResolver) MyEmotion() *string { return r.post.MyEmotion }
func (r *FeedPostResolver) CreatedAt() string {
	return r.post.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
}

// ─── ReactionCount resolver ───────────────────────────────────────────────────

type FeedReactionCountResolver struct {
	emotion string
	count   int32
}

func (r *FeedReactionCountResolver) Emotion() string { return r.emotion }
func (r *FeedReactionCountResolver) Count() int32    { return r.count }

// ─── FriendReactor resolver ───────────────────────────────────────────────────

type FriendReactorResolver struct {
	item db.PostReactor
}

func (r *FriendReactorResolver) ID() graphql.ID       { return graphql.ID(r.item.UserID.String()) }
func (r *FriendReactorResolver) Username() string     { return r.item.Username }
func (r *FriendReactorResolver) DisplayName() *string { return r.item.DisplayName }
func (r *FriendReactorResolver) Emotion() string      { return r.item.Emotion }

// ─── Article resolver (placeholder for future use) ───────────────────────────

type FeedArticleResolver struct{}

func (r *FeedArticleResolver) ID() graphql.ID        { return "" }
func (r *FeedArticleResolver) AuthorId() graphql.ID  { return "" }
func (r *FeedArticleResolver) AuthorUsername() string { return "" }
func (r *FeedArticleResolver) Title() string         { return "" }
func (r *FeedArticleResolver) Slug() string          { return "" }
func (r *FeedArticleResolver) PublishedAt() string   { return "" }
