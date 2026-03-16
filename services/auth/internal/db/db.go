package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool and provides all DB operations for the auth service.
type Pool struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Pool{pool: pool}, nil
}

func (p *Pool) Close() {
	p.pool.Close()
}

// ─── Models ──────────────────────────────────────────────────────────────────

type User struct {
	ID            uuid.UUID
	DID           string
	Username      string
	DisplayName   *string
	Email         *string
	EmailVerified bool
	TrustLevel    int16
	IsSuspended   bool
	APEnabled     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserCredential struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Type           string // 'password' | 'google' | 'passkey'
	CredentialID   *string
	CredentialData []byte
	SignCount      *int64
	CreatedAt      time.Time
}

type RefreshToken struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	TokenHash        string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	SessionTrustLevel int16
}

// ─── User queries ─────────────────────────────────────────────────────────────

type CreateUserParams struct {
	DID      string
	Username string
	Email    *string
}

func (p *Pool) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	const q = `
		INSERT INTO users (did, username, email)
		VALUES ($1, $2, $3)
		RETURNING id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
	`
	return scanUser(p.pool.QueryRow(ctx, q, params.DID, params.Username, params.Email))
}

func (p *Pool) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	const q = `
		SELECT id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
		FROM users WHERE id = $1
	`
	return scanUser(p.pool.QueryRow(ctx, q, id))
}

func (p *Pool) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `
		SELECT id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
		FROM users WHERE email = $1
	`
	return scanUser(p.pool.QueryRow(ctx, q, email))
}

func (p *Pool) GetUserByUsername(ctx context.Context, username string) (User, error) {
	const q = `
		SELECT id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
		FROM users WHERE username = $1
	`
	return scanUser(p.pool.QueryRow(ctx, q, username))
}

func (p *Pool) ExistsEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

func (p *Pool) ExistsUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	return exists, err
}

