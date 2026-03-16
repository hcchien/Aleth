package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/aleth/federation/internal/config"
)

func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestServeWebFinger_MissingResource(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger", nil)
	h.ServeWebFinger(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServeWebFinger_BadScheme(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=http://bad", nil)
	h.ServeWebFinger(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServeWebFinger_MalformedAcct(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:nodomain", nil)
	h.ServeWebFinger(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServeWebFinger_DomainMismatch(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:alice@other.com", nil)
	h.ServeWebFinger(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServeActor_Redirect(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/@alice", nil)
	r = withChiParam(r, "username", "alice")
	// No Accept: application/activity+json → redirect
	h.ServeActor(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "alice") {
		t.Fatalf("unexpected redirect: %v", loc)
	}
}

func TestHandleFollowMissingActor(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/@alice/inbox", nil)
	h.handleFollow(w, r, "alice", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotifyPostCreated_BadJSON(t *testing.T) {
	h := &Handler{cfg: config.Config{Domain: "example.com"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/internal/post-created", strings.NewReader("not-json"))
	h.NotifyPostCreated(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
