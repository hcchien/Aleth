package graph

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/aleth/auth/internal/db"
	"github.com/aleth/auth/internal/service"
)

func TestContextHelpers(t *testing.T) {
	claims := &service.Claims{Username: "alice"}
	ctx := WithClaims(context.Background(), claims)
	got, ok := ClaimsFromContext(ctx)
	if !ok || got.Username != "alice" {
		t.Fatalf("claims not found in context")
	}
}

func TestDidDocument(t *testing.T) {
	r := &Resolver{}
	doc, err := r.DidDocument(context.Background(), struct{ Did string }{Did: "did:aleth:1"})
	if err != nil {
		t.Fatalf("DidDocument error: %v", err)
	}
	if doc == nil || !strings.Contains(*doc, "did:aleth:1") {
		t.Fatalf("unexpected DID document: %v", doc)
	}
}

func TestResolverAuthChecks(t *testing.T) {
	r := &Resolver{}
	me, err := r.Me(context.Background())
	if err != nil {
		t.Fatalf("Me unexpected error: %v", err)
	}
	if me != nil {
		t.Fatalf("expected nil me without claims")
	}

	ctx := WithClaims(context.Background(), &service.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "invalid"}})
	if _, err := r.Me(ctx); err == nil {
		t.Fatalf("expected invalid subject claim error")
	}

	ok, err := r.RevokeToken(context.Background())
	if err == nil || ok {
		t.Fatalf("expected not authenticated error")
	}
}

func TestUserAndAuthPayloadResolvers(t *testing.T) {
	email := "a@example.com"
	display := "Alice"
	now := time.Now()
	u := db.User{
		ID:          uuid.New(),
		DID:         "did:aleth:abc",
		Username:    "alice",
		DisplayName: &display,
		Email:       &email,
		TrustLevel:  2,
		CreatedAt:   now,
	}
	ur := &UserResolver{user: u}
	if ur.Username() != "alice" || ur.Email() == nil {
		t.Fatalf("unexpected user resolver values")
	}

	p := &AuthPayloadResolver{result: &service.AuthResult{
		AccessToken:  "at",
		RefreshToken: "rt",
		User:         u,
	}}
	if p.AccessToken() != "at" || p.RefreshToken() != "rt" || p.User().Username() != "alice" {
		t.Fatalf("unexpected auth payload resolver values")
	}
}