// GetUsersByIDs returns all users with the given IDs (order not guaranteed).
func (p *Pool) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const q = `
		SELECT id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
		FROM users WHERE id = ANY($1)
	`
	rows, err := p.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("query users by ids: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID, &u.DID, &u.Username, &u.DisplayName,
			&u.Email, &u.EmailVerified, &u.TrustLevel,
			&u.IsSuspended, &u.APEnabled, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (p *Pool) UpdateTrustLevelAtLeast(ctx context.Context, userID uuid.UUID, minLevel int16) (User, error) {
	const q = `
		UPDATE users
		SET trust_level = GREATEST(trust_level, $2), updated_at = now()
		WHERE id = $1
		RETURNING id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
	`
	return scanUser(p.pool.QueryRow(ctx, q, userID, minLevel))
}

// SetAPEnabled updates whether the user's profile is federated via ActivityPub.
func (p *Pool) SetAPEnabled(ctx context.Context, userID uuid.UUID, enabled bool) (User, error) {
	const q = `
		UPDATE users
		SET ap_enabled = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, did, username, display_name, email, email_verified, trust_level, is_suspended, ap_enabled, created_at, updated_at
	`
	return scanUser(p.pool.QueryRow(ctx, q, userID, enabled))
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.DID, &u.Username, &u.DisplayName,
		&u.Email, &u.EmailVerified, &u.TrustLevel,
		&u.IsSuspended, &u.APEnabled, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

// ─── Credential queries ───────────────────────────────────────────────────────

type CreateCredentialParams struct {
	UserID         uuid.UUID
	Type           string
	CredentialID   *string
	CredentialData []byte
}

func (p *Pool) CreateCredential(ctx context.Context, params CreateCredentialParams) (UserCredential, error) {
	const q = `
		INSERT INTO user_credentials (user_id, type, credential_id, credential_data)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, type, credential_id, credential_data, sign_count, created_at
	`
	var c UserCredential
	err := p.pool.QueryRow(ctx, q,
		params.UserID, params.Type, params.CredentialID, params.CredentialData,
	).Scan(&c.ID, &c.UserID, &c.Type, &c.CredentialID, &c.CredentialData, &c.SignCount, &c.CreatedAt)
	if err != nil {
		return UserCredential{}, fmt.Errorf("create credential: %w", err)
	}
	return c, nil
}

func (p *Pool) GetPasswordCredential(ctx context.Context, userID uuid.UUID) (UserCredential, error) {
	const q = `
		SELECT id, user_id, type, credential_id, credential_data, sign_count, created_at
		FROM user_credentials WHERE user_id = $1 AND type = 'password'
		LIMIT 1
	`
	var c UserCredential
	err := p.pool.QueryRow(ctx, q, userID).Scan(
		&c.ID, &c.UserID, &c.Type, &c.CredentialID, &c.CredentialData, &c.SignCount, &c.CreatedAt,
	)
	if err != nil {
		return UserCredential{}, fmt.Errorf("get password credential: %w", err)
	}
	return c, nil
}

// GetOAuthCredential finds a credential by (type, credential_id).
// Used to look up Google/Facebook OAuth users by their sub claim.
func (p *Pool) GetOAuthCredential(ctx context.Context, credType, credentialID string) (UserCredential, error) {
	const q = `
		SELECT id, user_id, type, credential_id, credential_data, sign_count, created_at
		FROM user_credentials WHERE type = $1 AND credential_id = $2
		LIMIT 1
	`
	var c UserCredential
	err := p.pool.QueryRow(ctx, q, credType, credentialID).Scan(
		&c.ID, &c.UserID, &c.Type, &c.CredentialID, &c.CredentialData, &c.SignCount, &c.CreatedAt,
	)
	if err != nil {
		return UserCredential{}, fmt.Errorf("get oauth credential: %w", err)
	}
	return c, nil
}

func (p *Pool) ListCredentialIDsByUserAndType(ctx context.Context, userID uuid.UUID, credType string) ([]string, error) {
	const q = `
		SELECT credential_id
		FROM user_credentials
		WHERE user_id = $1 AND type = $2 AND credential_id IS NOT NULL
		ORDER BY created_at DESC
	`
	rows, err := p.pool.Query(ctx, q, userID, credType)
	if err != nil {
		return nil, fmt.Errorf("list credential ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan credential id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ─── Refresh token queries ────────────────────────────────────────────────────

type CreateRefreshTokenParams struct {
	UserID            uuid.UUID
	TokenHash         string
	ExpiresAt         time.Time
	SessionTrustLevel int16
}

func (p *Pool) CreateRefreshToken(ctx context.Context, params CreateRefreshTokenParams) (RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, session_trust_level)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at, session_trust_level
	`
	var t RefreshToken
	err := p.pool.QueryRow(ctx, q, params.UserID, params.TokenHash, params.ExpiresAt, params.SessionTrustLevel).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt, &t.SessionTrustLevel,
	)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("create refresh token: %w", err)
	}
	return t, nil
}

func (p *Pool) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at, session_trust_level
		FROM refresh_tokens WHERE token_hash = $1
	`
	var t RefreshToken
	err := p.pool.QueryRow(ctx, q, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt, &t.SessionTrustLevel,
	)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("get refresh token: %w", err)
	}
	return t, nil
}

func (p *Pool) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, id,
	)
	return err
}

func (p *Pool) RevokeAllRefreshTokensForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID,
	)
	return err
}

// ─── Follow queries ───────────────────────────────────────────────────────────

func (p *Pool) FollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	const q = `
		INSERT INTO user_follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	_, err := p.pool.Exec(ctx, q, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("follow user: %w", err)
	}
	return nil
}

func (p *Pool) UnfollowUser(ctx context.Context, followerID, followeeID uuid.UUID) error {
	const q = `DELETE FROM user_follows WHERE follower_id = $1 AND followee_id = $2`
	_, err := p.pool.Exec(ctx, q, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("unfollow user: %w", err)
	}
	return nil
}

func (p *Pool) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM user_follows WHERE follower_id = $1 AND followee_id = $2)`
	var exists bool
	if err := p.pool.QueryRow(ctx, q, followerID, followeeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is following: %w", err)
	}
	return exists, nil
}

