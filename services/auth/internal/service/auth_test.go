package service

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/aleth/auth/internal/db"
)

type fakeAuthStore struct {
	existsEmailFn                    func(context.Context, string) (bool, error)
	existsUsernameFn                 func(context.Context, string) (bool, error)
	createUserFn                     func(context.Context, db.CreateUserParams) (db.User, error)
	createCredentialFn               func(context.Context, db.CreateCredentialParams) (db.UserCredential, error)
	getUserByEmailFn                 func(context.Context, string) (db.User, error)
	getPasswordCredentialFn          func(context.Context, uuid.UUID) (db.UserCredential, error)
	getOAuthCredentialFn             func(context.Context, string, string) (db.UserCredential, error)
	getUserByIDFn                    func(context.Context, uuid.UUID) (db.User, error)
	createRefreshTokenFn             func(context.Context, db.CreateRefreshTokenParams) (db.RefreshToken, error)
	getRefreshTokenByHashFn          func(context.Context, string) (db.RefreshToken, error)
	revokeRefreshTokenFn             func(context.Context, uuid.UUID) error
	revokeAllRefreshTokensFn         func(context.Context, uuid.UUID) error
	getUsersByIDsFn                  func(context.Context, []uuid.UUID) ([]db.User, error)
	getUserByUsernameFn              func(context.Context, string) (db.User, error)
	listCredentialIDsByUserAndTypeFn func(context.Context, uuid.UUID, string) ([]string, error)
	updateTrustLevelAtLeastFn        func(context.Context, uuid.UUID, int16) (db.User, error)
	followUserFn                     func(context.Context, uuid.UUID, uuid.UUID) error
	unfollowUserFn                   func(context.Context, uuid.UUID, uuid.UUID) error
	isFollowingFn                    func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	countFollowersFn                 func(context.Context, uuid.UUID) (int64, error)
	countFollowingFn                 func(context.Context, uuid.UUID) (int64, error)
	disableVcTypeFn                  func(context.Context, string, string, uuid.UUID) error
	setAPEnabledFn                   func(context.Context, uuid.UUID, bool) (db.User, error)
	getUserVCsFn                     func(context.Context, uuid.UUID) ([]db.UserVC, error)
	upsertUserVCFn                   func(context.Context, db.UpsertUserVCParams) (db.UserVC, error)
	revokeUserVCFn                   func(context.Context, uuid.UUID, uuid.UUID) error
	listVcTypesFn                    func(context.Context) ([]db.VcTypeEntry, error)
	registerVcTypeFn                 func(context.Context, string, string, string, *string, uuid.UUID) (db.VcTypeEntry, error)
	upsertReputationStampFn          func(context.Context, uuid.UUID, string, string, int16, []byte, *time.Time) (db.ReputationStamp, error)
	getReputationStampsFn            func(context.Context, uuid.UUID) ([]db.ReputationStamp, error)
	sumValidStampScoreFn             func(context.Context, uuid.UUID) (int, error)
	upsertPhoneOTPFn                 func(context.Context, uuid.UUID, string, string, time.Time) error
	verifyPhoneOTPFn                 func(context.Context, uuid.UUID, string, string) (bool, error)
}

