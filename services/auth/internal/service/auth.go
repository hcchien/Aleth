package service

import (
	"context"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/aleth/auth/internal/db"
	"github.com/aleth/auth/internal/limiter"
	cbor "github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// OAuthConfig holds credentials for social OAuth reputation providers.
type OAuthConfig struct {
	TwitterClientID      string
	TwitterClientSecret  string
	FacebookClientID     string // App ID (same as FacebookApp for server-side flow)
	FacebookClientSecret string
	InstagramClientID    string
	InstagramClientSecret string
	LinkedInClientID     string
	LinkedInClientSecret string
	CallbackBase         string // Base URL of this auth service (e.g. "http://localhost:8081")
	FrontendURL          string // Next.js app URL (e.g. "http://localhost:3000")
}

// AuthService provides authentication business logic.
type AuthService struct {
	db           authStore
	tokens       *TokenService
	googleClient string
	facebookApp  string
	passkeyRPID    string
	passkeyRPOrigin string
	httpClient   *http.Client
	oauthCfg     OAuthConfig
	limiter      limiter.Limiter

	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	smtpFrom     string
}

// SetOAuthConfig wires in the social OAuth credentials.
func (s *AuthService) SetOAuthConfig(cfg OAuthConfig) {
	s.oauthCfg = cfg
}

// FrontendURL returns the configured frontend base URL (used by callback handlers).
func (s *AuthService) FrontendURL() string {
	return s.oauthCfg.FrontendURL
}

type authStore interface {
	ExistsEmail(ctx context.Context, email string) (bool, error)
	ExistsUsername(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, params db.CreateUserParams) (db.User, error)
	CreateCredential(ctx context.Context, params db.CreateCredentialParams) (db.UserCredential, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetPasswordCredential(ctx context.Context, userID uuid.UUID) (db.UserCredential, error)
	GetOAuthCredential(ctx context.Context, credType, credentialID string) (db.UserCredential, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	CreateRefreshToken(ctx context.Context, params db.CreateRefreshTokenParams) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeAllRefreshTokensForUser(ctx context.Context, userID uuid.UUID) error
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	ListCredentialIDsByUserAndType(ctx context.Context, userID uuid.UUID, credType string) ([]string, error)
	UpdateTrustLevelAtLeast(ctx context.Context, userID uuid.UUID, minLevel int16) (db.User, error)
	FollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error
	UnfollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error
	IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error)
	CountFollowers(ctx context.Context, userID uuid.UUID) (int64, error)
	CountFollowing(ctx context.Context, userID uuid.UUID) (int64, error)
	// VC operations
	GetUserVCs(ctx context.Context, userID uuid.UUID) ([]db.UserVC, error)
	UpsertUserVC(ctx context.Context, params db.UpsertUserVCParams) (db.UserVC, error)
	RevokeUserVC(ctx context.Context, vcID, userID uuid.UUID) error
	// VC type registry
	ListVcTypes(ctx context.Context) ([]db.VcTypeEntry, error)
	RegisterVcType(ctx context.Context, vcType, issuer, label string, description *string, createdBy uuid.UUID) (db.VcTypeEntry, error)
	DisableVcType(ctx context.Context, vcType, issuer string, ownerID uuid.UUID) error
	// Reputation stamps
	UpsertReputationStamp(ctx context.Context, userID uuid.UUID, provider, providerUserID string, score int16, metadata []byte, expiresAt *time.Time) (db.ReputationStamp, error)
	GetReputationStamps(ctx context.Context, userID uuid.UUID) ([]db.ReputationStamp, error)
	SumValidStampScore(ctx context.Context, userID uuid.UUID) (int, error)
	// Phone OTP
	UpsertPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string, expiresAt time.Time) error
	VerifyPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string) (bool, error)
	// ActivityPub toggle
	SetAPEnabled(ctx context.Context, userID uuid.UUID, enabled bool) (db.User, error)
	// OAuth states (social reputation flows)
	UpsertOAuthState(ctx context.Context, params db.UpsertOAuthStateParams) error
	GetOAuthState(ctx context.Context, nonce string) (db.OAuthState, error)
	DeleteOAuthState(ctx context.Context, nonce string) error
	// Password reset tokens
	CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (*db.PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, tokenHash string) error
	UpdatePasswordCredential(ctx context.Context, userID uuid.UUID, newHash []byte) error
	// OAuth disconnection
	ListCredentialTypes(ctx context.Context, userID uuid.UUID) ([]string, error)
	DeleteCredentialByType(ctx context.Context, userID uuid.UUID, credType string) error
	// Email verification
	CreateEmailVerificationToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetEmailVerificationToken(ctx context.Context, tokenHash string) (*db.EmailVerificationToken, error)
	DeleteEmailVerificationToken(ctx context.Context, tokenHash string) error
	DeleteEmailVerificationTokensByUser(ctx context.Context, userID uuid.UUID) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	// Passkey sign count
	UpdatePasskeySignCount(ctx context.Context, credentialID string, signCount uint32) error
	// Account deletion
	SoftDeleteUser(ctx context.Context, userID uuid.UUID) error
}

// SetSMTPConfig wires in the SMTP credentials for transactional email.
func (s *AuthService) SetSMTPConfig(host string, port int, user, password, from string) {
	s.smtpHost = host
	s.smtpPort = port
	s.smtpUser = user
	s.smtpPassword = password
	s.smtpFrom = from
}

