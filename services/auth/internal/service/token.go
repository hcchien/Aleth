package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/aleth/auth/internal/db"
)

// Claims is the JWT payload for access tokens.
type Claims struct {
	jwt.RegisteredClaims
	Username   string `json:"username"`
	TrustLevel int    `json:"trust_level"`
}

const (
	audienceApp              = "aleth:app"
	issuerAleth              = "aleth"
	audiencePasskeyChallenge = "aleth:passkey-challenge"
)

type TokenService struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type PasskeyChallengeClaims struct {
	jwt.RegisteredClaims
	Challenge string `json:"challenge"`
	Username  string `json:"username,omitempty"`
}

func NewTokenService(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// IssueAccessToken creates a signed JWT for the given user.
// trustLevel overrides the user's stored trust level and reflects the current session's auth method.
func (t *TokenService) IssueAccessToken(user db.User, trustLevel int) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    issuerAleth,
			Audience:  jwt.ClaimStrings{audienceApp},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTTL)),
		},
		Username:   user.Username,
		TrustLevel: trustLevel,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.accessSecret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// ValidateAccessToken parses and validates a JWT, returning the claims.
func (t *TokenService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.accessSecret, nil
	}, jwt.WithAudience(audienceApp), jwt.WithIssuer(issuerAleth))
	if err != nil {
		return nil, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func (t *TokenService) IssuePasskeyChallengeToken(challenge, username string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := PasskeyChallengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerAleth,
			Audience:  jwt.ClaimStrings{audiencePasskeyChallenge},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Challenge: challenge,
		Username:  username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.accessSecret)
	if err != nil {
		return "", fmt.Errorf("sign passkey challenge token: %w", err)
	}
	return signed, nil
}

func (t *TokenService) ValidatePasskeyChallengeToken(tokenStr string) (*PasskeyChallengeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PasskeyChallengeClaims{}, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.accessSecret, nil
	}, jwt.WithAudience(audiencePasskeyChallenge), jwt.WithIssuer(issuerAleth))
	if err != nil {
		return nil, fmt.Errorf("parse passkey challenge token: %w", err)
	}
	claims, ok := token.Claims.(*PasskeyChallengeClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid challenge token claims")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token.
// Returns (plaintext token, sha256 hash for storage, error).
func (t *TokenService) GenerateRefreshToken() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	plaintext = base64.URLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(plaintext))
	hash = base64.StdEncoding.EncodeToString(h[:])
	return plaintext, hash, nil
}

// HashToken hashes a token for lookup.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(h[:])
}

// RefreshTokenExpiresAt returns the expiry time for a new refresh token.
func (t *TokenService) RefreshTokenExpiresAt() time.Time {
	return time.Now().Add(t.refreshTTL)
}

// GenerateDID creates a new DID for a user.
func GenerateDID() string {
	return "did:aleth:" + uuid.New().String()
}
