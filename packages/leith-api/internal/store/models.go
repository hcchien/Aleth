package store

import (
	"database/sql"
	"time"
)

// TrustTier represents the L1-L4 user trust levels
type TrustTier int

const (
	L0_GUEST     TrustTier = 0 // OAuth readers (Zero voting weight)
	L1_DEVICE    TrustTier = 1 // Basic hardware validation
	L2_SOCIAL    TrustTier = 2 // Social guarantees
	L3_TEMPORAL  TrustTier = 3 // Time accumulated
	L4_AUTHORITY TrustTier = 4 // Authority credential
)

// User represents the identity and passkey information
type User struct {
	ID        int64     `db:"id" json:"id"`
	DID       string    `db:"did" json:"did"`      // Primary identity e.g., did:vflow:{pubkey} or oauth:google:{id}
	OAuthID   string    `db:"oauth_id" json:"-"`   // Only used if L0 Guest
	PublicKey []byte    `db:"public_key" json:"-"` // Ed25519 public key (empty if L0)
	TrustTier TrustTier `db:"trust_tier" json:"trustTier"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// Post represents a signed content payload
type Post struct {
	ID              int64         `db:"id" json:"id"`
	Body            string        `db:"body" json:"body"`
	MediaHashes     []string      `db:"media_hashes" json:"mediaHashes"`     // pHash array
	ParentID        sql.NullInt64 `db:"parent_id" json:"parentId,omitempty"` // For threads/vault links
	Timestamp       int64         `db:"timestamp" json:"timestamp"`          // Unix Epoch from client
	AuthorDID       string        `db:"author_did" json:"authorDid"`
	Signature       string        `db:"signature" json:"signature"` // Hex signature
	VisibilityScore float64       `db:"visibility_score" json:"visibilityScore"`
	CreatedAt       time.Time     `db:"created_at" json:"createdAt"`
}

// Store Interface to define DB interactions
type Store interface {
	CreateUser(user *User) error
	GetUserByDID(did string) (*User, error)
	CreatePost(post *Post) error
	GetPosts(limit, offset int) ([]Post, error)
	UpdateVisibilityScore(postID int64, score float64) error
	// Rate Limiting helpers
	CheckRateLimit(did string, tier TrustTier) (bool, error)
}
