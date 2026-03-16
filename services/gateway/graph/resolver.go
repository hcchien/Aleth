package graph

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	graphql "github.com/graph-gophers/graphql-go"

	"github.com/aleth/gateway/internal/client"
)

//go:embed schema.graphqls
var schemaString string

// NewSchema builds the executable GraphQL schema for the gateway.
func NewSchema(authClient *client.AuthClient, contentClient *client.ContentClient, feedClient *client.FeedClient) *graphql.Schema {
	r := &Resolver{auth: authClient, content: contentClient, feed: feedClient}
	return graphql.MustParseSchema(schemaString, r, graphql.UseStringDescriptions())
}

// ─── Context ──────────────────────────────────────────────────────────────────

type ctxKey int

const (
	ctxAuthHeader ctxKey = iota
	ctxUserClaims
)

// UserClaims holds the validated JWT payload injected by the auth middleware.
type UserClaims struct {
	UserID     string
	Username   string
	TrustLevel int
}

// WithAuthHeader stores the raw Authorization header in the context.
func WithAuthHeader(ctx context.Context, header string) context.Context {
	return context.WithValue(ctx, ctxAuthHeader, header)
}

func authHeaderFromCtx(ctx context.Context) string {
	h, _ := ctx.Value(ctxAuthHeader).(string)
	return h
}

// WithUserClaims stores validated JWT claims in the context.
func WithUserClaims(ctx context.Context, c UserClaims) context.Context {
	return context.WithValue(ctx, ctxUserClaims, c)
}

func claimsFromCtx(ctx context.Context) (UserClaims, bool) {
	c, ok := ctx.Value(ctxUserClaims).(UserClaims)
	return c, ok
}

// ─── Root Resolver ────────────────────────────────────────────────────────────

// Resolver is the root GraphQL resolver for the gateway.
type Resolver struct {
	auth    *client.AuthClient
	content *client.ContentClient
	feed    *client.FeedClient
}

// contentGQL calls the Content service GraphQL endpoint, forwarding the auth header.
func (r *Resolver) contentGQL(ctx context.Context, query string, vars map[string]any) (json.RawMessage, error) {
	return r.content.GraphQL(ctx, query, vars, authHeaderFromCtx(ctx))
}

// authGQL calls the Auth service GraphQL endpoint, forwarding the auth header.
func (r *Resolver) authGQL(ctx context.Context, query string, vars map[string]any) (json.RawMessage, error) {
	return r.auth.GraphQL(ctx, query, vars, authHeaderFromCtx(ctx))
}

