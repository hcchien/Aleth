package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// GatewayFriendReactor is a friend who reacted to a post, returned by the Feed service.
type GatewayFriendReactor struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	Emotion     string  `json:"emotion"`
}

// GatewayReactionCount is a per-emotion reaction count from the Feed service.
type GatewayReactionCount struct {
	Emotion string `json:"emotion"`
	Count   int32  `json:"count"`
}

// GatewayFeedPost is a post as returned by the Feed service.
type GatewayFeedPost struct {
	ID                string                 `json:"id"`
	AuthorID          string                 `json:"authorId"`
	AuthorUsername    string                 `json:"authorUsername"`
	AuthorDisplayName *string                `json:"authorDisplayName"`
	AuthorTrustLevel  int32                  `json:"authorTrustLevel"`
	IsSigned          bool                   `json:"isSigned"`
	Content           string                 `json:"content"`
	Kind              string                 `json:"kind"`
	NoteTitle         *string                `json:"noteTitle"`
	NoteSummary       *string                `json:"noteSummary"`
	ResharedFromID    *string                `json:"resharedFromId"`
	CommentCount      int32                  `json:"commentCount"`
	ReactionCounts    []GatewayReactionCount `json:"reactionCounts"`
	MyEmotion         *string                `json:"myEmotion"`
	CreatedAt         string                 `json:"createdAt"`
}

// GatewayFeedItem is a single item in a feed connection from the Feed service.
type GatewayFeedItem struct {
	ID    string           `json:"id"`
	Type  string           `json:"type"`
	Post  *GatewayFeedPost `json:"post"`
	Score float64          `json:"score"`
}

// GatewayFeedConnection is a paginated feed from the Feed service.
type GatewayFeedConnection struct {
	Items      []GatewayFeedItem `json:"items"`
	NextCursor *string           `json:"nextCursor"`
	HasMore    bool              `json:"hasMore"`
}

// FeedClient sends requests to the Feed service.
type FeedClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewFeedClient(baseURL string) *FeedClient {
	return &FeedClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

const feedPostFields = `
	id authorId authorUsername authorDisplayName authorTrustLevel isSigned
	content kind noteTitle noteSummary resharedFromId
	commentCount
	reactionCounts { emotion count }
	myEmotion createdAt
`

// GetFeed calls the Feed service's feed query and returns the connection.
func (c *FeedClient) GetFeed(ctx context.Context, after *string, limit *int32, authHeader string) (*GatewayFeedConnection, error) {
	return c.fetchFeed(ctx, "feed", after, limit, authHeader)
}

// GetExploreFeed calls the Feed service's exploreFeed query.
func (c *FeedClient) GetExploreFeed(ctx context.Context, after *string, limit *int32, authHeader string) (*GatewayFeedConnection, error) {
	return c.fetchFeed(ctx, "exploreFeed", after, limit, authHeader)
}

// GetFriendReactors calls the Feed service's postFriendReactors query.
func (c *FeedClient) GetFriendReactors(ctx context.Context, postID string, limit *int32, authHeader string) ([]GatewayFriendReactor, error) {
	vars := map[string]any{"postId": postID}
	if limit != nil {
		vars["limit"] = *limit
	}
	query := `query($postId: ID!, $limit: Int) {
		postFriendReactors(postId: $postId, limit: $limit) {
			id username displayName emotion
		}
	}`
	data, err := graphqlRequest(ctx, c.httpClient, c.baseURL+"/graphql", query, vars, authHeader)
	if err != nil {
		return nil, err
	}
	var resp struct {
		PostFriendReactors []GatewayFriendReactor `json:"postFriendReactors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.PostFriendReactors, nil
}

func (c *FeedClient) fetchFeed(ctx context.Context, queryName string, after *string, limit *int32, authHeader string) (*GatewayFeedConnection, error) {
	vars := map[string]any{}
	if after != nil {
		vars["after"] = *after
	}
	if limit != nil {
		vars["limit"] = *limit
	}

	query := `query($after: String, $limit: Int) {
		` + queryName + `(after: $after, limit: $limit) {
			items { id type score post {` + feedPostFields + `} }
			nextCursor hasMore
		}
	}`

	data, err := graphqlRequest(ctx, c.httpClient, c.baseURL+"/graphql", query, vars, authHeader)
	if err != nil {
		return nil, err
	}

	var resp map[string]GatewayFeedConnection
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	conn := resp[queryName]
	return &conn, nil
}
