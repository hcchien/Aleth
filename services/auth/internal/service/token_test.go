package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aleth/auth/internal/db"
)

func TestIssueAndValidateAccessToken(t *testing.T) {
	svc := NewTokenService("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)
	u := db.User{
		ID:         uuid.New(),
		Username:   "alice",
		TrustLevel: 3,
	}

	token, err := svc.IssueAccessToken(u, int(u.TrustLevel))
	if err != nil {
		t.Fatalf("IssueAccessToken error: %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken error: %v", err)
	}

	if claims.Subject != u.ID.String() {
		t.Fatalf("subject mismatch: got %s want %s", claims.Subject, u.ID.String())
	}
	if claims.Username != u.Username {
		t.Fatalf("username mismatch: got %s want %s", claims.Username, u.Username)
	}
	if claims.TrustLevel != int(u.TrustLevel) {
		t.Fatalf("trust mismatch: got %d want %d", claims.TrustLevel, u.TrustLevel)
	}
}

func TestValidateAccessTokenRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenService("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)
	validator := NewTokenService("wrong-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)
	u := db.User{ID: uuid.New(), Username: "bob", TrustLevel: 1}

	token, err := issuer.IssueAccessToken(u, int(u.TrustLevel))
	if err != nil {
		t.Fatalf("IssueAccessToken error: %v", err)
	}

	if _, err := validator.ValidateAccessToken(token); err == nil {
		t.Fatalf("expected validation failure with wrong secret")
	}
}

func TestGenerateRefreshTokenAndHash(t *testing.T) {
	svc := NewTokenService("a", "b", time.Minute, time.Hour)
	plaintext, hash, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken error: %v", err)
	}
	if plaintext == "" || hash == "" {
		t.Fatalf("expected non-empty token/hash")
	}
	if got := HashToken(plaintext); got != hash {
		t.Fatalf("hash mismatch: got %s want %s", got, hash)
	}
}

func TestRefreshTokenExpiresAt(t *testing.T) {
	ttl := 3 * time.Hour
	svc := NewTokenService("a", "b", time.Minute, ttl)
	exp := svc.RefreshTokenExpiresAt()
	now := time.Now()
	if exp.Before(now.Add(ttl-2*time.Second)) || exp.After(now.Add(ttl+2*time.Second)) {
		t.Fatalf("expiry outside expected window: %v", exp)
	}
}

func TestGenerateDID(t *testing.T) {
	did := GenerateDID()
	if !strings.HasPrefix(did, "did:aleth:") {
		t.Fatalf("unexpected DID prefix: %s", did)
	}
	if len(did) <= len("did:aleth:") {
		t.Fatalf("did too short: %s", did)
	}
}
