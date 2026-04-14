package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/store"
)

func TestOAuthPostAndListFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// 1) OAuth login/bootstrap
	oauthReq := api.OAuthRequest{
		Provider: "google",
		Token:    "integration-token-1",
	}
	var authResp api.AuthSuccess
	doJSON(t, http.MethodPost, srv.URL+"/auth/oauth", oauthReq, nil, http.StatusOK, &authResp)
	if authResp.Did == "" {
		t.Fatalf("expected non-empty did")
	}

	// 2) Create post as that OAuth user
	createReq := api.CreatePostRequest{
		Body:        "integration hello",
		MediaHashes: []string{"hash-a"},
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   authResp.Did,
		Signature:   "00", // OAuth/L0 bypass for MVP
	}
	headers := map[string]string{
		"X-Mock-OAuth-Token": oauthReq.Token,
	}
	var created api.Post
	doJSON(t, http.MethodPost, srv.URL+"/posts", createReq, headers, http.StatusCreated, &created)
	if created.Id == "" {
		t.Fatalf("expected created post id")
	}
	if created.AuthorDid != authResp.Did {
		t.Fatalf("created author mismatch: %s", created.AuthorDid)
	}

	// 3) Read public feed and verify the post appears
	var feed []api.Post
	doJSON(t, http.MethodGet, srv.URL+"/posts?limit=20&offset=0", nil, headers, http.StatusOK, &feed)
	if len(feed) == 0 {
		t.Fatalf("expected at least one post in feed")
	}
	found := false
	for _, p := range feed {
		if p.Id == created.Id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created post id=%s not found in feed", created.Id)
	}
}

func doJSON(t *testing.T, method, url string, payload any, headers map[string]string, wantStatus int, out any) {
	t.Helper()

	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s expected status %d, got %d", method, url, wantStatus, resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}
