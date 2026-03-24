package service

// oauth.go implements the social OAuth reputation verification flows for
// Twitter/X, Facebook, Instagram, and LinkedIn.
//
// Flow:
//  1. Frontend calls GQL mutation startSocialVerification(provider) → auth URL string
//  2. Frontend redirects user to that URL
//  3. Provider redirects back to /oauth/{provider}/callback?code=...&state=...
//  4. Auth service validates state, exchanges code, fetches profile, records stamp
//  5. Auth service redirects to {frontendURL}/settings/reputation?verified={provider}
//     (or ?error=... on failure)

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aleth/auth/internal/db"
)

// ─── PKCE + Nonce helpers ─────────────────────────────────────────────────────

func generateNonce(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkce generates a PKCE code_verifier and the corresponding S256 code_challenge.
func pkce() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

// ─── StartSocialOAuth ─────────────────────────────────────────────────────────

// StartSocialOAuth builds the OAuth authorization URL for the given provider,
// persists a PKCE/CSRF state row, and returns the URL to redirect the user to.
func (s *AuthService) StartSocialOAuth(ctx context.Context, userID uuid.UUID, provider string) (string, error) {
	nonce, err := generateNonce(24)
	if err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	stateParams := db.UpsertOAuthStateParams{
		UserID:    userID,
		Provider:  provider,
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	var authURL string
	switch provider {
	case "twitter":
		if s.oauthCfg.TwitterClientID == "" {
			return "", fmt.Errorf("Twitter OAuth not configured")
		}
		verifier, challenge, err := pkce()
		if err != nil {
			return "", fmt.Errorf("pkce: %w", err)
		}
		stateParams.CodeVerifier = &verifier
		authURL = buildTwitterAuthURL(
			s.oauthCfg.TwitterClientID,
			s.oauthCfg.CallbackBase+"/oauth/twitter/callback",
			nonce,
			challenge,
		)
	case "facebook":
		if s.oauthCfg.FacebookClientID == "" {
			return "", fmt.Errorf("Facebook OAuth not configured")
		}
		authURL = buildFacebookAuthURL(
			s.oauthCfg.FacebookClientID,
			s.oauthCfg.CallbackBase+"/oauth/facebook/callback",
			nonce,
		)
	case "instagram":
		if s.oauthCfg.InstagramClientID == "" {
			return "", fmt.Errorf("Instagram OAuth not configured")
		}
		authURL = buildInstagramAuthURL(
			s.oauthCfg.InstagramClientID,
			s.oauthCfg.CallbackBase+"/oauth/instagram/callback",
			nonce,
		)
	case "linkedin":
		if s.oauthCfg.LinkedInClientID == "" {
			return "", fmt.Errorf("LinkedIn OAuth not configured")
		}
		authURL = buildLinkedInAuthURL(
			s.oauthCfg.LinkedInClientID,
			s.oauthCfg.CallbackBase+"/oauth/linkedin/callback",
			nonce,
		)
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	if err := s.db.UpsertOAuthState(ctx, stateParams); err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}
	return authURL, nil
}

// ─── CompleteSocialOAuth ──────────────────────────────────────────────────────

// CompleteSocialOAuth handles the OAuth provider callback.  It validates the
// state, exchanges the code, fetches the user profile, records the reputation
// stamp, and returns the frontend redirect URL (always succeeds at the URL
// level — errors are surfaced as query params so the browser can show them).
func (s *AuthService) CompleteSocialOAuth(ctx context.Context, provider, code, state string) string {
	fail := func(reason string) string {
		return s.oauthCfg.FrontendURL + "/settings/reputation?error=" + url.QueryEscape(reason)
	}

	oauthState, err := s.db.GetOAuthState(ctx, state)
	if err != nil || oauthState.Provider != provider {
		return fail("invalid_state")
	}
	// Consume the state so it cannot be replayed.
	_ = s.db.DeleteOAuthState(ctx, state)

	userID := oauthState.UserID

	switch provider {
	case "twitter":
		if err := s.completeTwitter(ctx, userID, code, oauthState.CodeVerifier); err != nil {
			return fail(err.Error())
		}
	case "facebook":
		if err := s.completeFacebook(ctx, userID, code); err != nil {
			return fail(err.Error())
		}
	case "instagram":
		if err := s.completeInstagram(ctx, userID, code); err != nil {
			return fail(err.Error())
		}
	case "linkedin":
		if err := s.completeLinkedIn(ctx, userID, code); err != nil {
			return fail(err.Error())
		}
	default:
		return fail("unsupported_provider")
	}

	return s.oauthCfg.FrontendURL + "/settings/reputation?verified=" + provider
}

// ─── Auth URL builders ────────────────────────────────────────────────────────

func buildTwitterAuthURL(clientID, redirectURI, state, challenge string) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "tweet.read users.read")
	v.Set("state", state)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
	return "https://twitter.com/i/oauth2/authorize?" + v.Encode()
}

