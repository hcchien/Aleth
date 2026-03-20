package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RemoteFollowing represents a remote AP actor that a local user follows.
type RemoteFollowing struct {
	ActorURL  string `json:"actorURL"`
	InboxURL  string `json:"inboxURL"`
	Accepted  bool   `json:"accepted"`
	CreatedAt string `json:"createdAt"`
}

// RemotePost is an incoming federated post stored in the federation service.
type RemotePost struct {
	ID          string `json:"id"`
	ActivityID  string `json:"activityID"`
	ActorURL    string `json:"actorURL"`
	Content     string `json:"content"`
	PublishedAt string `json:"publishedAt"`
}

// RemotePostPage is a paginated list of remote posts.
type RemotePostPage struct {
	Posts   []RemotePost `json:"posts"`
	HasMore bool         `json:"hasMore"`
}

// FederationClient calls the federation service internal HTTP API.
type FederationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewFederationClient creates a client pointed at the federation service.
func NewFederationClient(baseURL string) *FederationClient {
	return &FederationClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// FollowRemoteActor asks the federation service to send a Follow activity.
func (c *FederationClient) FollowRemoteActor(ctx context.Context, localUsername, actorURL string) error {
	body, _ := json.Marshal(map[string]string{"localUsername": localUsername, "actorURL": actorURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/follow-remote", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("follow remote actor: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var msg map[string]string
		json.NewDecoder(resp.Body).Decode(&msg)
		return fmt.Errorf("federation service: %s", resp.Status)
	}
	return nil
}

// UnfollowRemoteActor asks the federation service to send an Undo(Follow) activity.
func (c *FederationClient) UnfollowRemoteActor(ctx context.Context, localUsername, actorURL string) error {
	body, _ := json.Marshal(map[string]string{"localUsername": localUsername, "actorURL": actorURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/internal/follow-remote", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unfollow remote actor: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("federation service: %s", resp.Status)
	}
	return nil
}

// ListRemoteFollowing fetches the list of remote actors followed by localUsername.
func (c *FederationClient) ListRemoteFollowing(ctx context.Context, localUsername string) ([]RemoteFollowing, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/remote-following?username="+localUsername, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list remote following: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("federation service: %s", resp.Status)
	}
	var out []RemoteFollowing
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode remote following: %w", err)
	}
	return out, nil
}

// ListRemotePosts fetches federated posts received by localUsername.
func (c *FederationClient) ListRemotePosts(ctx context.Context, localUsername string, limit int, before string) (*RemotePostPage, error) {
	url := fmt.Sprintf("%s/internal/remote-posts?username=%s&limit=%d", c.baseURL, localUsername, limit)
	if before != "" {
		url += "&before=" + before
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list remote posts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("federation service: %s", resp.Status)
	}
	var page RemotePostPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode remote posts: %w", err)
	}
	return &page, nil
}
