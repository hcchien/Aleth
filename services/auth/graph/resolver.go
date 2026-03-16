package graph

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/google/uuid"
	graphql "github.com/graph-gophers/graphql-go"

	"github.com/aleth/auth/internal/db"
	"github.com/aleth/auth/internal/service"
)

//go:embed schema.graphqls
var schemaString string

// NewSchema builds the executable GraphQL schema wired to the given services.
func NewSchema(auth *service.AuthService, tokens *service.TokenService) *graphql.Schema {
	return graphql.MustParseSchema(
		schemaString,
		&Resolver{auth: auth, tokens: tokens},
		graphql.UseStringDescriptions(),
	)
}

// ─── Context helpers ──────────────────────────────────────────────────────────

type contextKey string

const claimsKey contextKey = "claims"

// WithClaims injects validated JWT claims into the context.
func WithClaims(ctx context.Context, claims *service.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext retrieves JWT claims from the context (set by auth middleware).
func ClaimsFromContext(ctx context.Context) (*service.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*service.Claims)
	return c, ok
}

// ─── Root resolver ────────────────────────────────────────────────────────────

// Resolver is the root GraphQL resolver.
type Resolver struct {
	auth   *service.AuthService
	tokens *service.TokenService
}

// ─── Query resolvers ──────────────────────────────────────────────────────────

// Me returns the currently authenticated user, or nil if unauthenticated.
func (r *Resolver) Me(ctx context.Context) (*UserResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, nil
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	user, err := r.auth.GetMe(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &UserResolver{user: *user}, nil
}

// DidDocument returns a stub DID document for the given DID.
func (r *Resolver) DidDocument(ctx context.Context, args struct{ Did string }) (*string, error) {
	s := fmt.Sprintf(`{"id":%q,"@context":"https://www.w3.org/ns/did/v1"}`, args.Did)
	return &s, nil
}

func (r *Resolver) FollowStats(ctx context.Context, args struct{ UserID graphql.ID }) (*FollowStatsResolver, error) {
	targetID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}
	var viewerID *uuid.UUID
	if claims, ok := ClaimsFromContext(ctx); ok {
		id, err := uuid.Parse(claims.Subject)
		if err == nil {
			viewerID = &id
		}
	}
	stats, err := r.auth.FollowStats(ctx, viewerID, targetID)
	if err != nil {
		return nil, err
	}
	return &FollowStatsResolver{stats: *stats}, nil
}

// ─── Mutation resolvers ───────────────────────────────────────────────────────

// RegisterInput mirrors the GraphQL RegisterInput input type.
type RegisterInput struct {
	Username string
	Email    string
	Password string
}