func (s *AuthService) SetFacebookAppID(appID string) {
	s.facebookApp = strings.TrimSpace(appID)
}

func (s *AuthService) SetPasskeyRPID(rpID string) {
	rpID = strings.TrimSpace(rpID)
	if rpID == "" {
		rpID = "localhost"
	}
	s.passkeyRPID = rpID
}

func (s *AuthService) SetPasskeyRPOrigin(origin string) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		origin = "http://localhost:3000"
	}
	s.passkeyRPOrigin = origin
}

func NewAuthService(store authStore, tokens *TokenService, googleClientID string) *AuthService {
	return &AuthService{
		db:           store,
		tokens:       tokens,
		googleClient: googleClientID,
		passkeyRPID:  "localhost",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		limiter:      limiter.New(context.Background(), ""), // nop by default
	}
}

// SetLimiter wires in an optional Redis-backed login attempt limiter.
func (s *AuthService) SetLimiter(l limiter.Limiter) {
	s.limiter = l
}

// AuthResult holds the token pair and user returned after a successful auth operation.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         db.User
}

// Register creates a new user with email/password.
func (s *AuthService) Register(ctx context.Context, username, email, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	if username == "" || email == "" || password == "" {
		return nil, fmt.Errorf("username, email and password are required")
	}
	if !isValidEmail(email) {
		return nil, fmt.Errorf("invalid email address")
	}
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 72 {
		return nil, fmt.Errorf("password must be 72 characters or fewer")
	}

	if blocked, err := s.limiter.CheckAndIncrement(ctx, "reg:"+email, 5, time.Hour); err != nil {
		log.Warn().Err(err).Msg("registration rate limiter error")
	} else if blocked {
		return nil, fmt.Errorf("too many registration attempts for this email — try again later")
	}

	exists, err := s.db.ExistsEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	exists, err = s.db.ExistsUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("username already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.db.CreateUser(ctx, db.CreateUserParams{
		DID:      GenerateDID(),
		Username: username,
		Email:    &email,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = s.db.CreateCredential(ctx, db.CreateCredentialParams{
		UserID:         user.ID,
		Type:           "password",
		CredentialData: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("store credential: %w", err)
	}

	// Send verification email — non-fatal; user can resend later.
	if sendErr := s.sendVerificationEmail(ctx, user.ID, email); sendErr != nil {
		log.Warn().Err(sendErr).Str("userID", user.ID.String()).Msg("failed to send verification email on register")
	}

	return s.issueTokens(ctx, user, 0)
}

// Login authenticates a user with email and password.
// After 10 consecutive failures the account is locked for 15 minutes to
// prevent brute-force and credential-stuffing attacks.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check per-account lockout before any DB work.
	locked, err := s.limiter.IsLocked(ctx, email)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Msg("limiter check error — allowing login")
	} else if locked {
		return nil, fmt.Errorf("too many failed attempts — try again in 15 minutes")
	}

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Record failure even for unknown emails (prevents user enumeration
			// timing differences from revealing account existence).
			_ = s.limiter.RecordFailure(ctx, email)
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.IsSuspended {
		return nil, fmt.Errorf("account suspended")
	}

	cred, err := s.db.GetPasswordCredential(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = s.limiter.RecordFailure(ctx, email)
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("get credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(cred.CredentialData, []byte(password)); err != nil {
		_ = s.limiter.RecordFailure(ctx, email)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Successful login — clear the failure counter.
	_ = s.limiter.Reset(ctx, email)
	return s.issueTokens(ctx, user, 0)
}

// googleTokenInfo is the response shape from Google's tokeninfo endpoint.
type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Aud           string `json:"aud"`
	Error         string `json:"error"`
}

// facebookTokenInfo is the response shape from Facebook Graph /me endpoint.
type facebookTokenInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// LoginWithGoogle validates a Google ID token and upserts the user.
func (s *AuthService) LoginWithGoogle(ctx context.Context, idToken string) (*AuthResult, error) {
	info, err := s.verifyGoogleToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("verify google token: %w", err)
	}
	if info.Aud != s.googleClient {
		return nil, fmt.Errorf("token audience mismatch")
	}

	// Existing Google user
	cred, err := s.db.GetOAuthCredential(ctx, "google", info.Sub)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup oauth credential: %w", err)
	}
	if err == nil {
		user, err := s.db.GetUserByID(ctx, cred.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if user.IsSuspended {
			return nil, fmt.Errorf("account suspended")
		}
		return s.issueTokens(ctx, user, 1)
	}

	// New user via Google — auto-register
	email := strings.ToLower(info.Email)
	username, err := s.uniqueUsername(ctx, info.Name, info.Sub)
	if err != nil {
		return nil, fmt.Errorf("generate username: %w", err)
	}

	credData, _ := json.Marshal(map[string]string{"email": email, "name": info.Name})
	user, err := s.db.CreateUser(ctx, db.CreateUserParams{
		DID:      GenerateDID(),
		Username: username,
		Email:    &email,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = s.db.CreateCredential(ctx, db.CreateCredentialParams{
		UserID:         user.ID,
		Type:           "google",
		CredentialID:   &info.Sub,
		CredentialData: credData,
	})
	if err != nil {
		return nil, fmt.Errorf("store google credential: %w", err)
	}

	return s.issueTokens(ctx, user, 1)
}

// LoginWithFacebook validates a Facebook OAuth access token and upserts the user.
func (s *AuthService) LoginWithFacebook(ctx context.Context, accessToken string) (*AuthResult, error) {
	info, err := s.verifyFacebookToken(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("verify facebook token: %w", err)
	}
	if info.ID == "" {
		return nil, fmt.Errorf("facebook token missing subject")
	}

	cred, err := s.db.GetOAuthCredential(ctx, "facebook", info.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup oauth credential: %w", err)
	}
	if err == nil {
		user, err := s.db.GetUserByID(ctx, cred.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if user.IsSuspended {
			return nil, fmt.Errorf("account suspended")
		}
		return s.issueTokens(ctx, user, 1)
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	username, err := s.uniqueUsername(ctx, info.Name, info.ID)
	if err != nil {
		return nil, fmt.Errorf("generate username: %w", err)
	}

	credData, _ := json.Marshal(map[string]string{"email": email, "name": info.Name})
	user, err := s.db.CreateUser(ctx, db.CreateUserParams{
		DID:      GenerateDID(),
		Username: username,
		Email:    emailPtr,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = s.db.CreateCredential(ctx, db.CreateCredentialParams{
		UserID:         user.ID,
		Type:           "facebook",
		CredentialID:   &info.ID,
		CredentialData: credData,
	})
	if err != nil {
		return nil, fmt.Errorf("store facebook credential: %w", err)
	}

	return s.issueTokens(ctx, user, 1)
}

// RefreshToken rotates a refresh token, revoking the old one and issuing a new pair.
func (s *AuthService) RefreshToken(ctx context.Context, plaintext string) (*AuthResult, error) {
	hash := HashToken(plaintext)
	rt, err := s.db.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("invalid refresh token")
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	if rt.RevokedAt != nil {
		return nil, fmt.Errorf("refresh token revoked")
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}

	if err := s.db.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("revoke old token: %w", err)
	}

	user, err := s.db.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.IsSuspended {
		return nil, fmt.Errorf("account suspended")
	}

	return s.issueTokens(ctx, user, int(rt.SessionTrustLevel))
}

// RevokeAllTokens revokes all active refresh tokens for a user (logout from all devices).
func (s *AuthService) RevokeAllTokens(ctx context.Context, userID uuid.UUID) error {
	return s.db.RevokeAllRefreshTokensForUser(ctx, userID)
}

// GetUsersByIDs returns the users with the given IDs.
func (s *AuthService) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.User, error) {
	return s.db.GetUsersByIDs(ctx, ids)
}

// GetUserByUsername returns a user by username, or nil if not found.
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (*db.User, error) {
	user, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &user, nil
}

// GetMe returns the user record for the given user ID.
func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*db.User, error) {
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

func (s *AuthService) FollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return fmt.Errorf("cannot follow yourself")
	}
	return s.db.FollowUser(ctx, followerID, followeeID)
}

func (s *AuthService) UnfollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return nil
	}
	return s.db.UnfollowUser(ctx, followerID, followeeID)
}

func (s *AuthService) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	if followerID == followeeID {
		return false, nil
	}
	return s.db.IsFollowing(ctx, followerID, followeeID)
}

