// Package client provides HTTP clients for upstream Aleth services.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AuthUser is the user record returned by the auth service's internal API.
type AuthUser struct {
	ID          string  `json:"id"`
	DID         string  `json:"did"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	TrustLevel  int32   `json:"trustLevel"`
	APEnabled   bool    `json:"apEnabled"`
	CreatedAt   string  `json:"createdAt"`
}

// AuthClient calls the auth service's internal REST endpoints.
type AuthClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAuthClient creates a client targeting the given base URL
// (e.g. "http://localhost:8081").
func NewAuthClient(baseURL string) *AuthClient {
	return &AuthClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetUserByUsername fetches a user by username from the auth service.
// Returns nil, nil when the user does not exist.
func (c *AuthClient) GetUserByUsername(ctx context.Context, username string) (*AuthUser, error) {
	endpoint := c.baseURL + "/internal/user?username=" + url.QueryEscape(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth GetUserByUsername: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth GetUserByUsername: status %d", resp.StatusCode)
	}

	var u AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("auth GetUserByUsername decode: %w", err)
	}
	return &u, nil
}
