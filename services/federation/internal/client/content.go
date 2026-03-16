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

// ContentPage holds the minimal page data needed by the federation service.
type ContentPage struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	APEnabled bool   `json:"apEnabled"`
}

// GetPageInfo fetches page metadata from the content service internal endpoint.
// Returns nil, nil if the page is not found (404).
func (c *ContentClient) GetPageInfo(ctx context.Context, slug string) (*ContentPage, error) {
	url := c.baseURL + "/internal/pages/" + slug
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build page info request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get page info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get page info: status %d", resp.StatusCode)
	}
	var page ContentPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode page info: %w", err)
	}
	return &page, nil
}

// GetPageFeed fetches paginated posts for a page's AP outbox.
func (c *ContentClient) GetPageFeed(ctx context.Context, slug string, limit int, before *time.Time) ([]ContentPost, error) {
	endpoint := c.baseURL + "/internal/pages/" + slug + "/feed"
	if limit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", limit)
		if before != nil {
			endpoint += "&before=" + before.Format(time.RFC3339)
		}
	} else if before != nil {
		endpoint += "?before=" + before.Format(time.RFC3339)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build page feed request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get page feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get page feed: status %d", resp.StatusCode)
	}
	var posts []ContentPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("decode page feed: %w", err)
	}
	return posts, nil
}