type FollowStats struct {
	FollowerCount  int64
	FollowingCount int64
	IsFollowing    bool
}

type PasskeyLoginOptions struct {
	Challenge          string
	ChallengeToken     string
	RPID               string
	TimeoutMs          int32
	AllowCredentialIDs []string
}

type PasskeyAssertion struct {
	CredentialID      string
	ChallengeToken    string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	UserHandle        *string
	Username          *string
}

func (s *AuthService) FollowStats(ctx context.Context, viewerID *uuid.UUID, userID uuid.UUID) (*FollowStats, error) {
	followerCount, err := s.db.CountFollowers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count followers: %w", err)
	}
	followingCount, err := s.db.CountFollowing(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count following: %w", err)
	}

	isFollowing := false
	if viewerID != nil {
		isFollowing, err = s.db.IsFollowing(ctx, *viewerID, userID)
		if err != nil {
			return nil, fmt.Errorf("is following: %w", err)
		}
	}

	return &FollowStats{
		FollowerCount:  followerCount,
		FollowingCount: followingCount,
		IsFollowing:    isFollowing,
	}, nil
}

// extractCOSEKey extracts the raw COSE public key bytes from either:
//  1. A CBOR-encoded WebAuthn attestation object {fmt, attStmt, authData} (sent by the browser)
//  2. A raw COSE key (already in the correct format)
//
// Returns the COSE key as base64url (no padding).
func extractCOSEKey(attestationOrCOSEBase64 string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(attestationOrCOSEBase64, "="))
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	// Try to parse as a WebAuthn attestation object first.
	var attObj struct {
		AuthData []byte `cbor:"authData"`
	}
	if err := cbor.Unmarshal(raw, &attObj); err == nil && len(attObj.AuthData) > 55 {
		// authData layout:
		//   [0:32]  RP ID hash
		//   [32]    flags
		//   [33:37] sign count
		//   [37:53] AAGUID (16 bytes)
		//   [53:55] credential ID length (big-endian uint16)
		//   [55:55+L] credential ID
		//   [55+L:] COSE public key
		credIDLen := int(binary.BigEndian.Uint16(attObj.AuthData[53:55]))
		coseStart := 55 + credIDLen
		if len(attObj.AuthData) <= coseStart {
			return "", fmt.Errorf("authData too short to contain COSE key")
		}
		coseBytes := attObj.AuthData[coseStart:]
		// Validate it parses as a real COSE key.
		if _, err := webauthncose.ParsePublicKey(coseBytes); err != nil {
			return "", fmt.Errorf("invalid COSE key in authData: %w", err)
		}
		return base64.RawURLEncoding.EncodeToString(coseBytes), nil
	}

	// Already a COSE key — validate and return as-is.
	if _, err := webauthncose.ParsePublicKey(raw); err != nil {
		return "", fmt.Errorf("not a valid COSE key or attestation object: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *AuthService) RegisterPasskey(ctx context.Context, userID uuid.UUID, credentialID, credentialPublicKey string, signCount int32) (*AuthResult, error) {
	credentialID = strings.TrimSpace(credentialID)
	credentialPublicKey = strings.TrimSpace(credentialPublicKey)
	if credentialID == "" || credentialPublicKey == "" {
		return nil, fmt.Errorf("credentialID and credentialPublicKey are required")
	}

	// The browser sends the full attestation object; extract just the COSE public key.
	coseKey, err := extractCOSEKey(credentialPublicKey)
	if err != nil {
		return nil, fmt.Errorf("extract COSE key from attestation: %w", err)
	}

	credData, err := json.Marshal(map[string]any{
		"credentialPublicKey": coseKey,
		"signCount":           signCount,
		"registeredAt":        time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("encode passkey data: %w", err)
	}

	_, err = s.db.CreateCredential(ctx, db.CreateCredentialParams{
		UserID:         userID,
		Type:           "passkey",
		CredentialID:   &credentialID,
		CredentialData: credData,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("passkey already registered")
		}
		return nil, fmt.Errorf("store passkey credential: %w", err)
	}

	user, err := s.db.UpdateTrustLevelAtLeast(ctx, userID, 1)
	if err != nil {
		return nil, fmt.Errorf("upgrade trust level: %w", err)
	}

	return s.issueTokens(ctx, user, 1)
}

func (s *AuthService) BeginPasskeyLogin(ctx context.Context, username *string) (*PasskeyLoginOptions, error) {
	challenge, err := randomBase64URL(32)
	if err != nil {
		return nil, fmt.Errorf("generate challenge: %w", err)
	}
	trimmedUsername := ""
	if username != nil {
		trimmedUsername = strings.TrimSpace(*username)
	}
	challengeToken, err := s.tokens.IssuePasskeyChallengeToken(challenge, trimmedUsername, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("issue challenge token: %w", err)
	}

	allowIDs := []string{}
	if trimmedUsername != "" {
		user, err := s.GetUserByUsername(ctx, trimmedUsername)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, fmt.Errorf("user not found")
		}
		allowIDs, err = s.db.ListCredentialIDsByUserAndType(ctx, user.ID, "passkey")
		if err != nil {
			return nil, fmt.Errorf("list passkey credentials: %w", err)
		}
	}

	return &PasskeyLoginOptions{
		Challenge:          challenge,
		ChallengeToken:     challengeToken,
		RPID:               s.passkeyRPID,
		TimeoutMs:          60000,
		AllowCredentialIDs: allowIDs,
	}, nil
}

func (s *AuthService) FinishPasskeyLogin(ctx context.Context, assertion PasskeyAssertion) (*AuthResult, error) {
	credID := strings.TrimSpace(assertion.CredentialID)
	if credID == "" || strings.TrimSpace(assertion.ChallengeToken) == "" || strings.TrimSpace(assertion.ClientDataJSON) == "" {
		return nil, fmt.Errorf("credentialID, challengeToken and clientDataJSON are required")
	}
	if strings.TrimSpace(assertion.AuthenticatorData) == "" {
		return nil, fmt.Errorf("authenticatorData is required")
	}
	if strings.TrimSpace(assertion.Signature) == "" {
		return nil, fmt.Errorf("signature is required")
	}

	// ── 1. Validate challenge token ──────────────────────────────────────────
	challengeClaims, err := s.tokens.ValidatePasskeyChallengeToken(assertion.ChallengeToken)
	if err != nil {
		return nil, fmt.Errorf("invalid challenge token: %w", err)
	}

	// ── 2. Decode and validate clientDataJSON ────────────────────────────────
	clientDataBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(assertion.ClientDataJSON), "="))
	if err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON encoding")
	}
	var clientData struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Origin    string `json:"origin"`
	}
	if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON payload")
	}
	if clientData.Type != "webauthn.get" {
		return nil, fmt.Errorf("invalid assertion type")
	}

	// Verify challenge matches the stored challenge.
	browserChallengeBytes, decErr1 := base64.RawURLEncoding.DecodeString(strings.TrimRight(clientData.Challenge, "="))
	expectedChallengeBytes, decErr2 := base64.RawURLEncoding.DecodeString(strings.TrimRight(challengeClaims.Challenge, "="))
	if decErr1 != nil || decErr2 != nil || !bytes.Equal(browserChallengeBytes, expectedChallengeBytes) {
		return nil, fmt.Errorf("challenge mismatch")
	}

	// Verify origin matches the configured RP origin.
	if clientData.Origin != s.passkeyRPOrigin {
		return nil, fmt.Errorf("origin mismatch")
	}

	// ── 3. Decode authenticatorData ──────────────────────────────────────────
	authDataBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(assertion.AuthenticatorData), "="))
	if err != nil {
		return nil, fmt.Errorf("invalid authenticatorData encoding")
	}
	if len(authDataBytes) < 37 {
		return nil, fmt.Errorf("authenticatorData too short")
	}

	// ── 4. Verify RP ID hash (first 32 bytes) ───────────────────────────────
	rpIDHash := authDataBytes[:32]
	expectedRPIDHash := sha256.Sum256([]byte(s.passkeyRPID))
	if !bytes.Equal(rpIDHash, expectedRPIDHash[:]) {
		return nil, fmt.Errorf("RP ID hash mismatch")
	}

	// ── 5. Verify User Present flag (bit 0 of flags byte at index 32) ───────
	flags := authDataBytes[32]
	if flags&0x01 == 0 {
		return nil, fmt.Errorf("user present flag not set")
	}

	// ── 6. Decode signature ──────────────────────────────────────────────────
	sigBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(assertion.Signature), "="))
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	// ── 7. Build verification data: authData || SHA-256(clientDataJSON) ──────
	clientDataHash := sha256.Sum256(clientDataBytes)
	verificationData := make([]byte, len(authDataBytes)+len(clientDataHash))
	copy(verificationData, authDataBytes)
	copy(verificationData[len(authDataBytes):], clientDataHash[:])

	// ── 8. Load stored credential and verify signature ──────────────────────
	cred, err := s.db.GetOAuthCredential(ctx, "passkey", credID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("passkey not registered")
		}
		return nil, fmt.Errorf("lookup passkey: %w", err)
	}

	// Extract the COSE public key from the stored credential data.
	var credData struct {
		CredentialPublicKey string `json:"credentialPublicKey"`
		SignCount          int64  `json:"signCount"`
	}
	if err := json.Unmarshal(cred.CredentialData, &credData); err != nil {
		return nil, fmt.Errorf("parse stored credential data: %w", err)
	}
	// extractCOSEKey handles both the new format (raw COSE key) and the legacy
	// format (full attestation object) that older registrations may have stored.
	coseKeyB64, err := extractCOSEKey(credData.CredentialPublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse stored public key: %w", err)
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(coseKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode stored public key: %w", err)
	}

	parsedKey, err := webauthncose.ParsePublicKey(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse stored public key: %w", err)
	}

	valid, err := webauthncose.VerifySignature(parsedKey, verificationData, sigBytes)
	if err != nil {
		return nil, fmt.Errorf("passkey signature verification failed: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("passkey signature verification failed")
	}

	// ── 9. Sign count replay protection ─────────────────────────────────────
	signCount := binary.BigEndian.Uint32(authDataBytes[33:37])
	storedSignCount := uint32(credData.SignCount)
	if signCount > 0 && storedSignCount > 0 && signCount <= storedSignCount {
		return nil, fmt.Errorf("possible passkey cloning detected")
	}
	_ = s.db.UpdatePasskeySignCount(ctx, credID, signCount)

	// ── 10. Verify credential belongs to the claimed user ───────────────────
	if challengeClaims.Username != "" {
		userByName, err := s.db.GetUserByUsername(ctx, challengeClaims.Username)
		if err != nil {
			return nil, fmt.Errorf("get challenge user: %w", err)
		}
		if userByName.ID != cred.UserID {
			return nil, fmt.Errorf("credential-user mismatch")
		}
	}

	if assertion.Username != nil && strings.TrimSpace(*assertion.Username) != "" {
		userByInput, err := s.db.GetUserByUsername(ctx, strings.TrimSpace(*assertion.Username))
		if err != nil {
			return nil, fmt.Errorf("get input user: %w", err)
		}
		if userByInput.ID != cred.UserID {
			return nil, fmt.Errorf("credential-user mismatch")
		}
	}

	user, err := s.db.GetUserByID(ctx, cred.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.IsSuspended {
		return nil, fmt.Errorf("account suspended")
	}
	return s.issueTokens(ctx, user, 1)
}

// ─── private helpers ──────────────────────────────────────────────────────────

func (s *AuthService) issueTokens(ctx context.Context, user db.User, trustLevel int) (*AuthResult, error) {
	accessToken, err := s.tokens.IssueAccessToken(user, trustLevel)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	plaintext, hash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	_, err = s.db.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:            user.ID,
		TokenHash:         hash,
		ExpiresAt:         s.tokens.RefreshTokenExpiresAt(),
		SessionTrustLevel: int16(trustLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: plaintext,
		User:         user,
	}, nil
}

func randomBase64URL(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *AuthService) verifyGoogleToken(ctx context.Context, idToken string) (*googleTokenInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://oauth2.googleapis.com/tokeninfo?id_token="+idToken, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call tokeninfo: %w", err)
	}
	defer resp.Body.Close()

	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode tokeninfo: %w", err)
	}
	if info.Error != "" {
		return nil, fmt.Errorf("google: %s", info.Error)
	}
	return &info, nil
}