// Register creates a new user account.
func (r *Resolver) Register(ctx context.Context, args struct{ Input RegisterInput }) (*AuthPayloadResolver, error) {
	result, err := r.auth.Register(ctx, args.Input.Username, args.Input.Email, args.Input.Password)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// LoginInput mirrors the GraphQL LoginInput input type.
type LoginInput struct {
	Email    string
	Password string
}

type PasskeyAssertionInput struct {
	CredentialID      string
	ChallengeToken    string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	UserHandle        *string
	Username          *string
}

// Login authenticates with email and password.
func (r *Resolver) Login(ctx context.Context, args struct{ Input LoginInput }) (*AuthPayloadResolver, error) {
	result, err := r.auth.Login(ctx, args.Input.Email, args.Input.Password)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// LoginWithGoogle validates a Google ID token and returns a session.
func (r *Resolver) LoginWithGoogle(ctx context.Context, args struct{ IdToken string }) (*AuthPayloadResolver, error) {
	result, err := r.auth.LoginWithGoogle(ctx, args.IdToken)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// LoginWithFacebook validates a Facebook access token and returns a session.
func (r *Resolver) LoginWithFacebook(ctx context.Context, args struct{ AccessToken string }) (*AuthPayloadResolver, error) {
	result, err := r.auth.LoginWithFacebook(ctx, args.AccessToken)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

func (r *Resolver) BeginPasskeyLogin(ctx context.Context, args struct{ Username *string }) (*PasskeyLoginOptionsResolver, error) {
	opts, err := r.auth.BeginPasskeyLogin(ctx, args.Username)
	if err != nil {
		return nil, err
	}
	return &PasskeyLoginOptionsResolver{options: opts}, nil
}

func (r *Resolver) FinishPasskeyLogin(ctx context.Context, args struct{ Input PasskeyAssertionInput }) (*AuthPayloadResolver, error) {
	result, err := r.auth.FinishPasskeyLogin(ctx, service.PasskeyAssertion{
		CredentialID:      args.Input.CredentialID,
		ChallengeToken:    args.Input.ChallengeToken,
		ClientDataJSON:    args.Input.ClientDataJSON,
		AuthenticatorData: args.Input.AuthenticatorData,
		Signature:         args.Input.Signature,
		UserHandle:        args.Input.UserHandle,
		Username:          args.Input.Username,
	})
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

func (r *Resolver) RegisterPasskey(ctx context.Context, args struct {
	CredentialID        string
	CredentialPublicKey string
	SignCount           int32
}) (*AuthPayloadResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	result, err := r.auth.RegisterPasskey(ctx, uid, args.CredentialID, args.CredentialPublicKey, args.SignCount)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// RefreshToken rotates a refresh token and returns a new token pair.
func (r *Resolver) RefreshToken(ctx context.Context, args struct{ Token string }) (*AuthPayloadResolver, error) {
	result, err := r.auth.RefreshToken(ctx, args.Token)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// RevokeToken revokes all refresh tokens for the authenticated user (logout everywhere).
func (r *Resolver) RevokeToken(ctx context.Context) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	if err := r.auth.RevokeAllTokens(ctx, uid); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Resolver) FollowUser(ctx context.Context, args struct{ UserID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	followerID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	followeeID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return false, fmt.Errorf("invalid user id")
	}
	if err := r.auth.FollowUser(ctx, followerID, followeeID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Resolver) UnfollowUser(ctx context.Context, args struct{ UserID graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	followerID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	followeeID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return false, fmt.Errorf("invalid user id")
	}
	if err := r.auth.UnfollowUser(ctx, followerID, followeeID); err != nil {
		return false, err
	}
	return true, nil
}

// ─── VC query / mutation resolvers ───────────────────────────────────────────

func (r *Resolver) MyVcs(ctx context.Context) ([]*UserVcResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return []*UserVcResolver{}, nil
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	vcs, err := r.auth.GetUserVCs(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*UserVcResolver, len(vcs))
	for i, v := range vcs {
		v := v
		out[i] = &UserVcResolver{vc: v}
	}
	return out, nil
}

type UpsertUserVcInput struct {
	VcType     string
	Issuer     string
	Attributes string  // JSON string
	ExpiresAt  *string // ISO-8601
}

func (r *Resolver) UpsertUserVc(ctx context.Context, args struct{ Input UpsertUserVcInput }) (*UserVcResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	var expiresAt *time.Time
	if args.Input.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *args.Input.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid expiresAt: must be RFC3339")
		}
		expiresAt = &t
	}
	vc, err := r.auth.UpsertUserVC(ctx, userID, args.Input.VcType, args.Input.Issuer, []byte(args.Input.Attributes), expiresAt)
	if err != nil {
		return nil, err
	}
	return &UserVcResolver{vc: vc}, nil
}

func (r *Resolver) RevokeUserVc(ctx context.Context, args struct{ VcId graphql.ID }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	vcID, err := uuid.Parse(string(args.VcId))
	if err != nil {
		return false, fmt.Errorf("invalid vc id")
	}
	if err := r.auth.RevokeUserVC(ctx, vcID, userID); err != nil {
		return false, err
	}
	return true, nil
}

// ─── VC type registry resolvers ───────────────────────────────────────────────

func (r *Resolver) AvailableVcTypes(ctx context.Context) ([]*VcTypeInfoResolver, error) {
	entries, err := r.auth.ListVcTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*VcTypeInfoResolver, len(entries))
	for i, e := range entries {
		e := e
		out[i] = &VcTypeInfoResolver{entry: e}
	}
	return out, nil
}

type RegisterVcTypeInput struct {
	VcType      string
	Label       string
	Description *string
}

func (r *Resolver) RegisterVcType(ctx context.Context, args struct{ Input RegisterVcTypeInput }) (*VcTypeInfoResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	callerID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	// issuer namespace = caller's username
	caller, err := r.auth.GetMe(ctx, callerID)
	if err != nil {
		return nil, err
	}
	entry, err := r.auth.RegisterVcType(ctx,
		args.Input.VcType, caller.Username, args.Input.Label,
		args.Input.Description, callerID,
	)
	if err != nil {
		return nil, err
	}
	return &VcTypeInfoResolver{entry: entry}, nil
}

func (r *Resolver) DisableVcType(ctx context.Context, args struct {
	VcType string
	Issuer string
}) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	ownerID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	if err := r.auth.DisableVcType(ctx, args.VcType, args.Issuer, ownerID); err != nil {
		return false, err
	}
	return true, nil
}

// ─── Reputation resolvers ─────────────────────────────────────────────────────

// MyReputation returns the authenticated user's L2 reputation status.
func (r *Resolver) MyReputation(ctx context.Context) (*ReputationStatusResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	stamps, err := r.auth.GetReputationStamps(ctx, userID)
	if err != nil {
		return nil, err
	}
	total, _, err := r.auth.EvaluateL2(ctx, userID)
	if err != nil {
		return nil, err
	}
	user, err := r.auth.GetMe(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &ReputationStatusResolver{
		stamps:     stamps,
		totalScore: total,
		isL2:       user.TrustLevel >= 2,
	}, nil
}

// RequestPhoneOTP sends a 6-digit OTP to the given phone number.
func (r *Resolver) RequestPhoneOTP(ctx context.Context, args struct{ Phone string }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	devCode, err := r.auth.RequestPhoneOTP(ctx, userID, args.Phone)
	if err != nil {
		return false, err
	}
	// In dev mode the OTP is logged; in prod it would be sent via SMS.
	_ = devCode
	return true, nil
}

// VerifyPhoneOTP checks the OTP and records the phone stamp on success.
func (r *Resolver) VerifyPhoneOTP(ctx context.Context, args struct {
	Phone string
	Code  string
}) (*AuthPayloadResolver, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject claim")
	}
	result, err := r.auth.VerifyPhoneOTP(ctx, userID, args.Phone, args.Code)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{result: result}, nil
}

// SetActivityPubEnabled opts the current user in or out of ActivityPub federation.
func (r *Resolver) SetActivityPubEnabled(ctx context.Context, args struct{ Enabled bool }) (bool, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("not authenticated")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("invalid subject claim")
	}
	if _, err := r.auth.SetAPEnabled(ctx, userID, args.Enabled); err != nil {
		return false, err
	}
	return true, nil
}

// VcTypeInfoResolver resolves GraphQL VcTypeInfo fields.
type VcTypeInfoResolver struct {
	entry           db.VcTypeEntry
	createdByUser   *string // lazily resolved username, nil = not fetched yet
}

func (r *VcTypeInfoResolver) VcType() string      { return r.entry.VcType }
func (r *VcTypeInfoResolver) Issuer() string      { return r.entry.Issuer }
func (r *VcTypeInfoResolver) Label() string       { return r.entry.Label }
func (r *VcTypeInfoResolver) Description() *string { return r.entry.Description }
func (r *VcTypeInfoResolver) CreatedByUsername() *string {
	// Platform built-ins have no created_by row.
	if r.entry.CreatedBy == nil {
		return nil
	}
	// The issuer IS the username for user-registered types, so return that.
	return &r.entry.Issuer
}

// ─── Type resolvers ───────────────────────────────────────────────────────────

// UserResolver resolves fields on the GraphQL User type.
type UserResolver struct {
	user db.User
}

func (r *UserResolver) ID() graphql.ID       { return graphql.ID(r.user.ID.String()) }
func (r *UserResolver) Did() string          { return r.user.DID }
func (r *UserResolver) Username() string     { return r.user.Username }
func (r *UserResolver) DisplayName() *string { return r.user.DisplayName }
func (r *UserResolver) Email() *string       { return r.user.Email }
func (r *UserResolver) TrustLevel() int32    { return int32(r.user.TrustLevel) }
func (r *UserResolver) ApEnabled() bool      { return r.user.APEnabled }
func (r *UserResolver) CreatedAt() string    { return r.user.CreatedAt.UTC().Format(time.RFC3339) }

// AuthPayloadResolver resolves fields on the GraphQL AuthPayload type.
type AuthPayloadResolver struct {
	result *service.AuthResult
}

func (r *AuthPayloadResolver) AccessToken() string  { return r.result.AccessToken }
func (r *AuthPayloadResolver) RefreshToken() string { return r.result.RefreshToken }
func (r *AuthPayloadResolver) User() *UserResolver  { return &UserResolver{user: r.result.User} }

type PasskeyLoginOptionsResolver struct {
	options *service.PasskeyLoginOptions
}

func (r *PasskeyLoginOptionsResolver) Challenge() string      { return r.options.Challenge }
func (r *PasskeyLoginOptionsResolver) ChallengeToken() string { return r.options.ChallengeToken }
func (r *PasskeyLoginOptionsResolver) RpId() string           { return r.options.RPID }
func (r *PasskeyLoginOptionsResolver) TimeoutMs() int32       { return r.options.TimeoutMs }
func (r *PasskeyLoginOptionsResolver) AllowCredentialIds() []string {
	return r.options.AllowCredentialIDs
}

type FollowStatsResolver struct {
	stats service.FollowStats
}

func (r *FollowStatsResolver) FollowerCount() int32  { return int32(r.stats.FollowerCount) }
func (r *FollowStatsResolver) FollowingCount() int32 { return int32(r.stats.FollowingCount) }
func (r *FollowStatsResolver) IsFollowing() bool     { return r.stats.IsFollowing }

// UserVcResolver resolves fields on the GraphQL UserVc type.
type UserVcResolver struct {
	vc db.UserVC
}

func (r *UserVcResolver) ID() graphql.ID      { return graphql.ID(r.vc.ID.String()) }
func (r *UserVcResolver) VcType() string      { return r.vc.VcType }
func (r *UserVcResolver) Issuer() string      { return r.vc.Issuer }
func (r *UserVcResolver) Attributes() string  { return string(r.vc.Attributes) }
func (r *UserVcResolver) VerifiedAt() string  { return r.vc.VerifiedAt.UTC().Format(time.RFC3339) }
func (r *UserVcResolver) ExpiresAt() *string {
	if r.vc.ExpiresAt == nil {
		return nil
	}
	s := r.vc.ExpiresAt.UTC().Format(time.RFC3339)
	return &s
}
func (r *UserVcResolver) RevokedAt() *string {
	if r.vc.RevokedAt == nil {
		return nil
	}
	s := r.vc.RevokedAt.UTC().Format(time.RFC3339)
	return &s
}
func (r *UserVcResolver) IsValid() bool { return r.vc.IsValid() }

// ─── Reputation type resolvers ────────────────────────────────────────────────

// ReputationStatusResolver resolves the ReputationStatus GraphQL type.
type ReputationStatusResolver struct {
	stamps     []db.ReputationStamp
	totalScore int
	isL2       bool
}

func (r *ReputationStatusResolver) Stamps() []*ReputationStampResolver {
	out := make([]*ReputationStampResolver, len(r.stamps))
	for i, s := range r.stamps {
		s := s
		out[i] = &ReputationStampResolver{stamp: s}
	}
	return out
}
func (r *ReputationStatusResolver) TotalScore() int32 { return int32(r.totalScore) }
func (r *ReputationStatusResolver) Threshold() int32  { return int32(service.L2ScoreThreshold) }
func (r *ReputationStatusResolver) IsL2() bool        { return r.isL2 }

// ReputationStampResolver resolves a single ReputationStamp.
type ReputationStampResolver struct {
	stamp db.ReputationStamp
}

func (r *ReputationStampResolver) Provider() string  { return r.stamp.Provider }
func (r *ReputationStampResolver) Score() int32      { return int32(r.stamp.Score) }
func (r *ReputationStampResolver) MaxScore() int32 {
	if max, ok := service.ProviderMaxScore[r.stamp.Provider]; ok {
		return int32(max)
	}
	return int32(r.stamp.Score)
}
func (r *ReputationStampResolver) VerifiedAt() string { return r.stamp.VerifiedAt.UTC().Format(time.RFC3339) }
func (r *ReputationStampResolver) ExpiresAt() *string {
	if r.stamp.ExpiresAt == nil {
		return nil
	}
	s := r.stamp.ExpiresAt.UTC().Format(time.RFC3339)
	return &s
}
func (r *ReputationStampResolver) IsValid() bool { return r.stamp.IsValid() }
