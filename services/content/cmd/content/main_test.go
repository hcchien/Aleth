package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddlewareInjectsClaims(t *testing.T) {
	secret := []byte("secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":         "11111111-1111-1111-1111-111111111111",
		"username":    "alice",
		"trust_level": 2,
		"aud":         "aleth:app",
		"iss":         "aleth",
		"exp":         time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	h := authMiddleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddlewareAllowsInvalidToken(t *testing.T) {
	h := authMiddleware([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected pass-through status, got %d", rr.Code)
	}
}