func (f *fakeAuthStore) ExistsEmail(ctx context.Context, email string) (bool, error) {
	return f.existsEmailFn(ctx, email)
}
func (f *fakeAuthStore) ExistsUsername(ctx context.Context, username string) (bool, error) {
	return f.existsUsernameFn(ctx, username)
}
func (f *fakeAuthStore) CreateUser(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	return f.createUserFn(ctx, params)
}
func (f *fakeAuthStore) CreateCredential(ctx context.Context, params db.CreateCredentialParams) (db.UserCredential, error) {
	return f.createCredentialFn(ctx, params)
}
func (f *fakeAuthStore) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return f.getUserByEmailFn(ctx, email)
}
func (f *fakeAuthStore) GetPasswordCredential(ctx context.Context, userID uuid.UUID) (db.UserCredential, error) {
	return f.getPasswordCredentialFn(ctx, userID)
}
func (f *fakeAuthStore) GetOAuthCredential(ctx context.Context, credType, credentialID string) (db.UserCredential, error) {
	return f.getOAuthCredentialFn(ctx, credType, credentialID)
}
func (f *fakeAuthStore) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return f.getUserByIDFn(ctx, id)
}
func (f *fakeAuthStore) CreateRefreshToken(ctx context.Context, params db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	return f.createRefreshTokenFn(ctx, params)
}
func (f *fakeAuthStore) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	return f.getRefreshTokenByHashFn(ctx, tokenHash)
}
func (f *fakeAuthStore) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	return f.revokeRefreshTokenFn(ctx, id)
}
func (f *fakeAuthStore) RevokeAllRefreshTokensForUser(ctx context.Context, userID uuid.UUID) error {
	return f.revokeAllRefreshTokensFn(ctx, userID)
}
func (f *fakeAuthStore) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.User, error) {
	return f.getUsersByIDsFn(ctx, ids)
}
func (f *fakeAuthStore) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	return f.getUserByUsernameFn(ctx, username)
}
func (f *fakeAuthStore) ListCredentialIDsByUserAndType(ctx context.Context, userID uuid.UUID, credType string) ([]string, error) {
	return f.listCredentialIDsByUserAndTypeFn(ctx, userID, credType)
}
func (f *fakeAuthStore) UpdateTrustLevelAtLeast(ctx context.Context, userID uuid.UUID, minLevel int16) (db.User, error) {
	return f.updateTrustLevelAtLeastFn(ctx, userID, minLevel)
}
func (f *fakeAuthStore) FollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	return f.followUserFn(ctx, followerID, followeeID)
}
func (f *fakeAuthStore) UnfollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	return f.unfollowUserFn(ctx, followerID, followeeID)
}
func (f *fakeAuthStore) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	return f.isFollowingFn(ctx, followerID, followeeID)
}
func (f *fakeAuthStore) CountFollowers(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.countFollowersFn(ctx, userID)
}
func (f *fakeAuthStore) CountFollowing(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.countFollowingFn(ctx, userID)
}
func (f *fakeAuthStore) DisableVcType(ctx context.Context, vcType, issuer string, ownerID uuid.UUID) error {
	return f.disableVcTypeFn(ctx, vcType, issuer, ownerID)
}
func (f *fakeAuthStore) SetAPEnabled(ctx context.Context, userID uuid.UUID, enabled bool) (db.User, error) {
	return f.setAPEnabledFn(ctx, userID, enabled)
}
func (f *fakeAuthStore) GetUserVCs(ctx context.Context, userID uuid.UUID) ([]db.UserVC, error) {
	return f.getUserVCsFn(ctx, userID)
}
func (f *fakeAuthStore) UpsertUserVC(ctx context.Context, params db.UpsertUserVCParams) (db.UserVC, error) {
	return f.upsertUserVCFn(ctx, params)
}
func (f *fakeAuthStore) RevokeUserVC(ctx context.Context, vcID, userID uuid.UUID) error {
	return f.revokeUserVCFn(ctx, vcID, userID)
}
func (f *fakeAuthStore) ListVcTypes(ctx context.Context) ([]db.VcTypeEntry, error) {
	return f.listVcTypesFn(ctx)
}
func (f *fakeAuthStore) RegisterVcType(ctx context.Context, vcType, issuer, label string, description *string, createdBy uuid.UUID) (db.VcTypeEntry, error) {
	return f.registerVcTypeFn(ctx, vcType, issuer, label, description, createdBy)
}
func (f *fakeAuthStore) UpsertReputationStamp(ctx context.Context, userID uuid.UUID, provider, providerUserID string, score int16, metadata []byte, expiresAt *time.Time) (db.ReputationStamp, error) {
	return f.upsertReputationStampFn(ctx, userID, provider, providerUserID, score, metadata, expiresAt)
}
func (f *fakeAuthStore) GetReputationStamps(ctx context.Context, userID uuid.UUID) ([]db.ReputationStamp, error) {
	return f.getReputationStampsFn(ctx, userID)
}
func (f *fakeAuthStore) SumValidStampScore(ctx context.Context, userID uuid.UUID) (int, error) {
	return f.sumValidStampScoreFn(ctx, userID)
}
func (f *fakeAuthStore) UpsertPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string, expiresAt time.Time) error {
	return f.upsertPhoneOTPFn(ctx, userID, phone, code, expiresAt)
}
func (f *fakeAuthStore) VerifyPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string) (bool, error) {
	return f.verifyPhoneOTPFn(ctx, userID, phone, code)
}
func (f *fakeAuthStore) UpsertOAuthState(_ context.Context, _ db.UpsertOAuthStateParams) error {
	return nil
}
func (f *fakeAuthStore) GetOAuthState(_ context.Context, _ string) (db.OAuthState, error) {
	return db.OAuthState{}, fmt.Errorf("not found")
}
func (f *fakeAuthStore) DeleteOAuthState(_ context.Context, _ string) error {
	return nil
}
func (f *fakeAuthStore) CreatePasswordResetToken(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}
func (f *fakeAuthStore) GetPasswordResetToken(_ context.Context, _ string) (*db.PasswordResetToken, error) {
	return nil, nil
}
func (f *fakeAuthStore) DeletePasswordResetToken(_ context.Context, _ string) error {
	return nil
}
func (f *fakeAuthStore) UpdatePasswordCredential(_ context.Context, _ uuid.UUID, _ []byte) error {
	return nil
}
func (f *fakeAuthStore) ListCredentialTypes(_ context.Context, _ uuid.UUID) ([]string, error) {
	return []string{"passkey"}, nil
}
func (f *fakeAuthStore) DeleteCredentialByType(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeAuthStore) CreateEmailVerificationToken(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}
func (f *fakeAuthStore) GetEmailVerificationToken(_ context.Context, _ string) (*db.EmailVerificationToken, error) {
	return nil, nil
}
func (f *fakeAuthStore) DeleteEmailVerificationToken(_ context.Context, _ string) error {
	return nil
}
func (f *fakeAuthStore) DeleteEmailVerificationTokensByUser(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakeAuthStore) MarkEmailVerified(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakeAuthStore) SoftDeleteUser(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakeAuthStore) UpdatePasskeySignCount(_ context.Context, _ string, _ uint32) error {
	return nil
}