// resolveUsers batch-fetches users from the Auth service for the given IDs.
// Deduplicates the input and returns a map of id → *client.User.
func (r *Resolver) resolveUsers(ctx context.Context, ids []string) (map[string]*client.User, error) {
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; !ok && id != "" {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	return r.auth.GetUsersByIDs(ctx, unique)
}

// fetchBoardByID fetches a board from Content service by board ID.
func (r *Resolver) fetchBoardByID(ctx context.Context, boardID string) (*client.ContentBoard, error) {
	data, err := r.contentGQL(ctx,
		`query($id: ID!) { boardByID(id: $id) { id ownerId name description defaultAccess subscriberCount isSubscribed createdAt } }`,
		map[string]any{"id": boardID},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		BoardByID *client.ContentBoard `json:"boardByID"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.BoardByID, nil
}

// ─── Input types ──────────────────────────────────────────────────────────────

type RegisterInput struct {
	Username string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type PasskeyAssertionInput struct {
	CredentialID      string
	ChallengeToken    string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	UserHandle        *string
	Username          *string
}

type CreatePostInput struct {
	Content string
}

type CreateNoteInput struct {
	Content     string
	NoteTitle   string
	NoteCover   *string
	NoteSummary *string
}

type CreateArticleInput struct {
	Title        string
	ContentMd    *string
	AccessPolicy string
}

type UpdateArticleInput struct {
	Title        *string
	ContentMd    *string
	AccessPolicy *string
	Status       *string
}

type ResharePostInput struct {
	Content *string
}

type BoardSettingsInput struct {
	Name          *string
	Description   *string
	DefaultAccess *string
}

// ─── Query resolvers ──────────────────────────────────────────────────────────

func (r *Resolver) Me(ctx context.Context) (*UserResolver, error) {
	data, err := r.authGQL(ctx, `{ me { id did username displayName email trustLevel createdAt } }`, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Me *client.User `json:"me"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Me == nil {
		return nil, nil
	}
	return &UserResolver{user: *resp.Me, r: r}, nil
}

func (r *Resolver) DidDocument(ctx context.Context, args struct{ DID string }) (*string, error) {
	data, err := r.authGQL(ctx,
		`query($did: String!) { didDocument(did: $did) }`,
		map[string]any{"did": args.DID},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		DidDocument *string `json:"didDocument"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.DidDocument, nil
}

func (r *Resolver) FollowStats(ctx context.Context, args struct{ UserID graphql.ID }) (*FollowStatsResolver, error) {
	data, err := r.authGQL(ctx,
		`query($userID: ID!) { followStats(userID: $userID) { followerCount followingCount isFollowing } }`,
		map[string]any{"userID": string(args.UserID)},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		FollowStats client.FollowStats `json:"followStats"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &FollowStatsResolver{stats: resp.FollowStats}, nil
}

func (r *Resolver) Post(ctx context.Context, args struct{ ID graphql.ID }) (*PostResolver, error) {
	data, err := r.contentGQL(ctx,
		`query($id: ID!) {
			post(id: $id) {
				id authorId parentId rootId content replyCount likeCount isLiked createdAt
				viewerEmotion
				reactionCounts { emotion count }
				signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
			}
		}`,
		map[string]any{"id": string(args.ID)},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Post *client.ContentPost `json:"post"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Post == nil {
		return nil, nil
	}
	users, err := r.resolveUsers(ctx, []string{resp.Post.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: *resp.Post, users: users, r: r}, nil
}

func (r *Resolver) Posts(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*PostConnectionResolver, error) {
	vars := map[string]any{}
	if args.After != nil {
		vars["after"] = *args.After
	}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}
	data, err := r.contentGQL(ctx,
		`query($after: String, $limit: Int) {
			posts(after: $after, limit: $limit) {
				items {
					id authorId parentId rootId content replyCount likeCount isLiked createdAt
					viewerEmotion
					reactionCounts { emotion count }
					signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
				}
				nextCursor hasMore
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Posts client.ContentPostConnection `json:"posts"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(resp.Posts.Items))
	for _, p := range resp.Posts.Items {
		authorIDs = append(authorIDs, p.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	return &PostConnectionResolver{conn: resp.Posts, users: users, r: r}, nil
}

func (r *Resolver) Notes(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*NoteConnectionResolver, error) {
	vars := map[string]any{}
	if args.After != nil {
		vars["after"] = *args.After
	}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}
	data, err := r.contentGQL(ctx,
		`query($after: String, $limit: Int) {
			notes(after: $after, limit: $limit) {
				items { `+postFields+` }
				nextCursor hasMore
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Notes client.ContentNoteConnection `json:"notes"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(resp.Notes.Items))
	for _, p := range resp.Notes.Items {
		authorIDs = append(authorIDs, p.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	return &NoteConnectionResolver{conn: resp.Notes, users: users, r: r}, nil
}

func (r *Resolver) PostReplies(ctx context.Context, args struct {
	PostId graphql.ID
	Limit  *int32
}) ([]*PostResolver, error) {
	vars := map[string]any{"postId": string(args.PostId)}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}
	data, err := r.contentGQL(ctx,
		`query($postId: ID!, $limit: Int) { postReplies(postId: $postId, limit: $limit) { `+postFields+` } }`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		PostReplies []client.ContentPost `json:"postReplies"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(resp.PostReplies))
	for _, p := range resp.PostReplies {
		authorIDs = append(authorIDs, p.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*PostResolver, len(resp.PostReplies))
	for i, p := range resp.PostReplies {
		pc := p
		out[i] = &PostResolver{post: pc, users: users, r: r}
	}
	return out, nil
}

func (r *Resolver) Article(ctx context.Context, args struct {
	ID   *graphql.ID
	Slug *string
}) (*ArticleResolver, error) {
	if args.ID == nil {
		return nil, nil // slug-only lookup not supported in Phase 1
	}
	data, err := r.contentGQL(ctx,
		`query($id: ID!) {
			article(id: $id) {
				id boardId authorId title slug contentMd status accessPolicy publishedAt createdAt updatedAt
				signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
			}
		}`,
		map[string]any{"id": string(*args.ID)},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Article *client.ContentArticle `json:"article"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Article == nil {
		return nil, nil
	}
	users, err := r.resolveUsers(ctx, []string{resp.Article.AuthorID})
	if err != nil {
		return nil, err
	}
	board, err := r.fetchBoardByID(ctx, resp.Article.BoardID)
	if err != nil {
		return nil, err
	}
	return &ArticleResolver{article: *resp.Article, users: users, board: board, r: r}, nil
}

func (r *Resolver) Board(ctx context.Context, args struct{ Username string }) (*BoardResolver, error) {
	owner, err := r.auth.GetUserByUsername(ctx, args.Username)
	if err != nil {
		return nil, err
	}
	if owner == nil {
		return nil, nil
	}
	data, err := r.contentGQL(ctx,
		`query($ownerID: ID!) { board(ownerID: $ownerID) { `+boardFields+` } }`,
		map[string]any{"ownerID": owner.ID},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Board *client.ContentBoard `json:"board"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Board == nil {
		return nil, nil
	}
	return &BoardResolver{board: *resp.Board, owner: owner, r: r}, nil
}

func (r *Resolver) BoardArticles(ctx context.Context, args struct {
	Username string
	After    *string
	Limit    *int32
}) (*ArticleConnectionResolver, error) {
	owner, err := r.auth.GetUserByUsername(ctx, args.Username)
	if err != nil {
		return nil, err
	}
	if owner == nil {
		return &ArticleConnectionResolver{conn: client.ContentArticleConnection{}, r: r}, nil
	}

	vars := map[string]any{"ownerID": owner.ID}
	if args.After != nil {
		vars["after"] = *args.After
	}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}

	// Fetch board and articles concurrently would be ideal; sequential for Phase 1.
	boardData, err := r.contentGQL(ctx,
		`query($ownerID: ID!) { board(ownerID: $ownerID) { id ownerId name description defaultAccess subscriberCount isSubscribed createdAt } }`,
		map[string]any{"ownerID": owner.ID},
	)
	if err != nil {
		return nil, err
	}
	var boardResp struct {
		Board *client.ContentBoard `json:"board"`
	}
	json.Unmarshal(boardData, &boardResp) //nolint:errcheck

	data, err := r.contentGQL(ctx,
		`query($ownerID: ID!, $after: String, $limit: Int) {
			boardArticles(ownerID: $ownerID, after: $after, limit: $limit) {
				items {
					id boardId authorId title slug contentMd status accessPolicy publishedAt createdAt updatedAt
					signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
				}
				nextCursor hasMore
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var articlesResp struct {
		BoardArticles client.ContentArticleConnection `json:"boardArticles"`
	}
	if err := json.Unmarshal(data, &articlesResp); err != nil {
		return nil, err
	}

	authorIDs := make([]string, 0, len(articlesResp.BoardArticles.Items))
	for _, a := range articlesResp.BoardArticles.Items {
		authorIDs = append(authorIDs, a.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	return &ArticleConnectionResolver{conn: articlesResp.BoardArticles, users: users, board: boardResp.Board, r: r}, nil
}

func (r *Resolver) ArticleComments(ctx context.Context, args struct {
	ArticleId graphql.ID
	Limit     *int32
}) ([]*ArticleCommentResolver, error) {
	vars := map[string]any{"articleId": string(args.ArticleId)}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}
	data, err := r.contentGQL(ctx,
		`query($articleId: ID!, $limit: Int) {
			articleComments(articleId: $articleId, limit: $limit) {
				id articleId authorId parentId content createdAt
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ArticleComments []client.ContentArticleComment `json:"articleComments"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(resp.ArticleComments))
	for _, c := range resp.ArticleComments {
		authorIDs = append(authorIDs, c.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*ArticleCommentResolver, 0, len(resp.ArticleComments))
	for _, c := range resp.ArticleComments {
		item := c
		out = append(out, &ArticleCommentResolver{comment: item, users: users, r: r})
	}
	return out, nil
}

func (r *Resolver) CommentReplies(ctx context.Context, args struct {
	CommentId graphql.ID
	Limit     *int32
}) ([]*ArticleCommentResolver, error) {
	vars := map[string]any{"commentId": string(args.CommentId)}
	if args.Limit != nil {
		vars["limit"] = *args.Limit
	}
	data, err := r.contentGQL(ctx,
		`query($commentId: ID!, $limit: Int) {
			commentReplies(commentId: $commentId, limit: $limit) {
				id articleId authorId parentId content createdAt
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		CommentReplies []client.ContentArticleComment `json:"commentReplies"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(resp.CommentReplies))
	for _, c := range resp.CommentReplies {
		authorIDs = append(authorIDs, c.AuthorID)
	}
	users, err := r.resolveUsers(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*ArticleCommentResolver, 0, len(resp.CommentReplies))
	for _, c := range resp.CommentReplies {
		item := c
		out = append(out, &ArticleCommentResolver{comment: item, users: users, r: r})
	}
	return out, nil
}

func (r *Resolver) Feed(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*FeedConnectionResolver, error) {
	conn, err := r.feed.GetFeed(ctx, args.After, args.Limit, authHeaderFromCtx(ctx))
	if err != nil {
		return &FeedConnectionResolver{}, nil // degrade gracefully
	}
	return r.feedConnToResolver(conn), nil
}

func (r *Resolver) ExploreFeed(ctx context.Context, args struct {
	After *string
	Limit *int32
}) (*FeedConnectionResolver, error) {
	conn, err := r.feed.GetExploreFeed(ctx, args.After, args.Limit, authHeaderFromCtx(ctx))
	if err != nil {
		return &FeedConnectionResolver{}, nil
	}
	return r.feedConnToResolver(conn), nil
}

func (r *Resolver) PostFriendReactors(ctx context.Context, args struct {
	PostId graphql.ID
	Limit  *int32
}) ([]*FriendReactorResolver, error) {
	reactors, err := r.feed.GetFriendReactors(ctx, string(args.PostId), args.Limit, authHeaderFromCtx(ctx))
	if err != nil || len(reactors) == 0 {
		return []*FriendReactorResolver{}, nil
	}
	out := make([]*FriendReactorResolver, len(reactors))
	for i, rc := range reactors {
		rc := rc
		out[i] = &FriendReactorResolver{item: rc}
	}
	return out, nil
}

// FriendReactorResolver wraps a GatewayFriendReactor for GraphQL.
type FriendReactorResolver struct {
	item client.GatewayFriendReactor
}

func (r *FriendReactorResolver) ID() graphql.ID       { return graphql.ID(r.item.ID) }
func (r *FriendReactorResolver) Username() string     { return r.item.Username }
func (r *FriendReactorResolver) DisplayName() *string { return r.item.DisplayName }
func (r *FriendReactorResolver) Emotion() string      { return r.item.Emotion }

// feedConnToResolver maps a GatewayFeedConnection from the feed service into
// gateway resolvers, reusing PostResolver with pre-loaded author data.
func (r *Resolver) feedConnToResolver(conn *client.GatewayFeedConnection) *FeedConnectionResolver {
	items := make([]*FeedItemResolver, 0, len(conn.Items))
	for _, item := range conn.Items {
		if item.Post == nil {
			continue
		}
		fp := item.Post
		// Map per-emotion reaction counts from the feed service into ContentPost format.
		reactionCounts := make([]client.ContentReactionCount, len(fp.ReactionCounts))
		var totalLikes int32
		for i, rc := range fp.ReactionCounts {
			reactionCounts[i] = client.ContentReactionCount{Emotion: rc.Emotion, Count: rc.Count}
			totalLikes += rc.Count
		}
		// Build a ContentPost from the feed service's FeedPost so we can reuse PostResolver.
		signatureExplanation := "not signed"
		if fp.IsSigned {
			signatureExplanation = "signed by author"
		}
		cp := client.ContentPost{
			ID:             fp.ID,
			AuthorID:       fp.AuthorID,
			Content:        fp.Content,
			Kind:           fp.Kind,
			NoteTitle:      fp.NoteTitle,
			NoteSummary:    fp.NoteSummary,
			ResharedFromID: fp.ResharedFromID,
			ReplyCount:     fp.CommentCount,
			LikeCount:      totalLikes,
			ViewerEmotion:  fp.MyEmotion,
			ReactionCounts: reactionCounts,
			CreatedAt:      fp.CreatedAt,
			SignatureInfo: client.ContentSignatureInfo{
				IsSigned:    fp.IsSigned,
				IsVerified:  fp.IsSigned, // feed service confirms signature presence; treat as verified
				Explanation: signatureExplanation,
			},
		}
		// Build a minimal User from pre-loaded author data to avoid an auth service round-trip.
		user := &client.User{
			ID:          fp.AuthorID,
			Username:    fp.AuthorUsername,
			DisplayName: fp.AuthorDisplayName,
			TrustLevel:  fp.AuthorTrustLevel,
		}
		users := map[string]*client.User{fp.AuthorID: user}
		items = append(items, &FeedItemResolver{
			id:    item.ID,
			typ:   item.Type,
			score: item.Score,
			post:  &PostResolver{post: cp, users: users, r: r},
		})
	}
	return &FeedConnectionResolver{
		items:      items,
		nextCursor: conn.NextCursor,
		hasMore:    conn.HasMore,
	}
}

// ─── Auth mutation resolvers ──────────────────────────────────────────────────

const authUserFields = `id did username displayName email trustLevel createdAt`
const authPayloadFields = `accessToken refreshToken user { ` + authUserFields + ` }`

func (r *Resolver) Register(ctx context.Context, args struct{ Input RegisterInput }) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($input: RegisterInput!) { register(input: $input) { `+authPayloadFields+` } }`,
		map[string]any{"input": map[string]any{
			"username": args.Input.Username,
			"email":    args.Input.Email,
			"password": args.Input.Password,
		}},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Register client.AuthGQLPayload `json:"register"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.Register, r: r}, nil
}

func (r *Resolver) Login(ctx context.Context, args struct{ Input LoginInput }) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($input: LoginInput!) { login(input: $input) { `+authPayloadFields+` } }`,
		map[string]any{"input": map[string]any{
			"email":    args.Input.Email,
			"password": args.Input.Password,
		}},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Login client.AuthGQLPayload `json:"login"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.Login, r: r}, nil
}

func (r *Resolver) LoginWithGoogle(ctx context.Context, args struct{ IDToken string }) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($idToken: String!) { loginWithGoogle(idToken: $idToken) { `+authPayloadFields+` } }`,
		map[string]any{"idToken": args.IDToken},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		LoginWithGoogle client.AuthGQLPayload `json:"loginWithGoogle"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.LoginWithGoogle, r: r}, nil
}

func (r *Resolver) LoginWithFacebook(ctx context.Context, args struct{ AccessToken string }) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($accessToken: String!) { loginWithFacebook(accessToken: $accessToken) { `+authPayloadFields+` } }`,
		map[string]any{"accessToken": args.AccessToken},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		LoginWithFacebook client.AuthGQLPayload `json:"loginWithFacebook"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.LoginWithFacebook, r: r}, nil
}

func (r *Resolver) BeginPasskeyLogin(ctx context.Context, args struct{ Username *string }) (*PasskeyLoginOptionsResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($username: String) {
			beginPasskeyLogin(username: $username) {
				challenge challengeToken rpId timeoutMs allowCredentialIds
			}
		}`,
		map[string]any{"username": args.Username},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		BeginPasskeyLogin passkeyLoginOptions `json:"beginPasskeyLogin"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &PasskeyLoginOptionsResolver{options: resp.BeginPasskeyLogin}, nil
}

func (r *Resolver) FinishPasskeyLogin(ctx context.Context, args struct{ Input PasskeyAssertionInput }) (*AuthPayloadResolver, error) {
	input := map[string]any{
		"credentialID":      args.Input.CredentialID,
		"challengeToken":    args.Input.ChallengeToken,
		"clientDataJSON":    args.Input.ClientDataJSON,
		"authenticatorData": args.Input.AuthenticatorData,
		"signature":         args.Input.Signature,
		"userHandle":        args.Input.UserHandle,
		"username":          args.Input.Username,
	}
	data, err := r.authGQL(ctx,
		`mutation($input: PasskeyAssertionInput!) { finishPasskeyLogin(input: $input) { `+authPayloadFields+` } }`,
		map[string]any{"input": input},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		FinishPasskeyLogin client.AuthGQLPayload `json:"finishPasskeyLogin"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.FinishPasskeyLogin, r: r}, nil
}

func (r *Resolver) RegisterPasskey(ctx context.Context, args struct {
	CredentialID        string
	CredentialPublicKey string
	SignCount           int32
}) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($credentialID: String!, $credentialPublicKey: String!, $signCount: Int!) {
			registerPasskey(
				credentialID: $credentialID
				credentialPublicKey: $credentialPublicKey
				signCount: $signCount
			) { `+authPayloadFields+` }
		}`,
		map[string]any{
			"credentialID":        args.CredentialID,
			"credentialPublicKey": args.CredentialPublicKey,
			"signCount":           args.SignCount,
		},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RegisterPasskey client.AuthGQLPayload `json:"registerPasskey"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.RegisterPasskey, r: r}, nil
}

func (r *Resolver) RefreshToken(ctx context.Context, args struct{ Token string }) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($token: String!) { refreshToken(token: $token) { `+authPayloadFields+` } }`,
		map[string]any{"token": args.Token},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RefreshToken client.AuthGQLPayload `json:"refreshToken"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{payload: resp.RefreshToken, r: r}, nil
}

func (r *Resolver) RevokeToken(ctx context.Context) (bool, error) {
	data, err := r.authGQL(ctx, `mutation { revokeToken }`, nil)
	if err != nil {
		return false, err
	}
	var resp struct {
		RevokeToken bool `json:"revokeToken"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.RevokeToken, nil
}

func (r *Resolver) FollowUser(ctx context.Context, args struct{ UserID graphql.ID }) (bool, error) {
	data, err := r.authGQL(ctx,
		`mutation($userID: ID!) { followUser(userID: $userID) }`,
		map[string]any{"userID": string(args.UserID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		FollowUser bool `json:"followUser"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}
	return resp.FollowUser, nil
}

func (r *Resolver) UnfollowUser(ctx context.Context, args struct{ UserID graphql.ID }) (bool, error) {
	data, err := r.authGQL(ctx,
		`mutation($userID: ID!) { unfollowUser(userID: $userID) }`,
		map[string]any{"userID": string(args.UserID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		UnfollowUser bool `json:"unfollowUser"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}
	return resp.UnfollowUser, nil
}

// ─── Content mutation resolvers ───────────────────────────────────────────────

const postFields = `id authorId parentId rootId kind content noteTitle noteCover noteSummary resharedFromId replyCount likeCount isLiked viewerEmotion reactionCounts { emotion count } createdAt signatureInfo { isSigned isVerified contentHash signature algorithm explanation }`
const articleFields = `id boardId authorId title slug contentMd status accessPolicy publishedAt createdAt updatedAt signatureInfo { isSigned isVerified contentHash signature algorithm explanation }`
const boardFields = `id ownerId name description defaultAccess minTrustLevel commentPolicy minCommentTrust requireVcs { vcType issuer } requireCommentVcs { vcType issuer } subscriberCount isSubscribed createdAt`

func (r *Resolver) CreatePost(ctx context.Context, args struct{ Input CreatePostInput }) (*PostResolver, error) {
	data, err := r.contentGQL(ctx,
		`mutation($input: CreatePostInput!) { createPost(input: $input) { `+postFields+` } }`,
		map[string]any{"input": map[string]any{"content": args.Input.Content}},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		CreatePost client.ContentPost `json:"createPost"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	users, err := r.resolveUsers(ctx, []string{resp.CreatePost.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: resp.CreatePost, users: users, r: r}, nil
}

func (r *Resolver) CreateNote(ctx context.Context, args struct{ Input CreateNoteInput }) (*PostResolver, error) {
	inputMap := map[string]any{
		"content":   args.Input.Content,
		"noteTitle": args.Input.NoteTitle,
	}
	if args.Input.NoteCover != nil {
		inputMap["noteCover"] = *args.Input.NoteCover
	}
	if args.Input.NoteSummary != nil {
		inputMap["noteSummary"] = *args.Input.NoteSummary
	}
	data, err := r.contentGQL(ctx,
		`mutation($input: CreateNoteInput!) { createNote(input: $input) { `+postFields+` } }`,
		map[string]any{"input": inputMap},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		CreateNote client.ContentPost `json:"createNote"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	users, err := r.resolveUsers(ctx, []string{resp.CreateNote.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: resp.CreateNote, users: users, r: r}, nil
}

func (r *Resolver) ResharePost(ctx context.Context, args struct {
	PostId graphql.ID
	Input  ResharePostInput
}) (*PostResolver, error) {
	inputMap := map[string]any{}
	if args.Input.Content != nil {
		inputMap["content"] = *args.Input.Content
	}
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!, $input: ResharePostInput!) { resharePost(postId: $postId, input: $input) { `+postFields+` } }`,
		map[string]any{
			"postId": string(args.PostId),
			"input":  inputMap,
		},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ResharePost client.ContentPost `json:"resharePost"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	users, err := r.resolveUsers(ctx, []string{resp.ResharePost.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: resp.ResharePost, users: users, r: r}, nil
}

func (r *Resolver) ReplyPost(ctx context.Context, args struct {
	PostId graphql.ID
	Input  CreatePostInput
}) (*PostResolver, error) {
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!, $input: CreatePostInput!) { replyPost(postId: $postId, input: $input) { `+postFields+` } }`,
		map[string]any{
			"postId": string(args.PostId),
			"input":  map[string]any{"content": args.Input.Content},
		},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ReplyPost client.ContentPost `json:"replyPost"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	users, err := r.resolveUsers(ctx, []string{resp.ReplyPost.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: resp.ReplyPost, users: users, r: r}, nil
}

func (r *Resolver) DeletePost(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($id: ID!) { deletePost(id: $id) }`,
		map[string]any{"id": string(args.ID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		DeletePost bool `json:"deletePost"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.DeletePost, nil
}

func (r *Resolver) LikePost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!) { likePost(postId: $postId) }`,
		map[string]any{"postId": string(args.PostId)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		LikePost bool `json:"likePost"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.LikePost, nil
}

func (r *Resolver) UnlikePost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!) { unlikePost(postId: $postId) }`,
		map[string]any{"postId": string(args.PostId)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		UnlikePost bool `json:"unlikePost"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.UnlikePost, nil
}

func (r *Resolver) ReactPost(ctx context.Context, args struct {
	PostId  graphql.ID
	Emotion string
}) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!, $emotion: String!) { reactPost(postId: $postId, emotion: $emotion) }`,
		map[string]any{"postId": string(args.PostId), "emotion": args.Emotion},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		ReactPost bool `json:"reactPost"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}
	return resp.ReactPost, nil
}

func (r *Resolver) UnreactPost(ctx context.Context, args struct{ PostId graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($postId: ID!) { unreactPost(postId: $postId) }`,
		map[string]any{"postId": string(args.PostId)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		UnreactPost bool `json:"unreactPost"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}
	return resp.UnreactPost, nil
}

// ─── Board write-policy enforcement ──────────────────────────────────────────

// checkBoardWritePolicy verifies that the authenticated viewer satisfies the
// board's trust level and VC requirements for writing articles.
// Pass forComment=true to check the comment-write policy instead.
func (r *Resolver) checkBoardWritePolicy(ctx context.Context, board client.ContentBoard, forComment bool) error {
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		return fmt.Errorf("not authenticated")
	}

	var minTrust int
	var requireVcs []client.ContentVcRequirement
	if forComment {
		minTrust = int(board.MinCommentTrust)
		requireVcs = board.RequireCommentVcs
	} else {
		minTrust = int(board.MinTrustLevel)
		requireVcs = board.RequireVcs
	}

	// 1. Trust-level check
	if claims.TrustLevel < minTrust {
		return fmt.Errorf("insufficient trust level: need %d, have %d", minTrust, claims.TrustLevel)
	}

	// 2. VC check — fast path: no VCs required
	if len(requireVcs) == 0 {
		return nil
	}

	// 3. Fetch viewer's VCs from auth service
	vcs, err := r.auth.GetUserVCs(ctx, authHeaderFromCtx(ctx))
	if err != nil {
		return fmt.Errorf("fetch user VCs: %w", err)
	}

	// Build a set of (vcType, issuer) the user holds (non-revoked, non-expired)
	type vcKey struct{ vcType, issuer string }
	held := make(map[vcKey]bool, len(vcs))
	for _, vc := range vcs {
		if vc.RevokedAt == nil {
			held[vcKey{vc.VcType, vc.Issuer}] = true
		}
	}

	for _, req := range requireVcs {
		if !held[vcKey{req.VcType, req.Issuer}] {
			return fmt.Errorf("missing required credential: %s from %s", req.VcType, req.Issuer)
		}
	}
	return nil
}

func (r *Resolver) CreateArticleComment(ctx context.Context, args struct {
	ArticleId       graphql.ID
	Content         string
	ParentCommentId *graphql.ID
}) (*ArticleCommentResolver, error) {
	// Fetch the article to get its boardId, then enforce the comment-write policy.
	articleData, err := r.contentGQL(ctx,
		`query($id: ID!) { article(id: $id) { boardId } }`,
		map[string]any{"id": string(args.ArticleId)},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch article: %w", err)
	}
	var articleResp struct {
		Article *struct {
			BoardID string `json:"boardId"`
		} `json:"article"`
	}
	if err := json.Unmarshal(articleData, &articleResp); err != nil || articleResp.Article == nil {
		return nil, fmt.Errorf("article not found")
	}
	boardData, err := r.contentGQL(ctx,
		`query($id: ID!) { boardByID(id: $id) { `+boardFields+` } }`,
		map[string]any{"id": articleResp.Article.BoardID},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch board: %w", err)
	}
	var boardResp struct {
		Board *client.ContentBoard `json:"boardByID"`
	}
	if err := json.Unmarshal(boardData, &boardResp); err != nil || boardResp.Board == nil {
		return nil, fmt.Errorf("board not found")
	}
	if err := r.checkBoardWritePolicy(ctx, *boardResp.Board, true); err != nil {
		return nil, err
	}

	vars := map[string]any{
		"articleId": string(args.ArticleId),
		"content":   args.Content,
	}
	if args.ParentCommentId != nil {
		vars["parentCommentId"] = string(*args.ParentCommentId)
	}
	data, err := r.contentGQL(ctx,
		`mutation($articleId: ID!, $content: String!, $parentCommentId: ID) {
			createArticleComment(articleId: $articleId, content: $content, parentCommentId: $parentCommentId) {
				id articleId authorId parentId content createdAt
			}
		}`,
		vars,
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		CreateArticleComment client.ContentArticleComment `json:"createArticleComment"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	users, err := r.resolveUsers(ctx, []string{resp.CreateArticleComment.AuthorID})
	if err != nil {
		return nil, err
	}
	return &ArticleCommentResolver{comment: resp.CreateArticleComment, users: users, r: r}, nil
}

func (r *Resolver) CreateArticle(ctx context.Context, args struct{ Input CreateArticleInput }) (*ArticleResolver, error) {
	// Fetch the calling user's board to enforce the write policy.
	claims, ok := claimsFromCtx(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	boardData, err := r.contentGQL(ctx,
		`query($ownerID: ID!) { board(ownerID: $ownerID) { `+boardFields+` } }`,
		map[string]any{"ownerID": claims.UserID},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch board: %w", err)
	}
	var boardResp struct {
		Board *client.ContentBoard `json:"board"`
	}
	if err := json.Unmarshal(boardData, &boardResp); err != nil || boardResp.Board == nil {
		return nil, fmt.Errorf("board not found")
	}
	if err := r.checkBoardWritePolicy(ctx, *boardResp.Board, false); err != nil {
		return nil, err
	}

	input := map[string]any{
		"title":        args.Input.Title,
		"accessPolicy": args.Input.AccessPolicy,
	}
	if args.Input.ContentMd != nil {
		input["contentMd"] = *args.Input.ContentMd
	}
	data, err := r.contentGQL(ctx,
		`mutation($input: CreateArticleInput!) { createArticle(input: $input) { `+articleFields+` } }`,
		map[string]any{"input": input},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		CreateArticle client.ContentArticle `json:"createArticle"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return r.articleWithEnrichment(ctx, resp.CreateArticle)
}

func (r *Resolver) UpdateArticle(ctx context.Context, args struct {
	ID    graphql.ID
	Input UpdateArticleInput
}) (*ArticleResolver, error) {
	input := map[string]any{}
	if args.Input.Title != nil {
		input["title"] = *args.Input.Title
	}
	if args.Input.ContentMd != nil {
		input["contentMd"] = *args.Input.ContentMd
	}
	if args.Input.AccessPolicy != nil {
		input["accessPolicy"] = *args.Input.AccessPolicy
	}
	if args.Input.Status != nil {
		input["status"] = *args.Input.Status
	}
	data, err := r.contentGQL(ctx,
		`mutation($id: ID!, $input: UpdateArticleInput!) { updateArticle(id: $id, input: $input) { `+articleFields+` } }`,
		map[string]any{"id": string(args.ID), "input": input},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		UpdateArticle client.ContentArticle `json:"updateArticle"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return r.articleWithEnrichment(ctx, resp.UpdateArticle)
}

func (r *Resolver) PublishArticle(ctx context.Context, args struct{ ID graphql.ID }) (*ArticleResolver, error) {
	data, err := r.contentGQL(ctx,
		`mutation($id: ID!) { publishArticle(id: $id) { `+articleFields+` } }`,
		map[string]any{"id": string(args.ID)},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		PublishArticle client.ContentArticle `json:"publishArticle"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return r.articleWithEnrichment(ctx, resp.PublishArticle)
}

func (r *Resolver) DeleteArticle(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($id: ID!) { deleteArticle(id: $id) }`,
		map[string]any{"id": string(args.ID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		DeleteArticle bool `json:"deleteArticle"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.DeleteArticle, nil
}

func (r *Resolver) UpdateBoardSettings(ctx context.Context, args struct{ Input BoardSettingsInput }) (*BoardResolver, error) {
	input := map[string]any{}
	if args.Input.Name != nil {
		input["name"] = *args.Input.Name
	}
	if args.Input.Description != nil {
		input["description"] = *args.Input.Description
	}
	if args.Input.DefaultAccess != nil {
		input["defaultAccess"] = *args.Input.DefaultAccess
	}
	data, err := r.contentGQL(ctx,
		`mutation($input: BoardSettingsInput!) { updateBoardSettings(input: $input) { `+boardFields+` } }`,
		map[string]any{"input": input},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		UpdateBoardSettings client.ContentBoard `json:"updateBoardSettings"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &BoardResolver{board: resp.UpdateBoardSettings, r: r}, nil
}

// VcRequirementInput is the input type for gateway updateBoardVcPolicy.
type VcRequirementInput struct {
	VcType string
	Issuer string
}

// BoardVcPolicyInput is the input type for gateway updateBoardVcPolicy.
type BoardVcPolicyInput struct {
	MinTrustLevel     int32
	RequireVcs        []VcRequirementInput
	MinCommentTrust   int32
	RequireCommentVcs []VcRequirementInput
}

func (r *Resolver) UpdateBoardVcPolicy(ctx context.Context, args struct{ Input BoardVcPolicyInput }) (*BoardResolver, error) {
	requireVcs := make([]map[string]string, len(args.Input.RequireVcs))
	for i, v := range args.Input.RequireVcs {
		requireVcs[i] = map[string]string{"vcType": v.VcType, "issuer": v.Issuer}
	}
	requireCommentVcs := make([]map[string]string, len(args.Input.RequireCommentVcs))
	for i, v := range args.Input.RequireCommentVcs {
		requireCommentVcs[i] = map[string]string{"vcType": v.VcType, "issuer": v.Issuer}
	}
	data, err := r.contentGQL(ctx,
		`mutation($input: BoardVcPolicyInput!) { updateBoardVcPolicy(input: $input) { `+boardFields+` } }`,
		map[string]any{"input": map[string]any{
			"minTrustLevel":     args.Input.MinTrustLevel,
			"requireVcs":        requireVcs,
			"minCommentTrust":   args.Input.MinCommentTrust,
			"requireCommentVcs": requireCommentVcs,
		}},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		UpdateBoardVcPolicy client.ContentBoard `json:"updateBoardVcPolicy"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &BoardResolver{board: resp.UpdateBoardVcPolicy, r: r}, nil
}

func (r *Resolver) SubscribeBoard(ctx context.Context, args struct{ OwnerID graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($ownerID: ID!) { subscribeBoard(ownerID: $ownerID) }`,
		map[string]any{"ownerID": string(args.OwnerID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		SubscribeBoard bool `json:"subscribeBoard"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.SubscribeBoard, nil
}

func (r *Resolver) UnsubscribeBoard(ctx context.Context, args struct{ OwnerID graphql.ID }) (bool, error) {
	data, err := r.contentGQL(ctx,
		`mutation($ownerID: ID!) { unsubscribeBoard(ownerID: $ownerID) }`,
		map[string]any{"ownerID": string(args.OwnerID)},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		UnsubscribeBoard bool `json:"unsubscribeBoard"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.UnsubscribeBoard, nil
}

// articleWithEnrichment fetches the author and board for an article and returns an ArticleResolver.
func (r *Resolver) articleWithEnrichment(ctx context.Context, article client.ContentArticle) (*ArticleResolver, error) {
	users, err := r.resolveUsers(ctx, []string{article.AuthorID})
	if err != nil {
		return nil, err
	}
	board, err := r.fetchBoardByID(ctx, article.BoardID)
	if err != nil {
		return nil, err
	}
	return &ArticleResolver{article: article, users: users, board: board, r: r}, nil
}

// ─── Type resolvers ───────────────────────────────────────────────────────────

// UserResolver resolves the User GraphQL type.
type UserResolver struct {
	user client.User
	r    *Resolver
}

func (ur *UserResolver) ID() graphql.ID       { return graphql.ID(ur.user.ID) }
func (ur *UserResolver) DID() string          { return ur.user.DID }
func (ur *UserResolver) Username() string     { return ur.user.Username }
func (ur *UserResolver) DisplayName() *string { return ur.user.DisplayName }
func (ur *UserResolver) Email() *string       { return ur.user.Email }
func (ur *UserResolver) TrustLevel() int32    { return ur.user.TrustLevel }
func (ur *UserResolver) ApEnabled() bool      { return ur.user.APEnabled }
func (ur *UserResolver) CreatedAt() string    { return ur.user.CreatedAt }

func (ur *UserResolver) Board(ctx context.Context) (*BoardResolver, error) {
	data, err := ur.r.contentGQL(ctx,
		`query($ownerID: ID!) { board(ownerID: $ownerID) { `+boardFields+` } }`,
		map[string]any{"ownerID": ur.user.ID},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Board *client.ContentBoard `json:"board"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Board == nil {
		return nil, nil
	}
	return &BoardResolver{board: *resp.Board, owner: &ur.user, r: ur.r}, nil
}

// PostResolver resolves the Post GraphQL type.
type PostResolver struct {
	post  client.ContentPost
	users map[string]*client.User
	r     *Resolver
}

func (pr *PostResolver) ID() graphql.ID { return graphql.ID(pr.post.ID) }
func (pr *PostResolver) ParentId() *graphql.ID {
	if pr.post.ParentID == nil {
		return nil
	}
	id := graphql.ID(*pr.post.ParentID)
	return &id
}
func (pr *PostResolver) RootId() *graphql.ID {
	if pr.post.RootID == nil {
		return nil
	}
	id := graphql.ID(*pr.post.RootID)
	return &id
}
func (pr *PostResolver) Kind() string         { return pr.post.Kind }
func (pr *PostResolver) NoteTitle() *string   { return pr.post.NoteTitle }
func (pr *PostResolver) NoteCover() *string   { return pr.post.NoteCover }
func (pr *PostResolver) NoteSummary() *string { return pr.post.NoteSummary }
func (pr *PostResolver) ResharedFromId() *graphql.ID {
	if pr.post.ResharedFromID == nil {
		return nil
	}
	id := graphql.ID(*pr.post.ResharedFromID)
	return &id
}
func (pr *PostResolver) ResharedFrom(ctx context.Context) (*PostResolver, error) {
	if pr.post.ResharedFromID == nil {
		return nil, nil
	}
	data, err := pr.r.contentGQL(ctx,
		`query($id: ID!) { post(id: $id) { `+postFields+` } }`,
		map[string]any{"id": *pr.post.ResharedFromID},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Post *client.ContentPost `json:"post"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Post == nil {
		return nil, nil
	}
	users, err := pr.r.resolveUsers(ctx, []string{resp.Post.AuthorID})
	if err != nil {
		return nil, err
	}
	return &PostResolver{post: *resp.Post, users: users, r: pr.r}, nil
}
func (pr *PostResolver) Content() string        { return pr.post.Content }
func (pr *PostResolver) ReplyCount() int32      { return pr.post.ReplyCount }
func (pr *PostResolver) LikeCount() int32       { return pr.post.LikeCount }
func (pr *PostResolver) IsLiked() bool          { return pr.post.IsLiked }
func (pr *PostResolver) ViewerEmotion() *string { return pr.post.ViewerEmotion }
func (pr *PostResolver) ReactionCounts() []*ReactionCountResolver {
	out := make([]*ReactionCountResolver, 0, len(pr.post.ReactionCounts))
	for _, item := range pr.post.ReactionCounts {
		i := item
		out = append(out, &ReactionCountResolver{item: i})
	}
	return out
}
func (pr *PostResolver) CreatedAt() string { return pr.post.CreatedAt }
func (pr *PostResolver) SignatureInfo() *SignatureInfoResolver {
	return &SignatureInfoResolver{info: pr.post.SignatureInfo}
}

func (pr *PostResolver) Author(ctx context.Context) (*UserResolver, error) {
	u, ok := pr.users[pr.post.AuthorID]
	if !ok {
		users, err := pr.r.resolveUsers(ctx, []string{pr.post.AuthorID})
		if err != nil {
			return nil, err
		}
		u = users[pr.post.AuthorID]
		if u == nil {
			return nil, fmt.Errorf("author not found: %s", pr.post.AuthorID)
		}
	}
	return &UserResolver{user: *u, r: pr.r}, nil
}

// ArticleResolver resolves the Article GraphQL type.
type ArticleResolver struct {
	article client.ContentArticle
	users   map[string]*client.User
	board   *client.ContentBoard
	r       *Resolver
}

func (ar *ArticleResolver) ID() graphql.ID       { return graphql.ID(ar.article.ID) }
func (ar *ArticleResolver) Title() string        { return ar.article.Title }
func (ar *ArticleResolver) Slug() string         { return ar.article.Slug }
func (ar *ArticleResolver) ContentMd() *string   { return ar.article.ContentMd }
func (ar *ArticleResolver) Status() string       { return ar.article.Status }
func (ar *ArticleResolver) AccessPolicy() string { return ar.article.AccessPolicy }
func (ar *ArticleResolver) PublishedAt() *string { return ar.article.PublishedAt }
func (ar *ArticleResolver) CreatedAt() string    { return ar.article.CreatedAt }
func (ar *ArticleResolver) UpdatedAt() string    { return ar.article.UpdatedAt }
func (ar *ArticleResolver) SignatureInfo() *SignatureInfoResolver {
	return &SignatureInfoResolver{info: ar.article.SignatureInfo}
}

func (ar *ArticleResolver) Author(ctx context.Context) (*UserResolver, error) {
	u, ok := ar.users[ar.article.AuthorID]
	if !ok {
		users, err := ar.r.resolveUsers(ctx, []string{ar.article.AuthorID})
		if err != nil {
			return nil, err
		}
		u = users[ar.article.AuthorID]
		if u == nil {
			return nil, fmt.Errorf("author not found: %s", ar.article.AuthorID)
		}
	}
	return &UserResolver{user: *u, r: ar.r}, nil
}

func (ar *ArticleResolver) Board(ctx context.Context) (*BoardResolver, error) {
	if ar.board != nil {
		return &BoardResolver{board: *ar.board, r: ar.r}, nil
	}
	board, err := ar.r.fetchBoardByID(ctx, ar.article.BoardID)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, fmt.Errorf("board not found: %s", ar.article.BoardID)
	}
	return &BoardResolver{board: *board, r: ar.r}, nil
}

// BoardResolver resolves the Board GraphQL type.
type BoardResolver struct {
	board client.ContentBoard
	owner *client.User // nil = lazy load on Owner()
	r     *Resolver
}

func (br *BoardResolver) ID() graphql.ID         { return graphql.ID(br.board.ID) }
func (br *BoardResolver) Name() string           { return br.board.Name }
func (br *BoardResolver) Description() *string   { return br.board.Description }
func (br *BoardResolver) DefaultAccess() string  { return br.board.DefaultAccess }
func (br *BoardResolver) SubscriberCount() int32 { return br.board.SubscriberCount }
func (br *BoardResolver) IsSubscribed() bool     { return br.board.IsSubscribed }
func (br *BoardResolver) CreatedAt() string      { return br.board.CreatedAt }

func (br *BoardResolver) WritePolicy() *BoardWritePolicyResolver {
	return &BoardWritePolicyResolver{board: br.board}
}

// BoardWritePolicyResolver exposes the trust + VC gating policy for a board.
type BoardWritePolicyResolver struct {
	board client.ContentBoard
}

func (r *BoardWritePolicyResolver) MinTrustLevel() int32 { return r.board.MinTrustLevel }
func (r *BoardWritePolicyResolver) MinCommentTrust() int32 { return r.board.MinCommentTrust }

func (r *BoardWritePolicyResolver) RequireVcs() []*VcRequirementResolver {
	out := make([]*VcRequirementResolver, len(r.board.RequireVcs))
	for i, v := range r.board.RequireVcs {
		v := v
		out[i] = &VcRequirementResolver{VcTypeVal: v.VcType, IssuerVal: v.Issuer}
	}
	return out
}

func (r *BoardWritePolicyResolver) RequireCommentVcs() []*VcRequirementResolver {
	out := make([]*VcRequirementResolver, len(r.board.RequireCommentVcs))
	for i, v := range r.board.RequireCommentVcs {
		v := v
		out[i] = &VcRequirementResolver{VcTypeVal: v.VcType, IssuerVal: v.Issuer}
	}
	return out
}

type VcRequirementResolver struct {
	VcTypeVal string
	IssuerVal string
}

func (r *VcRequirementResolver) VcType() string { return r.VcTypeVal }
func (r *VcRequirementResolver) Issuer() string { return r.IssuerVal }

func (br *BoardResolver) Owner(ctx context.Context) (*UserResolver, error) {
	if br.owner != nil {
		return &UserResolver{user: *br.owner, r: br.r}, nil
	}
	users, err := br.r.auth.GetUsersByIDs(ctx, []string{br.board.OwnerID})
	if err != nil {
		return nil, err
	}
	u := users[br.board.OwnerID]
	if u == nil {
		return nil, fmt.Errorf("owner not found: %s", br.board.OwnerID)
	}
	return &UserResolver{user: *u, r: br.r}, nil
}

// PostConnectionResolver resolves the PostConnection GraphQL type.
type PostConnectionResolver struct {
	conn  client.ContentPostConnection
	users map[string]*client.User
	r     *Resolver
}

func (pcr *PostConnectionResolver) Items() []*PostResolver {
	res := make([]*PostResolver, len(pcr.conn.Items))
	for i := range pcr.conn.Items {
		res[i] = &PostResolver{post: pcr.conn.Items[i], users: pcr.users, r: pcr.r}
	}
	return res
}
func (pcr *PostConnectionResolver) NextCursor() *string { return pcr.conn.NextCursor }
func (pcr *PostConnectionResolver) HasMore() bool       { return pcr.conn.HasMore }

// NoteConnectionResolver resolves the NoteConnection GraphQL type.
type NoteConnectionResolver struct {
	conn  client.ContentNoteConnection
	users map[string]*client.User
	r     *Resolver
}

func (ncr *NoteConnectionResolver) Items() []*PostResolver {
	res := make([]*PostResolver, len(ncr.conn.Items))
	for i := range ncr.conn.Items {
		res[i] = &PostResolver{post: ncr.conn.Items[i], users: ncr.users, r: ncr.r}
	}
	return res
}
func (ncr *NoteConnectionResolver) NextCursor() *string { return ncr.conn.NextCursor }
func (ncr *NoteConnectionResolver) HasMore() bool       { return ncr.conn.HasMore }

// ArticleConnectionResolver resolves the ArticleConnection GraphQL type.
type ArticleConnectionResolver struct {
	conn  client.ContentArticleConnection
	users map[string]*client.User
	board *client.ContentBoard
	r     *Resolver
}

func (acr *ArticleConnectionResolver) Items() []*ArticleResolver {
	res := make([]*ArticleResolver, len(acr.conn.Items))
	for i := range acr.conn.Items {
		res[i] = &ArticleResolver{article: acr.conn.Items[i], users: acr.users, board: acr.board, r: acr.r}
	}
	return res
}
func (acr *ArticleConnectionResolver) NextCursor() *string { return acr.conn.NextCursor }
func (acr *ArticleConnectionResolver) HasMore() bool       { return acr.conn.HasMore }

type SignatureInfoResolver struct {
	info client.ContentSignatureInfo
}

func (r *SignatureInfoResolver) IsSigned() bool       { return r.info.IsSigned }
func (r *SignatureInfoResolver) IsVerified() bool     { return r.info.IsVerified }
func (r *SignatureInfoResolver) ContentHash() *string { return r.info.ContentHash }
func (r *SignatureInfoResolver) Signature() *string   { return r.info.Signature }
func (r *SignatureInfoResolver) Algorithm() *string   { return r.info.Algorithm }
func (r *SignatureInfoResolver) Explanation() string  { return r.info.Explanation }

type ReactionCountResolver struct {
	item client.ContentReactionCount
}

func (r *ReactionCountResolver) Emotion() string { return r.item.Emotion }
func (r *ReactionCountResolver) Count() int32    { return r.item.Count }

type ArticleCommentResolver struct {
	comment client.ContentArticleComment
	users   map[string]*client.User
	r       *Resolver
}

func (r *ArticleCommentResolver) ID() graphql.ID        { return graphql.ID(r.comment.ID) }
func (r *ArticleCommentResolver) ArticleId() graphql.ID { return graphql.ID(r.comment.ArticleID) }
func (r *ArticleCommentResolver) ParentId() *graphql.ID {
	if r.comment.ParentID == nil {
		return nil
	}
	id := graphql.ID(*r.comment.ParentID)
	return &id
}
func (r *ArticleCommentResolver) Content() string   { return r.comment.Content }
func (r *ArticleCommentResolver) CreatedAt() string { return r.comment.CreatedAt }
func (r *ArticleCommentResolver) Author(ctx context.Context) (*UserResolver, error) {
	u, ok := r.users[r.comment.AuthorID]
	if !ok {
		users, err := r.r.resolveUsers(ctx, []string{r.comment.AuthorID})
		if err != nil {
			return nil, err
		}
		u = users[r.comment.AuthorID]
		if u == nil {
			return nil, fmt.Errorf("author not found: %s", r.comment.AuthorID)
		}
	}
	return &UserResolver{user: *u, r: r.r}, nil
}

// AuthPayloadResolver resolves the AuthPayload GraphQL type.
type AuthPayloadResolver struct {
	payload client.AuthGQLPayload
	r       *Resolver
}

func (apr *AuthPayloadResolver) AccessToken() string  { return apr.payload.AccessToken }
func (apr *AuthPayloadResolver) RefreshToken() string { return apr.payload.RefreshToken }
func (apr *AuthPayloadResolver) User() *UserResolver {
	return &UserResolver{user: apr.payload.User, r: apr.r}
}

type passkeyLoginOptions struct {
	Challenge          string   `json:"challenge"`
	ChallengeToken     string   `json:"challengeToken"`
	RpID               string   `json:"rpId"`
	TimeoutMs          int32    `json:"timeoutMs"`
	AllowCredentialIDs []string `json:"allowCredentialIds"`
}

type PasskeyLoginOptionsResolver struct {
	options passkeyLoginOptions
}

func (r *PasskeyLoginOptionsResolver) Challenge() string      { return r.options.Challenge }
func (r *PasskeyLoginOptionsResolver) ChallengeToken() string { return r.options.ChallengeToken }
func (r *PasskeyLoginOptionsResolver) RpId() string           { return r.options.RpID }
func (r *PasskeyLoginOptionsResolver) TimeoutMs() int32       { return r.options.TimeoutMs }
func (r *PasskeyLoginOptionsResolver) AllowCredentialIds() []string {
	return r.options.AllowCredentialIDs
}

type FollowStatsResolver struct {
	stats client.FollowStats
}

func (fsr *FollowStatsResolver) FollowerCount() int32  { return fsr.stats.FollowerCount }
func (fsr *FollowStatsResolver) FollowingCount() int32 { return fsr.stats.FollowingCount }
func (fsr *FollowStatsResolver) IsFollowing() bool     { return fsr.stats.IsFollowing }

// FeedConnectionResolver resolves the FeedConnection GraphQL type.
type FeedConnectionResolver struct {
	items      []*FeedItemResolver
	nextCursor *string
	hasMore    bool
}

func (fcr *FeedConnectionResolver) Items() []*FeedItemResolver { return fcr.items }
func (fcr *FeedConnectionResolver) NextCursor() *string        { return fcr.nextCursor }
func (fcr *FeedConnectionResolver) HasMore() bool              { return fcr.hasMore }

// FeedItemResolver resolves the FeedItem GraphQL type.
type FeedItemResolver struct {
	id      string
	typ     string
	score   float64
	post    *PostResolver
	article *ArticleResolver
}

func (fir *FeedItemResolver) ID() graphql.ID            { return graphql.ID(fir.id) }
func (fir *FeedItemResolver) Type() string              { return fir.typ }
func (fir *FeedItemResolver) Post() *PostResolver       { return fir.post }
func (fir *FeedItemResolver) Article() *ArticleResolver { return fir.article }
func (fir *FeedItemResolver) Score() float64            { return fir.score }

// ─── VC type registry ─────────────────────────────────────────────────────────

func (r *Resolver) AvailableVcTypes(ctx context.Context) ([]*VcTypeInfoResolver, error) {
	data, err := r.authGQL(ctx, `{ availableVcTypes { vcType issuer label description createdByUsername } }`, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		AvailableVcTypes []struct {
			VcType            string  `json:"vcType"`
			Issuer            string  `json:"issuer"`
			Label             string  `json:"label"`
			Description       *string `json:"description"`
			CreatedByUsername *string `json:"createdByUsername"`
		} `json:"availableVcTypes"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]*VcTypeInfoResolver, len(resp.AvailableVcTypes))
	for i, e := range resp.AvailableVcTypes {
		out[i] = &VcTypeInfoResolver{
			vcType:            e.VcType,
			issuer:            e.Issuer,
			label:             e.Label,
			description:       e.Description,
			createdByUsername: e.CreatedByUsername,
		}
	}
	return out, nil
}

type GatewayRegisterVcTypeInput struct {
	VcType      string
	Label       string
	Description *string
}

func (r *Resolver) RegisterVcType(ctx context.Context, args struct{ Input GatewayRegisterVcTypeInput }) (*VcTypeInfoResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($input: RegisterVcTypeInput!) { registerVcType(input: $input) { vcType issuer label description createdByUsername } }`,
		map[string]any{"input": map[string]any{
			"vcType":      args.Input.VcType,
			"label":       args.Input.Label,
			"description": args.Input.Description,
		}},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RegisterVcType struct {
			VcType            string  `json:"vcType"`
			Issuer            string  `json:"issuer"`
			Label             string  `json:"label"`
			Description       *string `json:"description"`
			CreatedByUsername *string `json:"createdByUsername"`
		} `json:"registerVcType"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	e := resp.RegisterVcType
	return &VcTypeInfoResolver{
		vcType:            e.VcType,
		issuer:            e.Issuer,
		label:             e.Label,
		description:       e.Description,
		createdByUsername: e.CreatedByUsername,
	}, nil
}

func (r *Resolver) DisableVcType(ctx context.Context, args struct {
	VcType string
	Issuer string
}) (bool, error) {
	data, err := r.authGQL(ctx,
		`mutation($vcType: String!, $issuer: String!) { disableVcType(vcType: $vcType, issuer: $issuer) }`,
		map[string]any{"vcType": args.VcType, "issuer": args.Issuer},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		DisableVcType bool `json:"disableVcType"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.DisableVcType, nil
}

type VcTypeInfoResolver struct {
	vcType            string
	issuer            string
	label             string
	description       *string
	createdByUsername *string
}

func (v *VcTypeInfoResolver) VcType() string       { return v.vcType }
func (v *VcTypeInfoResolver) Issuer() string       { return v.issuer }
func (v *VcTypeInfoResolver) Label() string        { return v.label }
func (v *VcTypeInfoResolver) Description() *string { return v.description }
func (v *VcTypeInfoResolver) CreatedByUsername() *string { return v.createdByUsername }

// ─── Reputation resolvers ─────────────────────────────────────────────────────

// MyReputation forwards the myReputation query to the auth service.
func (r *Resolver) MyReputation(ctx context.Context) (*ReputationStatusResolver, error) {
	const query = `{
		myReputation {
			stamps { provider score maxScore verifiedAt expiresAt isValid }
			totalScore threshold isL2
		}
	}`
	data, err := r.authGQL(ctx, query, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		MyReputation GatewayReputationStatus `json:"myReputation"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode myReputation: %w", err)
	}
	return &ReputationStatusResolver{status: resp.MyReputation}, nil
}

// RequestPhoneOTP forwards to the auth service.
func (r *Resolver) RequestPhoneOTP(ctx context.Context, args struct{ Phone string }) (bool, error) {
	data, err := r.authGQL(ctx,
		`mutation($phone: String!) { requestPhoneOTP(phone: $phone) }`,
		map[string]any{"phone": args.Phone},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		RequestPhoneOTP bool `json:"requestPhoneOTP"`
	}
	json.Unmarshal(data, &resp) //nolint:errcheck
	return resp.RequestPhoneOTP, nil
}

// VerifyPhoneOTP forwards to the auth service and returns a fresh auth payload.
func (r *Resolver) VerifyPhoneOTP(ctx context.Context, args struct {
	Phone string
	Code  string
}) (*AuthPayloadResolver, error) {
	data, err := r.authGQL(ctx,
		`mutation($phone: String!, $code: String!) {
			verifyPhoneOTP(phone: $phone, code: $code) {
				accessToken refreshToken
				user { id did username displayName email trustLevel createdAt }
			}
		}`,
		map[string]any{"phone": args.Phone, "code": args.Code},
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		VerifyPhoneOTP client.AuthGQLPayload `json:"verifyPhoneOTP"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode verifyPhoneOTP: %w", err)
	}
	return &AuthPayloadResolver{payload: resp.VerifyPhoneOTP, r: r}, nil
}

// SetActivityPubEnabled forwards the AP toggle mutation to the auth service.
func (r *Resolver) SetActivityPubEnabled(ctx context.Context, args struct{ Enabled bool }) (bool, error) {
	data, err := r.authGQL(ctx,
		`mutation($enabled: Boolean!) { setActivityPubEnabled(enabled: $enabled) }`,
		map[string]any{"enabled": args.Enabled},
	)
	if err != nil {
		return false, err
	}
	var resp struct {
		SetActivityPubEnabled bool `json:"setActivityPubEnabled"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("decode setActivityPubEnabled: %w", err)
	}
	return resp.SetActivityPubEnabled, nil
}

// ─── Reputation type resolvers ────────────────────────────────────────────────

// GatewayReputationStamp is the JSON shape returned by the auth service.
type GatewayReputationStamp struct {
	Provider   string  `json:"provider"`
	Score      int32   `json:"score"`
	MaxScore   int32   `json:"maxScore"`
	VerifiedAt string  `json:"verifiedAt"`
	ExpiresAt  *string `json:"expiresAt"`
	IsValid    bool    `json:"isValid"`
}

// GatewayReputationStatus is the JSON shape returned by the auth service.
type GatewayReputationStatus struct {
	Stamps     []GatewayReputationStamp `json:"stamps"`
	TotalScore int32                    `json:"totalScore"`
	Threshold  int32                    `json:"threshold"`
	IsL2       bool                     `json:"isL2"`
}

type ReputationStatusResolver struct {
	status GatewayReputationStatus
}

func (r *ReputationStatusResolver) Stamps() []*ReputationStampResolver {
	out := make([]*ReputationStampResolver, len(r.status.Stamps))
	for i, s := range r.status.Stamps {
		s := s
		out[i] = &ReputationStampResolver{stamp: s}
	}
	return out
}
func (r *ReputationStatusResolver) TotalScore() int32 { return r.status.TotalScore }
func (r *ReputationStatusResolver) Threshold() int32  { return r.status.Threshold }
func (r *ReputationStatusResolver) IsL2() bool        { return r.status.IsL2 }

type ReputationStampResolver struct {
	stamp GatewayReputationStamp
}

func (r *ReputationStampResolver) Provider() string  { return r.stamp.Provider }
func (r *ReputationStampResolver) Score() int32      { return r.stamp.Score }
func (r *ReputationStampResolver) MaxScore() int32   { return r.stamp.MaxScore }
func (r *ReputationStampResolver) VerifiedAt() string { return r.stamp.VerifiedAt }
func (r *ReputationStampResolver) ExpiresAt() *string { return r.stamp.ExpiresAt }
func (r *ReputationStampResolver) IsValid() bool     { return r.stamp.IsValid }
