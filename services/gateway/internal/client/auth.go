package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// User represents a user as returned by the Auth service internal API.
type User struct {
	ID          string  `json:"id"`
	DID         string  `json:"did"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
	TrustLevel  int32   `json:"trustLevel"`
	APEnabled   bool    `json:"apEnabled"`
	CreatedAt   string  `json:"createdAt"`
}

// AuthGQLPayload represents an auth payload from Auth service GraphQL.
type AuthGQLPayload struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         User   `json:"user"`
}

type FollowStats struct {
	FollowerCount  int32 `json:"followerCount"`
	FollowingCount int32 `json:"followingCount"`
	IsFollowing    bool  `json:"isFollowing"`
}

// ValidateResponse is returned by POST /internal/validate.
type ValidateResponse struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	TrustLevel  int32  `json:"trust_level"`
	IsSuspended bool   `json:"is_suspended"`
}

// AuthClient sends requests to the Auth service.
type AuthClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAuthClient(baseURL string) *AuthClient {
	return &AuthClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// ValidateToken calls POST /internal/validate. Returns nil if the token is invalid.
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	body, _ := json.Marshal(map[string]string{"token": token})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/validate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("validate token: status %d", resp.StatusCode)
	}

	var v ValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode validate response: %w", err)
	}
	return &v, nil
}

// GetUsersByIDs fetches users by UUIDs via POST /internal/users.
// Returns a map of user_id → *User.
func (c *AuthClient) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*User, error) {
	if len(ids) == 0 {
		return map[string]*User{}, nil
	}
	body, _ := json.Marshal(map[string][]string{"ids": ids})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/users", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get users by ids: status %d", resp.StatusCode)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	result := make(map[string]*User, len(users))
	for i := range users {
		result[users[i].ID] = &users[i]
	}
	return result, nil
}

// GetUserByUsername fetches a user by username via GET /internal/user?username=...
// Returns nil if the user does not exist.
func (c *AuthClient) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	endpoint := c.baseURL + "/internal/user?username=" + url.QueryEscape(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get user by username: status %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &user, nil
}

// GraphQL sends a GraphQL request to the Auth service's /graphql endpoint.
func (c *AuthClient) GraphQL(ctx context.Context, query string, variables map[string]any, authHeader string) (json.RawMessage, error) {
	return graphqlRequest(ctx, c.httpClient, c.baseURL+"/graphql", query, variables, authHeader)
}

// UserVC is the VC record returned by the auth service.
type UserVC struct {
	ID         string  `json:"id"`
	VcType     string  `json:"vcType"`
	Issuer     string  `json:"issuer"`
	Attributes string  `json:"attributes"`
	VerifiedAt string  `json:"verifiedAt"`
	ExpiresAt  *string `json:"expiresAt"`
	RevokedAt  *string `json:"revokedAt"`
}

// GetUserVCs fetches all valid VCs for userID via the auth service GraphQL API.
// authHeader must be the Bearer token of the authenticated user.
func (c *AuthClient) GetUserVCs(ctx context.Context, authHeader string) ([]UserVC, error) {
	const query = `query {
		myVcs {
			id vcType issuer attributes verifiedAt expiresAt revokedAt
		}
	}`
	data, err := graphqlRequest(ctx, c.httpClient, c.baseURL+"/graphql", query, nil, authHeader)
	if err != nil {
		return nil, err
	}
	var resp struct {
		MyVcs []UserVC `json:"myVcs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode myVcs: %w", err)
	}
	return resp.MyVcs, nil
}