func newHappyStore() *fakeAuthStore {
	return &fakeAuthStore{
		existsEmailFn:    func(context.Context, string) (bool, error) { return false, nil },
		existsUsernameFn: func(context.Context, string) (bool, error) { return false, nil },
		createUserFn: func(_ context.Context, p db.CreateUserParams) (db.User, error) {
			return db.User{ID: uuid.New(), DID: p.DID, Username: p.Username, Email: p.Email, TrustLevel: 0}, nil
		},
		createCredentialFn: func(context.Context, db.CreateCredentialParams) (db.UserCredential, error) {
			return db.UserCredential{ID: uuid.New()}, nil
		},
		getUserByEmailFn: func(context.Context, string) (db.User, error) {
			return db.User{ID: uuid.New(), Username: "alice"}, nil
		},
		getPasswordCredentialFn: func(context.Context, uuid.UUID) (db.UserCredential, error) {
			return db.UserCredential{}, pgx.ErrNoRows
		},
		getOAuthCredentialFn: func(context.Context, string, string) (db.UserCredential, error) {
			return db.UserCredential{}, pgx.ErrNoRows
		},
		getUserByIDFn: func(_ context.Context, id uuid.UUID) (db.User, error) {
			return db.User{ID: id, Username: "alice"}, nil
		},
		createRefreshTokenFn: func(_ context.Context, p db.CreateRefreshTokenParams) (db.RefreshToken, error) {
			return db.RefreshToken{ID: uuid.New(), UserID: p.UserID, TokenHash: p.TokenHash, ExpiresAt: p.ExpiresAt}, nil
		},
		getRefreshTokenByHashFn: func(context.Context, string) (db.RefreshToken, error) {
			return db.RefreshToken{}, pgx.ErrNoRows
		},
		revokeRefreshTokenFn:     func(context.Context, uuid.UUID) error { return nil },
		revokeAllRefreshTokensFn: func(context.Context, uuid.UUID) error { return nil },
		getUsersByIDsFn: func(_ context.Context, ids []uuid.UUID) ([]db.User, error) {
			out := make([]db.User, 0, len(ids))
			for _, id := range ids {
				out = append(out, db.User{ID: id, Username: "u"})
			}
			return out, nil
		},
		getUserByUsernameFn:              func(context.Context, string) (db.User, error) { return db.User{}, pgx.ErrNoRows },
		listCredentialIDsByUserAndTypeFn: func(context.Context, uuid.UUID, string) ([]string, error) { return nil, nil },
		updateTrustLevelAtLeastFn: func(_ context.Context, userID uuid.UUID, minLevel int16) (db.User, error) {
			return db.User{ID: userID, Username: "u", TrustLevel: minLevel}, nil
		},
		followUserFn:     func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		unfollowUserFn:   func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		isFollowingFn:    func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
		countFollowersFn: func(context.Context, uuid.UUID) (int64, error) { return 0, nil },
		countFollowingFn: func(context.Context, uuid.UUID) (int64, error) { return 0, nil },
		disableVcTypeFn:  func(context.Context, string, string, uuid.UUID) error { return nil },
		setAPEnabledFn: func(_ context.Context, id uuid.UUID, enabled bool) (db.User, error) {
			return db.User{ID: id, APEnabled: enabled}, nil
		},
		getUserVCsFn:            func(context.Context, uuid.UUID) ([]db.UserVC, error) { return nil, nil },
		upsertUserVCFn:          func(context.Context, db.UpsertUserVCParams) (db.UserVC, error) { return db.UserVC{}, nil },
		revokeUserVCFn:          func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		listVcTypesFn:           func(context.Context) ([]db.VcTypeEntry, error) { return nil, nil },
		registerVcTypeFn:        func(context.Context, string, string, string, *string, uuid.UUID) (db.VcTypeEntry, error) { return db.VcTypeEntry{}, nil },
		upsertReputationStampFn: func(context.Context, uuid.UUID, string, string, int16, []byte, *time.Time) (db.ReputationStamp, error) { return db.ReputationStamp{}, nil },
		getReputationStampsFn:   func(context.Context, uuid.UUID) ([]db.ReputationStamp, error) { return nil, nil },
		sumValidStampScoreFn:    func(context.Context, uuid.UUID) (int, error) { return 0, nil },
		upsertPhoneOTPFn:        func(context.Context, uuid.UUID, string, string, time.Time) error { return nil },
		verifyPhoneOTPFn:        func(context.Context, uuid.UUID, string, string) (bool, error) { return false, nil },
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	s := NewAuthService(newHappyStore(), NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	_, err := s.Register(context.Background(), "", "", "")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required error, got %v", err)
	}
	_, err = s.Register(context.Background(), "u", "a@b.com", "short")
	if err == nil || !strings.Contains(err.Error(), "at least 8") {
		t.Fatalf("expected password length error, got %v", err)
	}
}

func TestRegisterSuccess(t *testing.T) {
	s := NewAuthService(newHappyStore(), NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	res, err := s.Register(context.Background(), "alice", "Alice@Email.Com", "password123")
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected tokens")
	}
	if res.User.Username != "alice" {
		t.Fatalf("expected username alice, got %s", res.User.Username)
	}
}

func TestLoginBranches(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")

	st.getUserByEmailFn = func(context.Context, string) (db.User, error) { return db.User{}, pgx.ErrNoRows }
	if _, err := s.Login(context.Background(), "a@b.com", "password123"); err == nil {
		t.Fatalf("expected invalid credentials")
	}

	st.getUserByEmailFn = func(context.Context, string) (db.User, error) {
		return db.User{ID: uuid.New(), Username: "x", IsSuspended: true}, nil
	}
	if _, err := s.Login(context.Background(), "a@b.com", "password123"); err == nil || !strings.Contains(err.Error(), "suspended") {
		t.Fatalf("expected suspended error")
	}
}

func TestLoginSuccessAndRevoke(t *testing.T) {
	st := newHappyStore()
	password := "password123"
	hashCred, err := bcryptGenerate(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	uid := uuid.New()
	st.getUserByEmailFn = func(context.Context, string) (db.User, error) {
		return db.User{ID: uid, Username: "alice"}, nil
	}
	st.getPasswordCredentialFn = func(context.Context, uuid.UUID) (db.UserCredential, error) {
		return db.UserCredential{CredentialData: hashCred}, nil
	}
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	res, err := s.Login(context.Background(), "a@b.com", password)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected tokens")
	}
	if err := s.RevokeAllTokens(context.Background(), uid); err != nil {
		t.Fatalf("RevokeAllTokens error: %v", err)
	}
}

func TestGetUserByUsernameAndMe(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")

	u, err := s.GetUserByUsername(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetUserByUsername unexpected error: %v", err)
	}
	if u != nil {
		t.Fatalf("expected nil user")
	}

	id := uuid.New()
	st.getUserByIDFn = func(context.Context, uuid.UUID) (db.User, error) { return db.User{}, pgx.ErrNoRows }
	if _, err := s.GetMe(context.Background(), id); err == nil {
		t.Fatalf("expected not found")
	}
}

func TestRefreshTokenBranches(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")

	if _, err := s.RefreshToken(context.Background(), "unknown"); err == nil {
		t.Fatalf("expected invalid refresh token")
	}

	rtID := uuid.New()
	uID := uuid.New()
	st.getRefreshTokenByHashFn = func(context.Context, string) (db.RefreshToken, error) {
		now := time.Now()
		return db.RefreshToken{ID: rtID, UserID: uID, ExpiresAt: now.Add(-time.Minute)}, nil
	}
	if _, err := s.RefreshToken(context.Background(), "expired"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error")
	}

	now := time.Now()
	st.getRefreshTokenByHashFn = func(context.Context, string) (db.RefreshToken, error) {
		revoked := now
		return db.RefreshToken{ID: rtID, UserID: uID, ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}, nil
	}
	if _, err := s.RefreshToken(context.Background(), "revoked"); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("expected revoked error")
	}
}