func buildFacebookAuthURL(clientID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "public_profile")
	v.Set("state", state)
	v.Set("response_type", "code")
	return "https://www.facebook.com/v20.0/dialog/oauth?" + v.Encode()
}

func buildInstagramAuthURL(clientID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "user_profile")
	v.Set("state", state)
	v.Set("response_type", "code")
	return "https://api.instagram.com/oauth/authorize?" + v.Encode()
}

func buildLinkedInAuthURL(clientID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "openid profile email")
	v.Set("state", state)
	v.Set("response_type", "code")
	return "https://www.linkedin.com/oauth/v2/authorization?" + v.Encode()
}

// ─── Twitter ──────────────────────────────────────────────────────────────────

func (s *AuthService) completeTwitter(ctx context.Context, userID uuid.UUID, code string, codeVerifier *string) error {
	if codeVerifier == nil {
		return fmt.Errorf("missing pkce verifier")
	}
	token, err := exchangeTwitterToken(s.httpClient, s.oauthCfg.TwitterClientID, s.oauthCfg.TwitterClientSecret,
		s.oauthCfg.CallbackBase+"/oauth/twitter/callback", code, *codeVerifier)
	if err != nil {
		return fmt.Errorf("twitter token exchange: %w", err)
	}

	profile, err := fetchTwitterProfile(s.httpClient, token)
	if err != nil {
		return fmt.Errorf("twitter profile: %w", err)
	}

	score := ComputeTwitterScore(profile.accountAgeDays, profile.followerCount, profile.tweetCount)
	meta, _ := json.Marshal(map[string]any{
		"username":        profile.username,
		"follower_count":  profile.followerCount,
		"tweet_count":     profile.tweetCount,
		"account_age_days": profile.accountAgeDays,
	})

	_, err = s.RecordSocialStamp(ctx, userID, "twitter", profile.id, score, meta)
	return err
}

type twitterProfile struct {
	id             string
	username       string
	followerCount  int
	tweetCount     int
	accountAgeDays int
}

