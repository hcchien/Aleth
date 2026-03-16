package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ContentPost is a public post returned by the content service's internal API.
type ContentPost struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// ContentClient calls the content service's internal REST endpoints.
type ContentClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewContentClient creates a client targeting the given base URL
// (e.g. "http://localhost:8082").
func NewContentClient(baseURL string) *ContentClient {
	return &ContentClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPublicPosts fetches public, non-deleted posts by authorID (UUID string).
// Pass before as a time cursor for pagination; nil fetches the latest posts.
// Maximum limit is 20.
func (c *ContentClient) GetPublicPosts(ctx context.Context, authorID string, limit int, before *time.Time) ([]ContentPost, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	params := url.Values{}
	params.Set("authorID", authorID)
	params.Set("limit", strconv.Itoa(limit))
	if before != nil {
		params.Set("before", before.UTC().Format(time.RFC3339))
	}

	endpoint := c.baseURL + "/internal/posts?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("content GetPublicPosts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("content GetPublicPosts: status %d", resp.StatusCode)
	}

	var posts []ContentPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("content GetPublicPosts decode: %w", err)
	}
	return posts, nil
}
