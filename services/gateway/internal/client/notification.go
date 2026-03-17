package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GatewayNotification represents a single notification as returned by the
// notification service REST API.
type GatewayNotification struct {
	ID         string `json:"id"`
	Type       string `json:"type"`        // "reply" | "reshare" | "comment" | "page_post"
	ActorID    string `json:"actor_id"`    // UUID of the user who triggered the notification
	EntityType string `json:"entity_type"` // "post" | "comment"
	EntityID   string `json:"entity_id"`   // UUID of the post or comment
	Read       bool   `json:"read"`
	CreatedAt  string `json:"created_at"`
}

// NotificationClient sends requests to the notification service REST API.
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewNotificationClient(baseURL string) *NotificationClient {
	return &NotificationClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetCount returns the number of unread notifications for the authenticated user.
// authHeader is the full "Bearer <token>" Authorization header value.
func (c *NotificationClient) GetCount(ctx context.Context, authHeader string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/notifications/count", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("notification service error: status %d", resp.StatusCode)
	}

	var body struct {
		Unread int64 `json:"unread"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Unread, nil
}

// List returns the most recent notifications for the authenticated user.
func (c *NotificationClient) List(ctx context.Context, authHeader string, limit int) ([]GatewayNotification, error) {
	url := fmt.Sprintf("%s/notifications?limit=%d", c.baseURL, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("notification service error: status %d", resp.StatusCode)
	}

	var items []GatewayNotification
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []GatewayNotification{}
	}
	return items, nil
}

// MarkRead marks specific notifications as read. If ids is empty, marks all
// unread notifications for the user as read. Returns immediately (202 Accepted).
func (c *NotificationClient) MarkRead(ctx context.Context, authHeader string, ids []string) error {
	var body struct {
		IDs []string `json:"ids"`
	}
	body.IDs = ids

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/notifications/mark-read", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized")
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service error: status %d", resp.StatusCode)
	}
	return nil
}
