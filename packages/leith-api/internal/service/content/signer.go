package content

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/store"
)

type Service struct {
	db store.Store
}

func NewService(db store.Store) *Service {
	return &Service{db: db}
}

// VerifySignature reconstructs the payload exactly as the frontend signed it,
// and uses the user's Ed25519 public key to verify it.
func (s *Service) VerifySignature(req api.CreatePostRequest, pubKeyBytes []byte) error {
	// L0 guest users authenticated by OAuth do not have passkeys yet.
	// For MVP, we skip cryptographic checks for oauth:* identities.
	if strings.HasPrefix(req.AuthorDid, "oauth:") && len(pubKeyBytes) == 0 {
		return nil
	}

	// Reconstruct the JSON payload the frontend signed.
	// We sort keys or use a deterministic JSON structure to ensure it matches.
	// For MVP: let's assume the frontend signs a strict structure.
	type PayloadToSign struct {
		Body        string   `json:"body"`
		MediaHashes []string `json:"mediaHashes"`
		Timestamp   int64    `json:"timestamp"`
		AuthorDID   string   `json:"authorDid"`
	}

	payload := PayloadToSign{
		Body:        req.Body,
		MediaHashes: req.MediaHashes,
		Timestamp:   int64(req.Timestamp),
		AuthorDID:   req.AuthorDid,
	}

	// In production, we'd ensure deterministic JSON or use a specific hash of the struct
	msg, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("invalid hex signature format: %w", err)
	}

	pubKey := ed25519.PublicKey(pubKeyBytes)
	if !ed25519.Verify(pubKey, msg, sigBytes) {
		return fmt.Errorf("cryptographic signature verification failed")
	}

	return nil
}

// CreatePost handles the verification and storage of a new post
func (s *Service) CreatePost(req api.CreatePostRequest) (*store.Post, error) {
	// 1. Get User by DID
	user, err := s.db.GetUserByDID(req.AuthorDid)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found for DID %s", req.AuthorDid)
	}

	// 2. Verify signature
	if err := s.VerifySignature(req, user.PublicKey); err != nil {
		return nil, err
	}

	// 3. Create DB Model
	post := &store.Post{
		Body:            req.Body,
		MediaHashes:     req.MediaHashes,
		Timestamp:       int64(req.Timestamp),
		AuthorDID:       req.AuthorDid,
		Signature:       req.Signature,
		VisibilityScore: 1.0, // Base default, adjusted later by Reputation engine
		CreatedAt:       time.Now(),
	}
	if req.ParentId != nil && *req.ParentId != "" {
		parentID, err := strconv.ParseInt(*req.ParentId, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid parentId: %w", err)
		}
		post.ParentID.Valid = true
		post.ParentID.Int64 = parentID
	}

	// 4. Save to DB
	if err := s.db.CreatePost(post); err != nil {
		return nil, fmt.Errorf("failed to save post: %w", err)
	}

	// TODO: Auto-link short posts to long "Vault" posts here if parent logic is needed

	return post, nil
}