func exchangeTwitterToken(client *http.Client, clientID, clientSecret, redirectURI, code, verifier string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", redirectURI)
	body.Set("code_verifier", verifier)

	req, _ := http.NewRequest(http.MethodPost, "https://api.twitter.com/2/oauth2/token", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Twitter uses HTTP Basic auth with client_id:client_secret
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitter token error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func fetchTwitterProfile(client *http.Client, token string) (twitterProfile, error) {
	req, _ := http.NewRequest(http.MethodGet,
		"https://api.twitter.com/2/users/me?user.fields=created_at,public_metrics,username",
		nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return twitterProfile{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return twitterProfile{}, fmt.Errorf("twitter profile error %d: %s", resp.StatusCode, string(raw))
	}

	var res struct {
		Data struct {
			ID        string `json:"id"`
			Username  string `json:"username"`
			CreatedAt string `json:"created_at"`
			PublicMetrics struct {
				FollowersCount int `json:"followers_count"`
				TweetCount     int `json:"tweet_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return twitterProfile{}, err
	}

	ageDays := 0
	if t, err := time.Parse(time.RFC3339, res.Data.CreatedAt); err == nil {
		ageDays = int(time.Since(t).Hours() / 24)
	}

	return twitterProfile{
		id:             res.Data.ID,
		username:       res.Data.Username,
		followerCount:  res.Data.PublicMetrics.FollowersCount,
		tweetCount:     res.Data.PublicMetrics.TweetCount,
		accountAgeDays: ageDays,
	}, nil
}

// ─── Facebook ─────────────────────────────────────────────────────────────────

func (s *AuthService) completeFacebook(ctx context.Context, userID uuid.UUID, code string) error {
	token, err := exchangeFacebookToken(s.httpClient,
		s.oauthCfg.FacebookClientID, s.oauthCfg.FacebookClientSecret,
		s.oauthCfg.CallbackBase+"/oauth/facebook/callback", code)
	if err != nil {
		return fmt.Errorf("facebook token exchange: %w", err)
	}

	fbID, fbName, err := fetchFacebookProfile(s.httpClient, token)
	if err != nil {
		return fmt.Errorf("facebook profile: %w", err)
	}

	// Grant a conservative fixed score — friends/age require extra permissions.
	score := int16(3)
	meta, _ := json.Marshal(map[string]any{"name": fbName})

	_, err = s.RecordSocialStamp(ctx, userID, "facebook", fbID, score, meta)
	return err
}

func exchangeFacebookToken(client *http.Client, appID, appSecret, redirectURI, code string) (string, error) {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("client_secret", appSecret)
	v.Set("redirect_uri", redirectURI)
	v.Set("code", code)
	resp, err := client.Get("https://graph.facebook.com/v20.0/oauth/access_token?" + v.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facebook token error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func fetchFacebookProfile(client *http.Client, token string) (id, name string, err error) {
	v := url.Values{}
	v.Set("fields", "id,name")
	v.Set("access_token", token)
	resp, err := client.Get("https://graph.facebook.com/me?" + v.Encode())
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("facebook profile error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", "", err
	}
	return res.ID, res.Name, nil
}

// ─── Instagram ────────────────────────────────────────────────────────────────

func (s *AuthService) completeInstagram(ctx context.Context, userID uuid.UUID, code string) error {
	token, igID, err := exchangeInstagramToken(s.httpClient,
		s.oauthCfg.InstagramClientID, s.oauthCfg.InstagramClientSecret,
		s.oauthCfg.CallbackBase+"/oauth/instagram/callback", code)
	if err != nil {
		return fmt.Errorf("instagram token exchange: %w", err)
	}

	username, err := fetchInstagramUsername(s.httpClient, token)
	if err != nil {
		return fmt.Errorf("instagram profile: %w", err)
	}

	score := int16(3)
	meta, _ := json.Marshal(map[string]any{"username": username})

	_, err = s.RecordSocialStamp(ctx, userID, "instagram", igID, score, meta)
	return err
}

func exchangeInstagramToken(client *http.Client, clientID, clientSecret, redirectURI, code string) (token, igID string, err error) {
	body := url.Values{}
	body.Set("client_id", clientID)
	body.Set("client_secret", clientSecret)
	body.Set("grant_type", "authorization_code")
	body.Set("redirect_uri", redirectURI)
	body.Set("code", code)

	resp, err := client.Post("https://api.instagram.com/oauth/access_token",
		"application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("instagram token error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		AccessToken string `json:"access_token"`
		UserID      int64  `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", "", err
	}
	return res.AccessToken, fmt.Sprintf("%d", res.UserID), nil
}

func fetchInstagramUsername(client *http.Client, token string) (string, error) {
	v := url.Values{}
	v.Set("fields", "id,username")
	v.Set("access_token", token)
	resp, err := client.Get("https://graph.instagram.com/me?" + v.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("instagram profile error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.Username, nil
}

// ─── LinkedIn ─────────────────────────────────────────────────────────────────

func (s *AuthService) completeLinkedIn(ctx context.Context, userID uuid.UUID, code string) error {
	token, err := exchangeLinkedInToken(s.httpClient,
		s.oauthCfg.LinkedInClientID, s.oauthCfg.LinkedInClientSecret,
		s.oauthCfg.CallbackBase+"/oauth/linkedin/callback", code)
	if err != nil {
		return fmt.Errorf("linkedin token exchange: %w", err)
	}

	liID, liName, err := fetchLinkedInProfile(s.httpClient, token)
	if err != nil {
		return fmt.Errorf("linkedin profile: %w", err)
	}

	score := ComputeLinkedInScore()
	meta, _ := json.Marshal(map[string]any{"name": liName})

	_, err = s.RecordSocialStamp(ctx, userID, "linkedin", liID, score, meta)
	return err
}

func exchangeLinkedInToken(client *http.Client, clientID, clientSecret, redirectURI, code string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", redirectURI)
	body.Set("client_id", clientID)
	body.Set("client_secret", clientSecret)

	resp, err := client.Post("https://www.linkedin.com/oauth/v2/accessToken",
		"application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("linkedin token error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func fetchLinkedInProfile(client *http.Client, token string) (id, name string, err error) {
	// LinkedIn OpenID Connect userinfo endpoint
	req, _ := http.NewRequest(http.MethodGet, "https://api.linkedin.com/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("linkedin profile error %d: %s", resp.StatusCode, string(raw))
	}
	var res struct {
		Sub  string `json:"sub"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", "", err
	}
	return res.Sub, res.Name, nil
}
