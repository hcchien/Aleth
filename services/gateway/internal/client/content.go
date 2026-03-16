package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ContentPost is a post as returned by the Content service.
type ContentPost struct {
	ID             string                 `json:"id"`
	AuthorID       string                 `json:"authorId"`
	ParentID       *string                `json:"parentId"`
	RootID         *string                `json:"rootId"`
	Kind           string                 `json:"kind"`
	Content        string                 `json:"content"`
	NoteTitle      *string                `json:"noteTitle"`
	NoteCover      *string                `json:"noteCover"`
	NoteSummary    *string                `json:"noteSummary"`
	ResharedFromID *string                `json:"resharedFromId"`
	ReplyCount     int32                  `json:"replyCount"`
	LikeCount      int32                  `json:"likeCount"`
	IsLiked        bool                   `json:"isLiked"`
	ViewerEmotion  *string                `json:"viewerEmotion"`
	ReactionCounts []ContentReactionCount `json:"reactionCounts"`
	CreatedAt      string                 `json:"createdAt"`
	SignatureInfo  ContentSignatureInfo   `json:"signatureInfo"`
}

type ContentReactionCount struct {
	Emotion string `json:"emotion"`
	Count   int32  `json:"count"`
}

type ContentArticleComment struct {
	ID        string  `json:"id"`
	ArticleID string  `json:"articleId"`
	AuthorID  string  `json:"authorId"`
	ParentID  *string `json:"parentId"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"createdAt"`
}

// ContentVcRequirement is a VC gate entry from the content service.
type ContentVcRequirement struct {
	VcType string `json:"vcType"`
	Issuer string `json:"issuer"`
}

// ContentBoard is a board as returned by the Content service.
type ContentBoard struct {
	ID                string                 `json:"id"`
	OwnerID           string                 `json:"ownerId"`
	Name              string                 `json:"name"`
	Description       *string                `json:"description"`
	DefaultAccess     string                 `json:"defaultAccess"`
	MinTrustLevel     int32                  `json:"minTrustLevel"`
	CommentPolicy     string                 `json:"commentPolicy"`
	MinCommentTrust   int32                  `json:"minCommentTrust"`
	RequireVcs        []ContentVcRequirement `json:"requireVcs"`
	RequireCommentVcs []ContentVcRequirement `json:"requireCommentVcs"`
	SubscriberCount   int32                  `json:"subscriberCount"`
	IsSubscribed      bool                   `json:"isSubscribed"`
	CreatedAt         string                 `json:"createdAt"`
}

// ContentArticle is an article as returned by the Content service.
type ContentArticle struct {
	ID            string               `json:"id"`
	BoardID       string               `json:"boardId"`
	AuthorID      string               `json:"authorId"`
	Title         string               `json:"title"`
	Slug          string               `json:"slug"`
	ContentMd     *string              `json:"contentMd"`
	Status        string               `json:"status"`
	AccessPolicy  string               `json:"accessPolicy"`
	PublishedAt   *string              `json:"publishedAt"`
	CreatedAt     string               `json:"createdAt"`
	UpdatedAt     string               `json:"updatedAt"`
	SignatureInfo ContentSignatureInfo `json:"signatureInfo"`
}

type ContentSignatureInfo struct {
	IsSigned    bool    `json:"isSigned"`
	IsVerified  bool    `json:"isVerified"`
	ContentHash *string `json:"contentHash"`
	Signature   *string `json:"signature"`
	Algorithm   *string `json:"algorithm"`
	Explanation string  `json:"explanation"`
}

// ContentPostConnection is a paginated posts result.
type ContentPostConnection struct {
	Items      []ContentPost `json:"items"`
	NextCursor *string       `json:"nextCursor"`
	HasMore    bool          `json:"hasMore"`
}

// ContentNoteConnection is a paginated notes result.
type ContentNoteConnection struct {
	Items      []ContentPost `json:"items"`
	NextCursor *string       `json:"nextCursor"`
	HasMore    bool          `json:"hasMore"`
}

// ContentArticleConnection is a paginated articles result.
type ContentArticleConnection struct {
	Items      []ContentArticle `json:"items"`
	NextCursor *string          `json:"nextCursor"`
	HasMore    bool             `json:"hasMore"`
}

// ContentFanPage is a fan page as returned by the Content service.
type ContentFanPage struct {
	ID              string  `json:"id"`
	Slug            string  `json:"slug"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	AvatarURL       *string `json:"avatarUrl"`
	CoverURL        *string `json:"coverUrl"`
	Category        string  `json:"category"`
	APEnabled       bool    `json:"apEnabled"`
	DefaultAccess   string  `json:"defaultAccess"`
	MinTrustLevel   int32   `json:"minTrustLevel"`
	CommentPolicy   string  `json:"commentPolicy"`
	MinCommentTrust int32   `json:"minCommentTrust"`
	FollowerCount   int32   `json:"followerCount"`
	IsFollowing     bool    `json:"isFollowing"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

// ContentPageMember is a page member as returned by the Content service.
type ContentPageMember struct {
	PageID   string `json:"pageId"`
	UserID   string `json:"userId"`
	Role     string `json:"role"`
	JoinedAt string `json:"joinedAt"`
}

// ContentPageMemberConnection wraps a list of page members.
type ContentPageMemberConnection struct {
	Items []ContentPageMember `json:"items"`
}

// ContentClient sends requests to the Content service.
type ContentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewContentClient(baseURL string) *ContentClient {
	return &ContentClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GraphQL sends a raw GraphQL request to the Content service's /graphql endpoint.
func (c *ContentClient) GraphQL(ctx context.Context, query string, variables map[string]any, authHeader string) (json.RawMessage, error) {
	return graphqlRequest(ctx, c.httpClient, c.baseURL+"/graphql", query, variables, authHeader)
}
