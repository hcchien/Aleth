package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aleth/auth/graph"
	"github.com/aleth/auth/internal/db"
	"github.com/aleth/auth/internal/service"
)

type fakeLookup struct {
	usersByIDs    []db.User
	usersByIDsErr error
	userByName    *db.User
	userByNameErr error
}

func (f *fakeLookup) GetUsersByIDs(context.Context, []uuid.UUID) ([]db.User, error) {
	return f.usersByIDs, f.usersByIDsErr
}
func (f *fakeLookup) GetUserByUsername(context.Context, string) (*db.User, error) {
	return f.userByName, f.userByNameErr
}

func TestAuthMiddlewareInjectsClaims(t *testing.T) {
	tokens := service.NewTokenService("secret", "refresh", time.Minute, time.Hour)
	u := db.User{ID: uuid.New(), Username: "alice", TrustLevel: 2}
	token, err := tokens.IssueAccessToken(u, int(u.TrustLevel))
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	var injected bool
	h := authMiddleware(tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, injected = graph.ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !injected {
		t.Fatalf("expected claims injection")
	}
}

func TestValidateHandler(t *testing.T) {
	tokens := service.NewTokenService("secret", "refresh", time.Minute, time.Hour)
	u := db.User{ID: uuid.New(), Username: "alice", TrustLevel: 2}
	token, _ := tokens.IssueAccessToken(u, int(u.TrustLevel))

	h := validateHandler(tokens, nil)
	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/internal/validate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestValidateHandlerRejectsBadInput(t *testing.T) {
	tokens := service.NewTokenService("secret", "refresh", time.Minute, time.Hour)
	h := validateHandler(tokens, nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/validate", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUsersHandlerAndUserByUsernameHandler(t *testing.T) {
	now := time.Now()
	user := db.User{
		ID:         uuid.New(),
		DID:        "did:aleth:x",
		Username:   "alice",
		TrustLevel: 1,
		CreatedAt:  now,
	}
	lookup := &fakeLookup{
		usersByIDs: []db.User{user},
		userByName: &user,
	}

	h1 := usersHandler(lookup)
	reqBody, _ := json.Marshal(map[string][]string{"ids": []string{user.ID.String()}})
	req := httptest.NewRequest(http.MethodPost, "/internal/users", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	h1.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	h2 := userByUsernameHandler(lookup)
	req2 := httptest.NewRequest(http.MethodGet, "/internal/user?username=alice", nil)
	rr2 := httptest.NewRecorder()
	h2.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/internal/user", nil)
	rr3 := httptest.NewRecorder()
	h2.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr3.Code)
	}

	lookup.userByName = nil
	req4 := httptest.NewRequest(http.MethodGet, "/internal/user?username=unknown", nil)
	rr4 := httptest.NewRecorder()
	h2.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr4.Code)
	}
}