func (s *AuthService) verifyFacebookToken(ctx context.Context, accessToken string) (*facebookTokenInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://graph.facebook.com/me?fields=id,name,email&access_token="+accessToken, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call facebook graph: %w", err)
	}
	defer resp.Body.Close()

	var info facebookTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode facebook me: %w", err)
	}
	if info.Error != nil && info.Error.Message != "" {
		return nil, fmt.Errorf("facebook: %s", info.Error.Message)
	}
	return &info, nil
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9_]`)

// uniqueUsername generates a non-colliding username derived from a display name.
func (s *AuthService) uniqueUsername(ctx context.Context, displayName, sub string) (string, error) {
	base := nonAlphanumRe.ReplaceAllString(
		strings.ToLower(strings.ReplaceAll(displayName, " ", "_")), "")
	if len(base) > 20 {
		base = base[:20]
	}
	if base == "" {
		base = "user"
	}

	suffix := sub
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}

	for _, candidate := range []string{base, base + "_" + suffix} {
		exists, err := s.db.ExistsUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	// Fallback: always unique since sub is globally unique
	return base + "_" + suffix, nil
}

// ─── VC service methods ───────────────────────────────────────────────────────

// GetUserVCs returns all VCs for the given user (including expired/revoked).
func (s *AuthService) GetUserVCs(ctx context.Context, userID uuid.UUID) ([]db.UserVC, error) {
	return s.db.GetUserVCs(ctx, userID)
}

// UpsertUserVC inserts or updates a VC.
func (s *AuthService) UpsertUserVC(ctx context.Context, userID uuid.UUID, vcType, issuer string, attributes []byte, expiresAt *time.Time) (db.UserVC, error) {
	return s.db.UpsertUserVC(ctx, db.UpsertUserVCParams{
		UserID:     userID,
		VcType:     vcType,
		Issuer:     issuer,
		Attributes: attributes,
		ExpiresAt:  expiresAt,
	})
}

// RevokeUserVC soft-deletes a VC, verifying ownership.
func (s *AuthService) RevokeUserVC(ctx context.Context, vcID, userID uuid.UUID) error {
	return s.db.RevokeUserVC(ctx, vcID, userID)
}

// ─── VC type registry ─────────────────────────────────────────────────────────

// ListVcTypes returns all enabled registry entries.
func (s *AuthService) ListVcTypes(ctx context.Context) ([]db.VcTypeEntry, error) {
	return s.db.ListVcTypes(ctx)
}

// RegisterVcType adds a new VC type to the registry under the caller's username as issuer.
// vcType must be snake_case; label is the human-readable name.
func (s *AuthService) RegisterVcType(ctx context.Context, vcType, issuer, label string, description *string, createdBy uuid.UUID) (db.VcTypeEntry, error) {
	if vcType == "" || issuer == "" || label == "" {
		return db.VcTypeEntry{}, fmt.Errorf("vcType, issuer, and label are required")
	}
	return s.db.RegisterVcType(ctx, vcType, issuer, label, description, createdBy)
}

// DisableVcType soft-removes a registry entry owned by ownerID.
func (s *AuthService) DisableVcType(ctx context.Context, vcType, issuer string, ownerID uuid.UUID) error {
	return s.db.DisableVcType(ctx, vcType, issuer, ownerID)
}

// ─── Reputation stamps ────────────────────────────────────────────────────────

// L2ScoreThreshold is the minimum total stamp score required to reach L2.
const L2ScoreThreshold = 10

// ProviderMaxScore defines the maximum points each stamp provider can contribute.
var ProviderMaxScore = map[string]int16{
	"phone":     5,
	"instagram": 5,
	"facebook":  5,
	"twitter":   4,
	"linkedin":  3,
}

// GetReputationStamps returns all reputation stamps for a user.
func (s *AuthService) GetReputationStamps(ctx context.Context, userID uuid.UUID) ([]db.ReputationStamp, error) {
	return s.db.GetReputationStamps(ctx, userID)
}

// EvaluateL2 sums valid stamp scores; if >= L2ScoreThreshold, promotes the user to L2.
// Returns the new total score and whether L2 was awarded.
func (s *AuthService) EvaluateL2(ctx context.Context, userID uuid.UUID) (total int, promoted bool, err error) {
	total, err = s.db.SumValidStampScore(ctx, userID)
	if err != nil {
		return 0, false, err
	}
	if total >= L2ScoreThreshold {
		if _, err := s.db.UpdateTrustLevelAtLeast(ctx, userID, 2); err != nil {
			return total, false, err
		}
		return total, true, nil
	}
	return total, false, nil
}

// ─── Phone OTP verification ───────────────────────────────────────────────────

const phoneOTPTTL = 10 * time.Minute

// RequestPhoneOTP generates a 6-digit OTP for phone verification.
// In production wire in an SMS provider (Twilio, AWS SNS, etc.).
// In dev/test the code is returned in the log and also as the function return value.
func (s *AuthService) RequestPhoneOTP(ctx context.Context, userID uuid.UUID, phone string) (devCode string, err error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", fmt.Errorf("phone number is required")
	}
	// Simple E.164-ish validation.
	if len(phone) < 8 || len(phone) > 20 {
		return "", fmt.Errorf("invalid phone number")
	}

	// Generate 6-digit code.
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}
	code := fmt.Sprintf("%06d", (int(b[0])<<16|int(b[1])<<8|int(b[2]))%1000000)

	expiresAt := time.Now().Add(phoneOTPTTL)
	if err := s.db.UpsertPhoneOTP(ctx, userID, phone, code, expiresAt); err != nil {
		return "", fmt.Errorf("store OTP: %w", err)
	}

	// TODO: In production, send via SMS provider here.
	// For dev, the code is returned so callers can log it.
	return code, nil
}

// VerifyPhoneOTP checks the OTP, and on success records the phone stamp.
func (s *AuthService) VerifyPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string) (*AuthResult, error) {
	phone = strings.TrimSpace(phone)
	ok, err := s.db.VerifyPhoneOTP(ctx, userID, phone, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid OTP code")
	}

	// Record the phone stamp (no expiry — phone numbers are stable).
	meta := []byte(`{}`)
	if _, err := s.db.UpsertReputationStamp(ctx, userID, "phone", phone, ProviderMaxScore["phone"], meta, nil); err != nil {
		return nil, fmt.Errorf("record phone stamp: %w", err)
	}

	// Re-evaluate L2.
	if _, _, err := s.EvaluateL2(ctx, userID); err != nil {
		return nil, err
	}

	// Re-issue tokens so the client gets an updated trust level immediately.
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user, int(user.TrustLevel))
}

// ─── Social OAuth stamp helpers ───────────────────────────────────────────────

// socialStampExpiry is how long OAuth-sourced stamps remain valid before refresh.
const socialStampExpiry = 30 * 24 * time.Hour

// ComputeInstagramScore scores an Instagram account from basic profile metrics.
func ComputeInstagramScore(accountAgeDays int, followerCount int, hasBio bool) int16 {
	var s int16
	switch {
	case accountAgeDays > 730:
		s += 2
	case accountAgeDays > 180:
		s += 1
	}
	switch {
	case followerCount > 500:
		s += 2
	case followerCount > 50:
		s += 1
	}
	if hasBio {
		s += 1
	}
	if s > ProviderMaxScore["instagram"] {
		s = ProviderMaxScore["instagram"]
	}
	return s
}

// ComputeFacebookScore scores a Facebook account from basic profile metrics.
func ComputeFacebookScore(accountAgeDays int, friendCount int) int16 {
	var s int16
	switch {
	case accountAgeDays > 1460: // 4 years
		s += 2
	case accountAgeDays > 365:
		s += 1
	}
	switch {
	case friendCount > 200:
		s += 2
	case friendCount > 50:
		s += 1
	}
	if s > ProviderMaxScore["facebook"] {
		s = ProviderMaxScore["facebook"]
	}
	return s
}

// ComputeTwitterScore scores a Twitter/X account from basic profile metrics.
func ComputeTwitterScore(accountAgeDays int, followerCount int, tweetCount int) int16 {
	var s int16
	switch {
	case accountAgeDays > 730:
		s += 2
	case accountAgeDays > 180:
		s += 1
	}
	if followerCount > 100 {
		s += 1
	}
	if tweetCount > 50 {
		s += 1
	}
	if s > ProviderMaxScore["twitter"] {
		s = ProviderMaxScore["twitter"]
	}
	return s
}

// ComputeLinkedInScore scores a LinkedIn account.
// LinkedIn professional identity is inherently valuable; verification earns max score.
func ComputeLinkedInScore() int16 {
	return ProviderMaxScore["linkedin"]
}

// SetAPEnabled updates whether the user's profile is discoverable via ActivityPub.
func (s *AuthService) SetAPEnabled(ctx context.Context, userID uuid.UUID, enabled bool) (db.User, error) {
	return s.db.SetAPEnabled(ctx, userID, enabled)
}

// RecordSocialStamp stores an OAuth-based stamp and re-evaluates L2.
func (s *AuthService) RecordSocialStamp(ctx context.Context, userID uuid.UUID, provider, providerUserID string, score int16, metadata []byte) (*AuthResult, error) {
	exp := time.Now().Add(socialStampExpiry)
	if _, err := s.db.UpsertReputationStamp(ctx, userID, provider, providerUserID, score, metadata, &exp); err != nil {
		return nil, fmt.Errorf("record %s stamp: %w", provider, err)
	}
	if _, _, err := s.EvaluateL2(ctx, userID); err != nil {
		return nil, err
	}
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user, int(user.TrustLevel))
}

// ─── Password reset ───────────────────────────────────────────────────────────

// RequestPasswordReset initiates a password reset flow by sending a reset link
// to the user's email address.  If the email is not found, the call returns nil
// to avoid leaking whether an account exists.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	if blocked, err := s.limiter.CheckAndIncrement(ctx, "pwreset:"+email, 5, time.Hour); err != nil {
		log.Warn().Err(err).Msg("password reset rate limiter error")
	} else if blocked {
		return fmt.Errorf("too many password reset attempts — try again later")
	}

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No user found — return silently to avoid user enumeration.
			return nil
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	// Generate a 32-byte crypto-random token.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}
	rawToken := hex.EncodeToString(raw)

	// Hash with SHA-256 before storing.
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	expiresAt := time.Now().Add(1 * time.Hour)
	if err := s.db.CreatePasswordResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	frontendURL := s.oauthCfg.FrontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	resetLink := frontendURL + "/reset-password?token=" + rawToken

	subject := "Reset your password"
	body := "Hello,\n\nClick the link below to reset your password (valid for 1 hour):\n\n" +
		resetLink + "\n\nIf you did not request a password reset, you can ignore this email.\n"

	if err := s.sendEmail(email, subject, body); err != nil {
		log.Warn().Err(err).Str("email", email).Msg("failed to send password reset email")
	}

	return nil
}

// ResetPassword validates the reset token and updates the user's password.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Hash the incoming token to look it up.
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	prt, err := s.db.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	if time.Now().After(prt.ExpiresAt) {
		_ = s.db.DeletePasswordResetToken(ctx, tokenHash)
		return fmt.Errorf("reset token has expired")
	}

	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.UpdatePasswordCredential(ctx, prt.UserID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_ = s.db.DeletePasswordResetToken(ctx, tokenHash)

	return nil
}

// ListCredentialTypes returns the distinct credential type strings for a user.
func (s *AuthService) ListCredentialTypes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return s.db.ListCredentialTypes(ctx, userID)
}

// DisconnectOAuth removes an OAuth credential type (e.g. "google", "facebook") for the
// authenticated user.  Returns an error if the user would have no remaining login method.
func (s *AuthService) DisconnectOAuth(ctx context.Context, userID uuid.UUID, provider string) error {
	if provider != "google" && provider != "facebook" {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	types, err := s.db.ListCredentialTypes(ctx, userID)
	if err != nil {
		return fmt.Errorf("list credential types: %w", err)
	}

	// Check user has at least one other credential type after removal.
	remaining := 0
	for _, t := range types {
		if t != provider {
			remaining++
		}
	}
	if remaining == 0 {
		return fmt.Errorf("cannot disconnect last login method")
	}

	return s.db.DeleteCredentialByType(ctx, userID, provider)
}

// ─── Email verification ───────────────────────────────────────────────────────

const emailVerifyTTL = 24 * time.Hour

// sendVerificationEmail generates a verification token and emails the link.
// It is a private helper — callers must already know the user's email.
func (s *AuthService) sendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	// Delete any existing tokens for this user before creating a new one.
	_ = s.db.DeleteEmailVerificationTokensByUser(ctx, userID)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate email verify token: %w", err)
	}
	rawToken := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	expiresAt := time.Now().Add(emailVerifyTTL)
	if err := s.db.CreateEmailVerificationToken(ctx, userID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("store email verify token: %w", err)
	}

	frontendURL := s.oauthCfg.FrontendURL
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	verifyLink := frontendURL + "/verify-email?token=" + rawToken

	subject := "Verify your email address"
	body := "Hello,\n\nPlease verify your email address by clicking the link below (valid for 24 hours):\n\n" +
		verifyLink + "\n\nIf you did not create an account on Aleth, you can safely ignore this email.\n"

	return s.sendEmail(email, subject, body)
}

// VerifyEmail validates a verification token and marks the user's email as verified.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	evt, err := s.db.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("invalid or expired verification token")
	}

	if time.Now().After(evt.ExpiresAt) {
		_ = s.db.DeleteEmailVerificationToken(ctx, tokenHash)
		return fmt.Errorf("verification token has expired")
	}

	if err := s.db.MarkEmailVerified(ctx, evt.UserID); err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}

	_ = s.db.DeleteEmailVerificationToken(ctx, tokenHash)
	return nil
}

// ResendVerificationEmail looks up the user by ID, then re-sends the verification email.
func (s *AuthService) ResendVerificationEmail(ctx context.Context, userID uuid.UUID) error {
	if blocked, err := s.limiter.CheckAndIncrement(ctx, "verify:"+userID.String(), 3, time.Hour); err != nil {
		log.Warn().Err(err).Msg("email verify rate limiter error")
	} else if blocked {
		return fmt.Errorf("too many resend attempts — please wait before requesting another verification email")
	}

	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user.EmailVerified {
		return fmt.Errorf("email already verified")
	}
	if user.Email == nil {
		return fmt.Errorf("no email address on file")
	}
	return s.sendVerificationEmail(ctx, userID, *user.Email)
}

// sendEmail sends a plaintext email via SMTP.  If SMTPHost is empty, the call
// is a no-op (dev mode — a warning is logged instead).
// DeleteAccount permanently removes a user's PII and revokes all sessions.
// Posts and other content authored by this user remain in the database but are
// orphaned (the author record is anonymised). The user is identified by the
// caller's JWT — they must re-authenticate if they want to undo this.
func (s *AuthService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	// Revoke all active sessions first so concurrent requests stop working immediately.
	if err := s.db.RevokeAllRefreshTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	// Clear the login-failure counter (user is gone, no need to keep it).
	_ = s.limiter.Reset(ctx, "")
	// Soft-delete and anonymise the user record.
	if err := s.db.SoftDeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

func (s *AuthService) sendEmail(to, subject, body string) error {
	if s.smtpHost == "" {
		log.Info().Str("to", to).Str("subject", subject).Msg("SMTP not configured — skipping email send")
		return nil
	}

	from := s.smtpFrom
	if from == "" {
		from = "noreply@aleth.social"
	}

	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"\r\n" +
			body,
	)

	addr := fmt.Sprintf("%s:%d", s.smtpHost, s.smtpPort)

	var auth smtp.Auth
	if s.smtpUser != "" {
		auth = smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	return nil
}

// ─── Input validation helpers ─────────────────────────────────────────────────

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func isValidEmail(email string) bool {
	return len(email) <= 254 && emailRegex.MatchString(email)
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

var reservedUsernames = map[string]bool{
	"admin": true, "api": true, "graphql": true, "healthz": true,
	"support": true, "help": true, "about": true, "terms": true,
	"privacy": true, "security": true, "login": true, "logout": true,
	"register": true, "signup": true, "signin": true, "me": true,
	"settings": true, "notifications": true, "explore": true, "feed": true,
	"moderator": true, "mod": true, "staff": true, "team": true,
	"aleth": true, "system": true, "null": true, "undefined": true,
}

func validateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username must be 3–30 characters and contain only letters, numbers, and underscores")
	}
	if reservedUsernames[strings.ToLower(username)] {
		return fmt.Errorf("that username is reserved")
	}
	return nil
}