func TestVerifyGoogleTokenAndUniqueUsername(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "client-id")
	s.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := `{"sub":"sub123","email":"x@example.com","name":"X","aud":"client-id"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	info, err := s.verifyGoogleToken(context.Background(), "id-token")
	if err != nil {
		t.Fatalf("verifyGoogleToken error: %v", err)
	}
	if info.Sub != "sub123" {
		t.Fatalf("unexpected sub: %s", info.Sub)
	}

	st.existsUsernameFn = func(_ context.Context, candidate string) (bool, error) {
		return candidate == "name", nil
	}
	u, err := s.uniqueUsername(context.Background(), "Name", "abcdef123456")
	if err != nil {
		t.Fatalf("uniqueUsername error: %v", err)
	}
	if u == "name" {
		t.Fatalf("expected fallback username")
	}
}

func TestLoginWithGoogleBranches(t *testing.T) {
	st := newHappyStore()
	tokens := NewTokenService("a", "b", time.Minute, time.Hour)
	s := NewAuthService(st, tokens, "client-id")

	makeInfo := func(info googleTokenInfo) {
		s.httpClient = &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				b, _ := json.Marshal(info)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(b)),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	makeInfo(googleTokenInfo{Sub: "sub", Email: "a@b.com", Name: "Alice", Aud: "wrong"})
	if _, err := s.LoginWithGoogle(context.Background(), "token"); err == nil {
		t.Fatalf("expected audience mismatch")
	}

	uID := uuid.New()
	st.getOAuthCredentialFn = func(context.Context, string, string) (db.UserCredential, error) {
		return db.UserCredential{UserID: uID}, nil
	}
	st.getUserByIDFn = func(context.Context, uuid.UUID) (db.User, error) {
		return db.User{ID: uID, Username: "alice"}, nil
	}
	makeInfo(googleTokenInfo{Sub: "sub", Email: "a@b.com", Name: "Alice", Aud: "client-id"})
	if _, err := s.LoginWithGoogle(context.Background(), "token"); err != nil {
		t.Fatalf("existing LoginWithGoogle error: %v", err)
	}

	st.getOAuthCredentialFn = func(context.Context, string, string) (db.UserCredential, error) {
		return db.UserCredential{}, pgx.ErrNoRows
	}
	if _, err := s.LoginWithGoogle(context.Background(), "token"); err != nil {
		t.Fatalf("new user LoginWithGoogle error: %v", err)
	}
}

func TestGetUsersByIDs(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	users, err := s.GetUsersByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("GetUsersByIDs error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestFollowUserSelf(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	id := uuid.New()

	if err := s.FollowUser(context.Background(), id, id); err == nil {
		t.Fatalf("expected self-follow error")
	}
}

func TestIsFollowingSelf(t *testing.T) {
	st := newHappyStore()
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	id := uuid.New()

	ok, err := s.IsFollowing(context.Background(), id, id)
	if err != nil {
		t.Fatalf("IsFollowing self unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected false for self")
	}
}

func TestFollowStats(t *testing.T) {
	st := newHappyStore()
	viewerID := uuid.New()
	targetID := uuid.New()
	st.countFollowersFn = func(context.Context, uuid.UUID) (int64, error) { return 12, nil }
	st.countFollowingFn = func(context.Context, uuid.UUID) (int64, error) { return 5, nil }
	st.isFollowingFn = func(_ context.Context, followerID, followeeID uuid.UUID) (bool, error) {
		if followerID != viewerID || followeeID != targetID {
			t.Fatalf("unexpected ids: follower=%s followee=%s", followerID, followeeID)
		}
		return true, nil
	}

	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	stats, err := s.FollowStats(context.Background(), &viewerID, targetID)
	if err != nil {
		t.Fatalf("FollowStats error: %v", err)
	}
	if stats.FollowerCount != 12 || stats.FollowingCount != 5 || !stats.IsFollowing {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRegisterPasskey(t *testing.T) {
	st := newHappyStore()
	userID := uuid.New()
	st.createCredentialFn = func(_ context.Context, p db.CreateCredentialParams) (db.UserCredential, error) {
		if p.Type != "passkey" {
			t.Fatalf("unexpected credential type: %s", p.Type)
		}
		if p.CredentialID == nil || *p.CredentialID == "" {
			t.Fatalf("expected credential id")
		}
		return db.UserCredential{ID: uuid.New(), UserID: p.UserID, Type: p.Type, CredentialID: p.CredentialID}, nil
	}
	st.updateTrustLevelAtLeastFn = func(_ context.Context, uid uuid.UUID, level int16) (db.User, error) {
		if uid != userID {
			t.Fatalf("unexpected user id")
		}
		if level != 1 {
			t.Fatalf("expected level 1, got %d", level)
		}
		return db.User{ID: uid, Username: "alice", TrustLevel: 1}, nil
	}
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	res, err := s.RegisterPasskey(context.Background(), userID, "cred-1", "pk-data", 0)
	if err != nil {
		t.Fatalf("RegisterPasskey error: %v", err)
	}
	if res.User.TrustLevel < 1 {
		t.Fatalf("expected trust level >= 1, got %d", res.User.TrustLevel)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected tokens")
	}
}

// buildCOSEKey encodes an ECDSA P-256 public key in CBOR/COSE Key format
// compatible with webauthncose.ParsePublicKey.
func buildCOSEKey(pub *ecdsa.PublicKey) []byte {
	x := pub.X.Bytes()
	y := pub.Y.Bytes()
	// Pad to 32 bytes
	for len(x) < 32 {
		x = append([]byte{0}, x...)
	}
	for len(y) < 32 {
		y = append([]byte{0}, y...)
	}
	// Minimal CBOR map: {1:2, 3:-7, -1:1, -2:x, -3:y}
	// kty=EC2(2), alg=ES256(-7), crv=P-256(1)
	buf := []byte{
		0xa5,       // map(5)
		0x01, 0x02, // 1: 2 (kty: EC2)
		0x03, 0x26, // 3: -7 (alg: ES256)
		0x20, 0x01, // -1: 1 (crv: P-256)
		0x21, 0x58, 0x20, // -2: bytes(32)
	}
	buf = append(buf, x...)
	buf = append(buf, 0x22, 0x58, 0x20) // -3: bytes(32)
	buf = append(buf, y...)
	return buf
}

// buildAuthenticatorData constructs a minimal authenticatorData for the given rpID with
// flags=0x01 (user present) and the given signCount.
func buildAuthenticatorData(rpID string, signCount uint32) []byte {
	rpIDHash := sha256.Sum256([]byte(rpID))
	data := make([]byte, 37)
	copy(data[:32], rpIDHash[:])
	data[32] = 0x01 // flags: UP=1
	binary.BigEndian.PutUint32(data[33:37], signCount)
	return data
}

// signWebAuthn signs the standard WebAuthn verification data (authData || sha256(clientDataJSON)).
func signWebAuthn(key *ecdsa.PrivateKey, authData, clientDataJSON []byte) []byte {
	clientDataHash := sha256.Sum256(clientDataJSON)
	verificationData := append(authData, clientDataHash[:]...)
	hash := sha256.Sum256(verificationData)
	r, s, _ := ecdsa.Sign(rand.Reader, key, hash[:])
	// Encode as ASN.1 DER — webauthncose expects this format for ES256
	// We'll use a simple manual DER encoding
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	// Pad with leading zero if high bit set (DER integer encoding)
	if rBytes[0]&0x80 != 0 {
		rBytes = append([]byte{0}, rBytes...)
	}
	if sBytes[0]&0x80 != 0 {
		sBytes = append([]byte{0}, sBytes...)
	}
	seq := []byte{0x02, byte(len(rBytes))}
	seq = append(seq, rBytes...)
	seq = append(seq, 0x02, byte(len(sBytes)))
	seq = append(seq, sBytes...)
	der := []byte{0x30, byte(len(seq))}
	der = append(der, seq...)
	return der
}

func TestPasskeyLoginFlow(t *testing.T) {
	// Generate a real ECDSA P-256 key pair for WebAuthn testing.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	coseKey := buildCOSEKey(&privKey.PublicKey)
	credData, _ := json.Marshal(map[string]any{
		"credentialPublicKey": base64.RawURLEncoding.EncodeToString(coseKey),
		"signCount":          0,
	})

	st := newHappyStore()
	uID := uuid.New()
	st.getUserByUsernameFn = func(context.Context, string) (db.User, error) {
		return db.User{ID: uID, Username: "alice", TrustLevel: 1}, nil
	}
	st.listCredentialIDsByUserAndTypeFn = func(context.Context, uuid.UUID, string) ([]string, error) {
		return []string{"cred-1"}, nil
	}
	st.getOAuthCredentialFn = func(context.Context, string, string) (db.UserCredential, error) {
		return db.UserCredential{UserID: uID, Type: "passkey", CredentialData: credData}, nil
	}
	st.getUserByIDFn = func(context.Context, uuid.UUID) (db.User, error) {
		return db.User{ID: uID, Username: "alice", TrustLevel: 1}, nil
	}

	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	s.SetPasskeyRPOrigin("http://localhost:3000")
	username := "alice"
	opts, err := s.BeginPasskeyLogin(context.Background(), &username)
	if err != nil {
		t.Fatalf("BeginPasskeyLogin error: %v", err)
	}
	if opts.Challenge == "" || opts.ChallengeToken == "" {
		t.Fatalf("expected challenge fields")
	}

	authData := buildAuthenticatorData("localhost", 1)
	clientDataJSON, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": opts.Challenge,
		"origin":    "http://localhost:3000",
	})
	sig := signWebAuthn(privKey, authData, clientDataJSON)

	assertion := PasskeyAssertion{
		CredentialID:      "cred-1",
		ChallengeToken:    opts.ChallengeToken,
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientDataJSON),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
		Signature:         base64.RawURLEncoding.EncodeToString(sig),
	}
	res, err := s.FinishPasskeyLogin(context.Background(), assertion)
	if err != nil {
		t.Fatalf("FinishPasskeyLogin error: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected auth tokens")
	}
}

func TestPasskeyLoginChallengeMismatch(t *testing.T) {
	st := newHappyStore()
	uID := uuid.New()
	st.getOAuthCredentialFn = func(context.Context, string, string) (db.UserCredential, error) {
		return db.UserCredential{UserID: uID, Type: "passkey"}, nil
	}
	st.getUserByIDFn = func(context.Context, uuid.UUID) (db.User, error) {
		return db.User{ID: uID, Username: "alice", TrustLevel: 1}, nil
	}
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	opts, err := s.BeginPasskeyLogin(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginPasskeyLogin error: %v", err)
	}
	clientData, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": "wrong",
	})
	_, err = s.FinishPasskeyLogin(context.Background(), PasskeyAssertion{
		CredentialID:      "cred-1",
		ChallengeToken:    opts.ChallengeToken,
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("auth")),
		Signature:         base64.RawURLEncoding.EncodeToString([]byte("sig")),
	})
	if err == nil || !strings.Contains(err.Error(), "challenge mismatch") {
		t.Fatalf("expected challenge mismatch, got %v", err)
	}
}

func TestBeginPasskeyLoginUserNotFound(t *testing.T) {
	st := newHappyStore()
	st.getUserByUsernameFn = func(context.Context, string) (db.User, error) { return db.User{}, pgx.ErrNoRows }
	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	username := "missing"
	_, err := s.BeginPasskeyLogin(context.Background(), &username)
	if err == nil || !strings.Contains(err.Error(), "user not found") {
		t.Fatalf("expected user not found, got %v", err)
	}
}

func TestFinishPasskeyLoginUsernameMismatch(t *testing.T) {
	// Generate a real ECDSA P-256 key pair.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	coseKey := buildCOSEKey(&privKey.PublicKey)
	credData, _ := json.Marshal(map[string]any{
		"credentialPublicKey": base64.RawURLEncoding.EncodeToString(coseKey),
		"signCount":          0,
	})

	st := newHappyStore()
	u1 := uuid.New()
	u2 := uuid.New()
	st.getUserByUsernameFn = func(_ context.Context, username string) (db.User, error) {
		if username == "alice" {
			return db.User{ID: u1, Username: "alice"}, nil
		}
		return db.User{ID: u2, Username: username}, nil
	}
	st.listCredentialIDsByUserAndTypeFn = func(context.Context, uuid.UUID, string) ([]string, error) {
		return []string{"cred-1"}, nil
	}
	st.getOAuthCredentialFn = func(context.Context, string, string) (db.UserCredential, error) {
		return db.UserCredential{UserID: u2, Type: "passkey", CredentialData: credData}, nil
	}

	s := NewAuthService(st, NewTokenService("a", "b", time.Minute, time.Hour), "cid")
	s.SetPasskeyRPOrigin("http://localhost:3000")
	username := "alice"
	opts, err := s.BeginPasskeyLogin(context.Background(), &username)
	if err != nil {
		t.Fatalf("BeginPasskeyLogin error: %v", err)
	}

	authData := buildAuthenticatorData("localhost", 1)
	clientDataJSON, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": opts.Challenge,
		"origin":    "http://localhost:3000",
	})
	sig := signWebAuthn(privKey, authData, clientDataJSON)

	_, err = s.FinishPasskeyLogin(context.Background(), PasskeyAssertion{
		CredentialID:      "cred-1",
		ChallengeToken:    opts.ChallengeToken,
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientDataJSON),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
		Signature:         base64.RawURLEncoding.EncodeToString(sig),
	})
	if err == nil || !strings.Contains(err.Error(), "credential-user mismatch") {
		t.Fatalf("expected credential-user mismatch, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func bcryptGenerate(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}
