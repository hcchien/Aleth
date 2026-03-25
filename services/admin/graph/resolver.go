package graph

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	graphql "github.com/graph-gophers/graphql-go"
	"github.com/rs/zerolog/log"

	"github.com/aleth/admin/internal/db"
	"github.com/aleth/admin/internal/service"
)

//go:embed schema.graphqls
var schemaString string

func NewSchema(svc *service.AdminService) *graphql.Schema {
	return graphql.MustParseSchema(
		schemaString,
		&Resolver{svc: svc},
		graphql.UseStringDescriptions(),
	)
}

// ─── Context helpers ──────────────────────────────────────────────────────────

type contextKey string

const adminKey contextKey = "admin"

func WithAdmin(ctx context.Context, a *db.AdminUser) context.Context {
	return context.WithValue(ctx, adminKey, a)
}

func AdminFromContext(ctx context.Context) (*db.AdminUser, bool) {
	a, ok := ctx.Value(adminKey).(*db.AdminUser)
	return a, ok && a != nil
}

// AuthMiddleware injects the admin into the context if a valid Bearer token is present.
// Protected resolvers call requireAdmin() which returns an error if no admin is set.
// The adminLogin mutation doesn't call requireAdmin, so it works without a token.
func AuthMiddleware(svc *service.AdminService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				if admin, err := svc.ValidateToken(r.Context(), strings.TrimPrefix(header, "Bearer ")); err == nil && admin != nil {
					r = r.WithContext(WithAdmin(r.Context(), admin))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Root resolver ────────────────────────────────────────────────────────────

type Resolver struct {
	svc *service.AdminService
}

func requireAdmin(ctx context.Context) (*db.AdminUser, error) {
	a, ok := AdminFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}
	return a, nil
}

// ─── Admin user resolvers ─────────────────────────────────────────────────────

type adminUserResolver struct{ a *db.AdminUser }

func (r *adminUserResolver) ID() graphql.ID       { return graphql.ID(r.a.ID.String()) }
func (r *adminUserResolver) Username() string      { return r.a.Username }
func (r *adminUserResolver) Email() string         { return r.a.Email }
func (r *adminUserResolver) Role() string          { return r.a.Role }
func (r *adminUserResolver) IsActive() bool        { return r.a.IsActive }
func (r *adminUserResolver) CreatedAt() string     { return r.a.CreatedAt.Format(time.RFC3339) }
func (r *adminUserResolver) LastLoginAt() *string {
	if r.a.LastLoginAt == nil {
		return nil
	}
	s := r.a.LastLoginAt.Format(time.RFC3339)
	return &s
}

// ─── Queries ──────────────────────────────────────────────────────────────────

func (r *Resolver) Me(ctx context.Context) (*adminUserResolver, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	return &adminUserResolver{a}, nil
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

type dashboardResolver struct{ s service.DashboardStats }

func (r *dashboardResolver) OpenReports() int32  { return int32(r.s.OpenReports) }
func (r *dashboardResolver) NewUsers24h() int32  { return int32(r.s.NewUsers24h) }
func (r *dashboardResolver) NewUsers7d() int32   { return int32(r.s.NewUsers7d) }
func (r *dashboardResolver) NewPosts24h() int32  { return int32(r.s.NewPosts24h) }
func (r *dashboardResolver) NewPosts7d() int32   { return int32(r.s.NewPosts7d) }
func (r *dashboardResolver) TotalUsers() int32   { return int32(r.s.TotalUsers) }
func (r *dashboardResolver) TotalPosts() int32   { return int32(r.s.TotalPosts) }

func (r *Resolver) DashboardStats(ctx context.Context) (*dashboardResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	stats, err := r.svc.GetDashboardStats(ctx)
	if err != nil {
		return nil, err
	}
	return &dashboardResolver{stats}, nil
}

// ─── Reports ──────────────────────────────────────────────────────────────────

type reportItemResolver struct{ r db.Report }

func (r *reportItemResolver) ID() graphql.ID          { return graphql.ID(r.r.ID.String()) }
func (r *reportItemResolver) Reason() string           { return r.r.Reason }
func (r *reportItemResolver) Note() *string            { return r.r.Note }
func (r *reportItemResolver) ReporterID() graphql.ID   { return graphql.ID(r.r.ReporterID.String()) }
func (r *reportItemResolver) ReporterUsername() string { return r.r.ReporterUsername }
func (r *reportItemResolver) CreatedAt() string        { return r.r.CreatedAt.Format(time.RFC3339) }

type reportGroupResolver struct{ g db.ReportGroup }

func (r *reportGroupResolver) PostID() graphql.ID      { return graphql.ID(r.g.PostID.String()) }
func (r *reportGroupResolver) PostContent() string     { return r.g.PostContent }
func (r *reportGroupResolver) PostDeleted() bool       { return r.g.PostDeleted }
func (r *reportGroupResolver) AuthorID() graphql.ID    { return graphql.ID(r.g.AuthorID.String()) }
func (r *reportGroupResolver) AuthorUsername() string  { return r.g.AuthorUsername }
func (r *reportGroupResolver) ReportCount() int32      { return int32(r.g.ReportCount) }
func (r *reportGroupResolver) OldestReport() string    { return r.g.OldestReport.Format(time.RFC3339) }
func (r *reportGroupResolver) Reports() []*reportItemResolver {
	out := make([]*reportItemResolver, len(r.g.Reports))
	for i, rep := range r.g.Reports {
		out[i] = &reportItemResolver{rep}
	}
	return out
}

type reportGroupConnResolver struct {
	items  []db.ReportGroup
	limit  int
}

func (r *reportGroupConnResolver) Items() []*reportGroupResolver {
	out := make([]*reportGroupResolver, len(r.items))
	for i, g := range r.items {
		out[i] = &reportGroupResolver{g}
	}
	return out
}
func (r *reportGroupConnResolver) NextCursor() *string {
	if len(r.items) < r.limit {
		return nil
	}
	t := r.items[len(r.items)-1].OldestReport.Format(time.RFC3339Nano)
	return &t
}
func (r *reportGroupConnResolver) HasMore() bool { return len(r.items) == r.limit }

type reportsArgs struct {
	After *string
	Limit *int32
}

func (r *Resolver) ReportQueue(ctx context.Context, args reportsArgs) (*reportGroupConnResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var after *time.Time
	if args.After != nil {
		t, err := time.Parse(time.RFC3339Nano, *args.After)
		if err == nil {
			after = &t
		}
	}
	groups, err := r.svc.ListReportGroups(ctx, false, after, limit)
	if err != nil {
		return nil, err
	}
	return &reportGroupConnResolver{items: groups, limit: limit}, nil
}

func (r *Resolver) ResolvedReports(ctx context.Context, args reportsArgs) (*reportGroupConnResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var after *time.Time
	if args.After != nil {
		t, _ := time.Parse(time.RFC3339Nano, *args.After)
		after = &t
	}
	groups, err := r.svc.ListReportGroups(ctx, true, after, limit)
	if err != nil {
		return nil, err
	}
	return &reportGroupConnResolver{items: groups, limit: limit}, nil
}

// ─── Users ────────────────────────────────────────────────────────────────────

type adminUserSummaryResolver struct {
	u           db.User
	postCount   int
	reportCount int
}

func (r *adminUserSummaryResolver) ID() graphql.ID        { return graphql.ID(r.u.ID.String()) }
func (r *adminUserSummaryResolver) Username() string       { return r.u.Username }
func (r *adminUserSummaryResolver) DisplayName() *string   { return r.u.DisplayName }
func (r *adminUserSummaryResolver) Email() *string         { return r.u.Email }
func (r *adminUserSummaryResolver) TrustLevel() int32      { return int32(r.u.TrustLevel) }
func (r *adminUserSummaryResolver) IsSuspended() bool      { return r.u.IsSuspended }
func (r *adminUserSummaryResolver) CreatedAt() string      { return r.u.CreatedAt.Format(time.RFC3339) }
func (r *adminUserSummaryResolver) PostCount() int32       { return int32(r.postCount) }
func (r *adminUserSummaryResolver) ReportCount() int32     { return int32(r.reportCount) }

type userConnResolver struct {
	items []db.User
	limit int
}

func (r *userConnResolver) Items() []*adminUserSummaryResolver {
	out := make([]*adminUserSummaryResolver, len(r.items))
	for i, u := range r.items {
		out[i] = &adminUserSummaryResolver{u: u}
	}
	return out
}
func (r *userConnResolver) NextCursor() *string {
	if len(r.items) < r.limit {
		return nil
	}
	t := r.items[len(r.items)-1].CreatedAt.Format(time.RFC3339Nano)
	return &t
}
func (r *userConnResolver) HasMore() bool { return len(r.items) == r.limit }

type usersArgs struct {
	Search     *string
	TrustLevel *int32
	Suspended  *bool
	After      *string
	Limit      *int32
}

func (r *Resolver) Users(ctx context.Context, args usersArgs) (*userConnResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var after *time.Time
	if args.After != nil {
		t, _ := time.Parse(time.RFC3339Nano, *args.After)
		after = &t
	}
	var tl *int16
	if args.TrustLevel != nil {
		v := int16(*args.TrustLevel)
		tl = &v
	}
	users, err := r.svc.ListUsers(ctx, args.Search, tl, args.Suspended, after, limit)
	if err != nil {
		return nil, err
	}
	return &userConnResolver{items: users, limit: limit}, nil
}

type userIDArg struct{ ID graphql.ID }

func (r *Resolver) User(ctx context.Context, args userIDArg) (*adminUserSummaryResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(string(args.ID))
	if err != nil {
		return nil, fmt.Errorf("invalid id")
	}
	detail, err := r.svc.GetUser(ctx, id)
	if err != nil || detail == nil {
		return nil, err
	}
	return &adminUserSummaryResolver{u: detail.User, postCount: detail.PostCount, reportCount: detail.ReportCount}, nil
}

type userPostsArgs struct {
	UserID graphql.ID
	After  *string
	Limit  *int32
}

func (r *Resolver) Posts(ctx context.Context, args struct {
	AuthorID *graphql.ID
	After    *string
	Limit    *int32
}) (*postConnResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit := 20
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var after *time.Time
	if args.After != nil {
		t, _ := time.Parse(time.RFC3339Nano, *args.After)
		after = &t
	}
	var authorID *uuid.UUID
	if args.AuthorID != nil {
		id, err := uuid.Parse(string(*args.AuthorID))
		if err != nil {
			return nil, fmt.Errorf("invalid authorId")
		}
		authorID = &id
	}
	posts, err := r.svc.ListPosts(ctx, authorID, after, limit)
	if err != nil {
		return nil, err
	}
	return &postConnResolver{items: posts, limit: limit}, nil
}

// ─── Posts ────────────────────────────────────────────────────────────────────

type adminPostResolver struct{ p service.PostWithAuthor }

func (r *adminPostResolver) ID() graphql.ID         { return graphql.ID(r.p.ID.String()) }
func (r *adminPostResolver) Content() string         { return r.p.Content }
func (r *adminPostResolver) AuthorID() graphql.ID    { return graphql.ID(r.p.AuthorID.String()) }
func (r *adminPostResolver) AuthorUsername() string  { return r.p.AuthorUsername }
func (r *adminPostResolver) ReplyCount() int32       { return int32(r.p.ReplyCount) }
func (r *adminPostResolver) LikeCount() int32        { return int32(r.p.LikeCount) }
func (r *adminPostResolver) OpenReports() int32      { return int32(r.p.OpenReports) }
func (r *adminPostResolver) CreatedAt() string       { return r.p.CreatedAt.Format(time.RFC3339) }
func (r *adminPostResolver) DeletedAt() *string {
	if r.p.DeletedAt == nil {
		return nil
	}
	s := r.p.DeletedAt.Format(time.RFC3339)
	return &s
}

type postConnResolver struct {
	items []service.PostWithAuthor
	limit int
}

func (r *postConnResolver) Items() []*adminPostResolver {
	out := make([]*adminPostResolver, len(r.items))
	for i, p := range r.items {
		out[i] = &adminPostResolver{p}
	}
	return out
}
func (r *postConnResolver) NextCursor() *string {
	if len(r.items) < r.limit {
		return nil
	}
	t := r.items[len(r.items)-1].CreatedAt.Format(time.RFC3339Nano)
	return &t
}
func (r *postConnResolver) HasMore() bool { return len(r.items) == r.limit }

// ─── Audit log ────────────────────────────────────────────────────────────────

type auditEntryResolver struct{ e db.AuditEntry }

func (r *auditEntryResolver) ID() graphql.ID          { return graphql.ID(r.e.ID.String()) }
func (r *auditEntryResolver) AdminID() graphql.ID      { return graphql.ID(r.e.AdminID.String()) }
func (r *auditEntryResolver) AdminUsername() string    { return r.e.AdminUsername }
func (r *auditEntryResolver) Action() string           { return r.e.Action }
func (r *auditEntryResolver) TargetType() string       { return r.e.TargetType }
func (r *auditEntryResolver) TargetID() graphql.ID     { return graphql.ID(r.e.TargetID.String()) }
func (r *auditEntryResolver) Note() *string            { return r.e.Note }
func (r *auditEntryResolver) Metadata() *string        { return r.e.Metadata }
func (r *auditEntryResolver) CreatedAt() string        { return r.e.CreatedAt.Format(time.RFC3339) }

type auditConnResolver struct {
	items []db.AuditEntry
	limit int
}

func (r *auditConnResolver) Items() []*auditEntryResolver {
	out := make([]*auditEntryResolver, len(r.items))
	for i, e := range r.items {
		out[i] = &auditEntryResolver{e}
	}
	return out
}
func (r *auditConnResolver) NextCursor() *string {
	if len(r.items) < r.limit {
		return nil
	}
	t := r.items[len(r.items)-1].CreatedAt.Format(time.RFC3339Nano)
	return &t
}
func (r *auditConnResolver) HasMore() bool { return len(r.items) == r.limit }

type auditArgs struct {
	AdminID    *graphql.ID
	TargetType *string
	TargetID   *graphql.ID
	After      *string
	Limit      *int32
}

func (r *Resolver) AuditLog(ctx context.Context, args auditArgs) (*auditConnResolver, error) {
	if a, err := requireAdmin(ctx); err != nil || a.Role == "moderator" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("forbidden: requires admin role")
	}
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var after *time.Time
	if args.After != nil {
		t, _ := time.Parse(time.RFC3339Nano, *args.After)
		after = &t
	}
	var adminID, targetID *uuid.UUID
	if args.AdminID != nil {
		id, _ := uuid.Parse(string(*args.AdminID))
		adminID = &id
	}
	if args.TargetID != nil {
		id, _ := uuid.Parse(string(*args.TargetID))
		targetID = &id
	}
	entries, err := r.svc.ListAuditLog(ctx, adminID, args.TargetType, targetID, after, limit)
	if err != nil {
		return nil, err
	}
	return &auditConnResolver{items: entries, limit: limit}, nil
}

// ─── Mutations ────────────────────────────────────────────────────────────────

type loginArgs struct {
	Username string
	Password string
}

func (r *Resolver) AdminLogin(ctx context.Context, args loginArgs) (*struct {
	Token string
	Admin *adminUserResolver
}, error) {
	result, err := r.svc.Login(ctx, args.Username, args.Password)
	if err != nil {
		log.Warn().Str("username", args.Username).Msg("admin login failed")
		return nil, fmt.Errorf("invalid credentials")
	}
	return &struct {
		Token string
		Admin *adminUserResolver
	}{Token: result.Token, Admin: &adminUserResolver{result.Admin}}, nil
}

type postActionArgs struct {
	PostID graphql.ID
	Note   *string
}

func (r *Resolver) ResolveReports(ctx context.Context, args postActionArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	postID, err := uuid.Parse(string(args.PostID))
	if err != nil {
		return false, fmt.Errorf("invalid postId")
	}
	return true, r.svc.ResolveReports(ctx, a.ID, postID, args.Note)
}

func (r *Resolver) DismissReports(ctx context.Context, args postActionArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	postID, err := uuid.Parse(string(args.PostID))
	if err != nil {
		return false, fmt.Errorf("invalid postId")
	}
	return true, r.svc.DismissReports(ctx, a.ID, postID, args.Note)
}

func (r *Resolver) DeletePost(ctx context.Context, args postActionArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	postID, err := uuid.Parse(string(args.PostID))
	if err != nil {
		return false, fmt.Errorf("invalid postId")
	}
	return true, r.svc.DeletePost(ctx, a.ID, postID, args.Note)
}

type trustArgs struct {
	UserID     graphql.ID
	TrustLevel int32
	Note       *string
}

func (r *Resolver) SetTrustLevel(ctx context.Context, args trustArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	if a.Role == "moderator" {
		return false, fmt.Errorf("forbidden: requires admin role")
	}
	userID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return false, fmt.Errorf("invalid userId")
	}
	return true, r.svc.SetTrustLevel(ctx, a.ID, userID, int16(args.TrustLevel), args.Note)
}

type userActionArgs struct {
	UserID graphql.ID
	Note   *string
}

func (r *Resolver) SuspendUser(ctx context.Context, args userActionArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	if a.Role == "moderator" {
		return false, fmt.Errorf("forbidden: requires admin role")
	}
	userID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return false, fmt.Errorf("invalid userId")
	}
	return true, r.svc.SuspendUser(ctx, a.ID, userID, args.Note)
}

func (r *Resolver) UnsuspendUser(ctx context.Context, args userActionArgs) (bool, error) {
	a, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	if a.Role == "moderator" {
		return false, fmt.Errorf("forbidden: requires admin role")
	}
	userID, err := uuid.Parse(string(args.UserID))
	if err != nil {
		return false, fmt.Errorf("invalid userId")
	}
	return true, r.svc.UnsuspendUser(ctx, a.ID, userID, args.Note)
}

// ensure log is used (for the warn in AdminLogin)
var _ = log.Logger