func (p *Pool) CountFollowers(ctx context.Context, userID uuid.UUID) (int64, error) {
	const q = `SELECT COUNT(*) FROM user_follows WHERE followee_id = $1`
	var count int64
	if err := p.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count followers: %w", err)
	}
	return count, nil
}

func (p *Pool) CountFollowing(ctx context.Context, userID uuid.UUID) (int64, error) {
	const q = `SELECT COUNT(*) FROM user_follows WHERE follower_id = $1`
	var count int64
	if err := p.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count following: %w", err)
	}
	return count, nil
}

// ─── Verifiable Credential queries ───────────────────────────────────────────

// UserVC is a verified credential attached to a user account.
type UserVC struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	VcType     string  // "NationalID", "Journalist", "MedicalLicense"
	Issuer     string  // "GOV_TW", "PRESS_ASSOC_TW"
	Attributes []byte  // raw JSONB — caller unmarshals as needed
	VerifiedAt time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// IsValid reports whether the VC is currently valid (not revoked, not expired).
func (v UserVC) IsValid() bool {
	if v.RevokedAt != nil {
		return false
	}
	if v.ExpiresAt != nil && v.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

type UpsertUserVCParams struct {
	UserID     uuid.UUID
	VcType     string
	Issuer     string
	Attributes []byte // JSON
	ExpiresAt  *time.Time
}

// UpsertUserVC inserts or updates a VC for a user.  Re-verifying the same
// credential type from the same issuer refreshes verified_at and attributes.
func (p *Pool) UpsertUserVC(ctx context.Context, params UpsertUserVCParams) (UserVC, error) {
	const q = `
		INSERT INTO user_vcs (user_id, vc_type, issuer, attributes, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, vc_type, issuer) DO UPDATE
		  SET attributes  = EXCLUDED.attributes,
		      expires_at  = EXCLUDED.expires_at,
		      verified_at = now(),
		      revoked_at  = NULL
		RETURNING id, user_id, vc_type, issuer, attributes, verified_at, expires_at, revoked_at, created_at
	`
	return scanUserVC(p.pool.QueryRow(ctx, q,
		params.UserID, params.VcType, params.Issuer, params.Attributes, params.ExpiresAt,
	))
}

// GetUserVCs returns all VCs for a user, including expired and revoked ones.
// Callers should filter with IsValid() if they only want active credentials.
func (p *Pool) GetUserVCs(ctx context.Context, userID uuid.UUID) ([]UserVC, error) {
	const q = `
		SELECT id, user_id, vc_type, issuer, attributes, verified_at, expires_at, revoked_at, created_at
		FROM user_vcs WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := p.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get user vcs: %w", err)
	}
	defer rows.Close()

	var vcs []UserVC
	for rows.Next() {
		vc, err := scanUserVC(rows)
		if err != nil {
			return nil, err
		}
		vcs = append(vcs, vc)
	}
	return vcs, rows.Err()
}

// RevokeUserVC soft-deletes a VC by setting revoked_at.  Only the owning
// user or an admin should be permitted to call this.
func (p *Pool) RevokeUserVC(ctx context.Context, vcID, userID uuid.UUID) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE user_vcs SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		vcID, userID,
	)
	if err != nil {
		return fmt.Errorf("revoke user vc: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("vc not found or already revoked")
	}
	return nil
}

func scanUserVC(row interface {
	Scan(...any) error
}) (UserVC, error) {
	var v UserVC
	err := row.Scan(
		&v.ID, &v.UserID, &v.VcType, &v.Issuer,
		&v.Attributes, &v.VerifiedAt, &v.ExpiresAt, &v.RevokedAt,
		&v.CreatedAt,
	)
	if err != nil {
		return UserVC{}, fmt.Errorf("scan user vc: %w", err)
	}
	return v, nil
}

// ─── VC type registry ─────────────────────────────────────────────────────────

// VcTypeEntry is a row in vc_type_registry.
type VcTypeEntry struct {
	ID          uuid.UUID
	VcType      string
	Issuer      string
	Label       string
	Description *string
	CreatedBy   *uuid.UUID // nil for platform built-ins
	Enabled     bool
	CreatedAt   time.Time
}

// ListVcTypes returns all enabled entries, optionally filtered by issuer.
func (p *Pool) ListVcTypes(ctx context.Context) ([]VcTypeEntry, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, vc_type, issuer, label, description, created_by, enabled, created_at
		FROM vc_type_registry
		WHERE enabled = true
		ORDER BY issuer, vc_type
	`)
	if err != nil {
		return nil, fmt.Errorf("list vc types: %w", err)
	}
	defer rows.Close()
	var out []VcTypeEntry
	for rows.Next() {
		var e VcTypeEntry
		if err := rows.Scan(&e.ID, &e.VcType, &e.Issuer, &e.Label, &e.Description, &e.CreatedBy, &e.Enabled, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vc type: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RegisterVcType inserts a new entry; returns an existing one on conflict.
func (p *Pool) RegisterVcType(ctx context.Context, vcType, issuer, label string, description *string, createdBy uuid.UUID) (VcTypeEntry, error) {
	var e VcTypeEntry
	err := p.pool.QueryRow(ctx, `
		INSERT INTO vc_type_registry (vc_type, issuer, label, description, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (vc_type, issuer) DO UPDATE
			SET label       = EXCLUDED.label,
			    description = EXCLUDED.description,
			    enabled     = true
		RETURNING id, vc_type, issuer, label, description, created_by, enabled, created_at
	`, vcType, issuer, label, description, createdBy).Scan(
		&e.ID, &e.VcType, &e.Issuer, &e.Label, &e.Description, &e.CreatedBy, &e.Enabled, &e.CreatedAt,
	)
	if err != nil {
		return VcTypeEntry{}, fmt.Errorf("register vc type: %w", err)
	}
	return e, nil
}

// DisableVcType soft-deletes a registry entry owned by ownerID.
func (p *Pool) DisableVcType(ctx context.Context, vcType, issuer string, ownerID uuid.UUID) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE vc_type_registry SET enabled = false
		WHERE vc_type = $1 AND issuer = $2 AND created_by = $3
	`, vcType, issuer, ownerID)
	if err != nil {
		return fmt.Errorf("disable vc type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("vc type not found or not owned by caller")
	}
	return nil
}

// ─── Reputation stamps ────────────────────────────────────────────────────────

// ReputationStamp is one verified social-proof data point for a user.
type ReputationStamp struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string    // "phone" | "instagram" | "facebook" | "twitter" | "linkedin"
	ProviderUserID *string   // external account ID or phone number
	Score          int16
	Metadata       []byte    // JSONB: account_age_days, follower_count, etc.
	VerifiedAt     time.Time
	ExpiresAt      *time.Time
}

// IsValid reports whether the stamp is still current (not expired).
func (s ReputationStamp) IsValid() bool {
	if s.ExpiresAt == nil {
		return true
	}
	return time.Now().Before(*s.ExpiresAt)
}

// UpsertReputationStamp inserts or refreshes a reputation stamp.
func (p *Pool) UpsertReputationStamp(ctx context.Context, userID uuid.UUID, provider, providerUserID string, score int16, metadata []byte, expiresAt *time.Time) (ReputationStamp, error) {
	const q = `
		INSERT INTO reputation_stamps (user_id, provider, provider_user_id, score, metadata, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, provider) DO UPDATE
		  SET provider_user_id = EXCLUDED.provider_user_id,
		      score             = EXCLUDED.score,
		      metadata          = EXCLUDED.metadata,
		      verified_at       = NOW(),
		      expires_at        = EXCLUDED.expires_at
		RETURNING id, user_id, provider, provider_user_id, score, metadata, verified_at, expires_at
	`
	var s ReputationStamp
	var puid *string
	if providerUserID != "" {
		puid = &providerUserID
	}
	row := p.pool.QueryRow(ctx, q, userID, provider, puid, score, metadata, expiresAt)
	err := row.Scan(&s.ID, &s.UserID, &s.Provider, &s.ProviderUserID, &s.Score, &s.Metadata, &s.VerifiedAt, &s.ExpiresAt)
	if err != nil {
		return ReputationStamp{}, fmt.Errorf("upsert reputation stamp: %w", err)
	}
	return s, nil
}

// GetReputationStamps returns all stamps for a user.
func (p *Pool) GetReputationStamps(ctx context.Context, userID uuid.UUID) ([]ReputationStamp, error) {
	const q = `
		SELECT id, user_id, provider, provider_user_id, score, metadata, verified_at, expires_at
		FROM reputation_stamps
		WHERE user_id = $1
		ORDER BY provider
	`
	rows, err := p.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get reputation stamps: %w", err)
	}
	defer rows.Close()
	var stamps []ReputationStamp
	for rows.Next() {
		var s ReputationStamp
		if err := rows.Scan(&s.ID, &s.UserID, &s.Provider, &s.ProviderUserID, &s.Score, &s.Metadata, &s.VerifiedAt, &s.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan reputation stamp: %w", err)
		}
		stamps = append(stamps, s)
	}
	return stamps, rows.Err()
}

// SumValidStampScore returns the total score from non-expired stamps for a user.
func (p *Pool) SumValidStampScore(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `
		SELECT COALESCE(SUM(score), 0)
		FROM reputation_stamps
		WHERE user_id = $1
		  AND (expires_at IS NULL OR expires_at > NOW())
	`
	var total int
	err := p.pool.QueryRow(ctx, q, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum stamp score: %w", err)
	}
	return total, nil
}

// ─── Phone OTP ────────────────────────────────────────────────────────────────

// UpsertPhoneOTP stores (or replaces) a one-time code for phone verification.
func (p *Pool) UpsertPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string, expiresAt time.Time) error {
	const q = `
		INSERT INTO phone_otps (user_id, phone, code, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, phone) DO UPDATE
		  SET code       = EXCLUDED.code,
		      attempts   = 0,
		      expires_at = EXCLUDED.expires_at
	`
	_, err := p.pool.Exec(ctx, q, userID, phone, code, expiresAt)
	return err
}

// VerifyPhoneOTP checks the code, increments attempts, deletes on success.
// Returns (true, nil) on match; (false, nil) on wrong code; (false, err) on system error.
func (p *Pool) VerifyPhoneOTP(ctx context.Context, userID uuid.UUID, phone, code string) (bool, error) {
	// Fetch the record (also checks expiry).
	var storedCode string
	var expiresAt time.Time
	var attempts int16
	err := p.pool.QueryRow(ctx,
		`SELECT code, expires_at, attempts FROM phone_otps WHERE user_id = $1 AND phone = $2`,
		userID, phone,
	).Scan(&storedCode, &expiresAt, &attempts)
	if err != nil {
		return false, nil // not found → treat as wrong code
	}
	if time.Now().After(expiresAt) {
		_, _ = p.pool.Exec(ctx, `DELETE FROM phone_otps WHERE user_id = $1 AND phone = $2`, userID, phone)
		return false, fmt.Errorf("OTP expired")
	}
	if attempts >= 5 {
		return false, fmt.Errorf("too many attempts")
	}
	if storedCode != code {
		_, _ = p.pool.Exec(ctx,
			`UPDATE phone_otps SET attempts = attempts + 1 WHERE user_id = $1 AND phone = $2`,
			userID, phone,
		)
		return false, nil
	}
	// Correct — clean up.
	_, _ = p.pool.Exec(ctx, `DELETE FROM phone_otps WHERE user_id = $1 AND phone = $2`, userID, phone)
	return true, nil
}
