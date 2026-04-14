package content

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/store"
)

func TestCreatePostSignedSuccess(t *testing.T) {
	db := store.NewMemoryStore()
	svc := NewService(db)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const did = "did:vflow:test-signer"
	if err := db.CreateUser(&store.User{
		DID:       did,
		PublicKey: pub,
		TrustTier: store.L1_DEVICE,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	req := api.CreatePostRequest{
		Body:        "hello signed world",
		MediaHashes: []string{"hash1", "hash2"},
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   did,
	}
	req.Signature = signRequest(t, req, priv)

	post, err := svc.CreatePost(req)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if post.ID == 0 {
		t.Fatalf("expected non-zero post id")
	}
	if post.AuthorDID != did {
		t.Fatalf("author did mismatch: got %s", post.AuthorDID)
	}
}

func TestCreatePostOAuthBypassSignature(t *testing.T) {
	db := store.NewMemoryStore()
	svc := NewService(db)

	const did = "oauth:google:test-token"
	if err := db.CreateUser(&store.User{
		DID:       did,
		TrustTier: store.L0_GUEST,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// oauth:* users bypass cryptographic verification in MVP.
	req := api.CreatePostRequest{
		Body:        "guest post",
		MediaHashes: []string{},
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   did,
		Signature:   "00",
	}

	if _, err := svc.CreatePost(req); err != nil {
		t.Fatalf("create oauth post: %v", err)
	}
}

func TestCreatePostInvalidParentID(t *testing.T) {
	db := store.NewMemoryStore()
	svc := NewService(db)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const did = "did:vflow:test-parent"
	if err := db.CreateUser(&store.User{DID: did, PublicKey: pub, TrustTier: store.L1_DEVICE}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	parentID := "not-a-number"
	req := api.CreatePostRequest{
		Body:        "reply",
		MediaHashes: []string{},
		ParentId:    &parentID,
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   did,
	}
	req.Signature = signRequest(t, req, priv)

	if _, err := svc.CreatePost(req); err == nil {
		t.Fatalf("expected error for invalid parentId")
	}
}

func TestCreatePostUserNotFound(t *testing.T) {
	db := store.NewMemoryStore()
	svc := NewService(db)

	req := api.CreatePostRequest{
		Body:        "no user",
		MediaHashes: []string{},
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   "did:vflow:missing",
		Signature:   "abcd",
	}

	if _, err := svc.CreatePost(req); err == nil {
		t.Fatalf("expected user not found error")
	}
}

func signRequest(t *testing.T, req api.CreatePostRequest, priv ed25519.PrivateKey) string {
	t.Helper()
	payload := struct {
		Body        string   `json:"body"`
		MediaHashes []string `json:"mediaHashes"`
		Timestamp   int64    `json:"timestamp"`
		AuthorDID   string   `json:"authorDid"`
	}{
		Body:        req.Body,
		MediaHashes: req.MediaHashes,
		Timestamp:   int64(req.Timestamp),
		AuthorDID:   req.AuthorDid,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, b)
	return hex.EncodeToString(sig)
}
