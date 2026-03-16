package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/aleth/auth/internal/db"
)

// AuthService provides authentication business logic.
type AuthService struct {
	db           authStore
	tokens       *TokenService
	googleClient string
	facebookApp  string
	passkeyRPID  string
	httpClient   *http.Client
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

func NewAuthService(store authStore, tokens *TokenService, googleClientID string) *AuthService {
	return &AuthService{
		db:           store,
		tokens:       tokens,
		googleClient: googleClientID,
		passkeyRPID:  "localhost",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
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
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
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

	return s.issueTokens(ctx, user, 0)
}

// Login authenticates a user with email and password.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("get credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(cred.CredentialData, []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

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

func (s *AuthService) RegisterPasskey(ctx context.Context, userID uuid.UUID, credentialID, credentialPublicKey string, signCount int32) (*AuthResult, error) {
	credentialID = strings.TrimSpace(credentialID)
	credentialPublicKey = strings.TrimSpace(credentialPublicKey)
	if credentialID == "" || credentialPublicKey == "" {
		return nil, fmt.Errorf("credentialID and credentialPublicKey are required")
	}

	credData, err := json.Marshal(map[string]any{
		"credentialPublicKey": credentialPublicKey,
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
	if _, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(assertion.AuthenticatorData)); err != nil && strings.TrimSpace(assertion.AuthenticatorData) != "" {
		return nil, fmt.Errorf("invalid authenticatorData encoding")
	}
	if _, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(assertion.Signature)); err != nil && strings.TrimSpace(assertion.Signature) != "" {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	challengeClaims, err := s.tokens.ValidatePasskeyChallengeToken(assertion.ChallengeToken)
	if err != nil {
		return nil, fmt.Errorf("invalid challenge token: %w", err)
	}

	clientDataBytes, err := base64.RawURLEncoding.DecodeString(assertion.ClientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON encoding")
	}
	var clientData struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
		return nil, fmt.Errorf("invalid clientDataJSON payload")
	}
	if clientData.Type != "webauthn.get" {
		return nil, fmt.Errorf("invalid assertion type")
	}
	if clientData.Challenge != challengeClaims.Challenge {
		return nil, fmt.Errorf("challenge mismatch")
	}

	cred, err := s.db.GetOAuthCredential(ctx, "passkey", credID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("passkey not registered")
		}
		return nil, fmt.Errorf("lookup passkey: %w", err)
	}

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
