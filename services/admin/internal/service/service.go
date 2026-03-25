package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/aleth/admin/internal/db"
)

type adminLoginAttempts struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var loginAttempts = &adminLoginAttempts{
	attempts: make(map[string][]time.Time),
}

func (a *adminLoginAttempts) isBlocked(username string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	window := time.Now().Add(-15 * time.Minute)
	// Remove old attempts.
	var recent []time.Time
	for _, t := range a.attempts[username] {
		if t.After(window) {
			recent = append(recent, t)
		}
	}
	a.attempts[username] = recent
	return len(recent) >= 10
}

func (a *adminLoginAttempts) record(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attempts[username] = append(a.attempts[username], time.Now())
}

func (a *adminLoginAttempts) reset(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.attempts, username)
}

// AdminService handles all admin business logic.
// It reads from both the auth DB and content DB with capped pools,
// and writes directly to both DBs for admin-specific mutations.
type AdminService struct {
	authDB    *db.AuthPool
	contentDB *db.ContentPool
	jwtSecret []byte
}

func New(authDB *db.AuthPool, contentDB *db.ContentPool, jwtSecret string) *AdminService {
	return &AdminService{
		authDB:    authDB,
		contentDB: contentDB,
		jwtSecret: []byte(jwtSecret),
	}
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

type LoginResult struct {
	Token string
	Admin *db.AdminUser
}

func (s *AdminService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	if loginAttempts.isBlocked(username) {
		return nil, fmt.Errorf("too many failed login attempts — try again in 15 minutes")
	}

	admin, err := s.authDB.GetAdminByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if admin == nil {
		loginAttempts.record(username)
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword(admin.PasswordHash, []byte(password)); err != nil {
		loginAttempts.record(username)
		return nil, fmt.Errorf("invalid credentials")
	}
	loginAttempts.reset(username)
	_ = s.authDB.TouchAdminLogin(ctx, admin.ID)

	token, err := s.issueToken(admin)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return &LoginResult{Token: token, Admin: admin}, nil
}

func (s *AdminService) issueToken(admin *db.AdminUser) (string, error) {
	claims := jwt.MapClaims{
		"sub":  admin.ID.String(),
		"role": admin.Role,
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

// ValidateToken parses and validates an admin JWT, returning the admin user.
func (s *AdminService) ValidateToken(ctx context.Context, tokenStr string) (*db.AdminUser, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return nil, fmt.Errorf("invalid sub")
	}
	admin, err := s.authDB.GetAdminByID(ctx, id)
	if err != nil || admin == nil {
		return nil, fmt.Errorf("admin not found")
	}
	return admin, nil
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

type DashboardStats struct {
	OpenReports int
	NewUsers24h int
	NewUsers7d  int
	NewPosts24h int
	NewPosts7d  int
	TotalUsers  int
	TotalPosts  int
}

func (s *AdminService) GetDashboardStats(ctx context.Context) (DashboardStats, error) {
	now := time.Now()
	var stats DashboardStats
	var err error

	stats.OpenReports, err = s.contentDB.CountOpenReports(ctx)
	if err != nil {
		return stats, err
	}
	stats.NewUsers24h, err = s.authDB.CountNewUsers(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return stats, err
	}
	stats.NewUsers7d, err = s.authDB.CountNewUsers(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return stats, err
	}
	stats.NewPosts24h, err = s.contentDB.CountNewPosts(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return stats, err
	}
	stats.NewPosts7d, err = s.contentDB.CountNewPosts(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return stats, err
	}
	stats.TotalUsers, err = s.authDB.CountTotalUsers(ctx)
	if err != nil {
		return stats, err
	}
	stats.TotalPosts, err = s.contentDB.CountTotalPosts(ctx)
	if err != nil {
		return stats, err
	}
	return stats, nil
}

// ─── Reports ──────────────────────────────────────────────────────────────────

func (s *AdminService) ListReportGroups(ctx context.Context, resolved bool, after *time.Time, limit int) ([]db.ReportGroup, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	groups, err := s.contentDB.ListReportGroups(ctx, resolved, after, limit)
	if err != nil {
		return nil, err
	}
	// Enrich with reporter usernames (batch fetch from auth DB).
	reporterIDs := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]bool)
	for i := range groups {
		for _, r := range groups[i].Reports {
			if !seen[r.ReporterID] {
				reporterIDs = append(reporterIDs, r.ReporterID)
				seen[r.ReporterID] = true
			}
		}
	}
	// Also collect author IDs.
	authorIDs := make([]uuid.UUID, 0, len(groups))
	seenAuthors := make(map[uuid.UUID]bool)
	for _, g := range groups {
		if !seenAuthors[g.AuthorID] {
			authorIDs = append(authorIDs, g.AuthorID)
			seenAuthors[g.AuthorID] = true
		}
	}
	allIDs := append(reporterIDs, authorIDs...)
	users, err := s.authDB.GetUsersByIDs(ctx, allIDs)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if u, ok := users[groups[i].AuthorID]; ok {
			groups[i].AuthorUsername = u.Username
		}
		for j := range groups[i].Reports {
			if u, ok := users[groups[i].Reports[j].ReporterID]; ok {
				groups[i].Reports[j].ReporterUsername = u.Username
			}
		}
	}
	return groups, nil
}

func (s *AdminService) ResolveReports(ctx context.Context, adminID, postID uuid.UUID, note *string) error {
	if err := s.contentDB.ResolveReportsByPost(ctx, postID, adminID); err != nil {
		return err
	}
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "resolve_report",
		TargetType: "post", TargetID: postID, Note: note,
	})
}

func (s *AdminService) DismissReports(ctx context.Context, adminID, postID uuid.UUID, note *string) error {
	// Dismiss = resolve without deleting the post.
	if err := s.contentDB.ResolveReportsByPost(ctx, postID, adminID); err != nil {
		return err
	}
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "dismiss_report",
		TargetType: "post", TargetID: postID, Note: note,
	})
}

// ─── Posts ────────────────────────────────────────────────────────────────────

type PostWithAuthor struct {
	db.Post
	AuthorUsername string
}

func (s *AdminService) ListPosts(ctx context.Context, authorID *uuid.UUID, after *time.Time, limit int) ([]PostWithAuthor, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	posts, err := s.contentDB.ListPosts(ctx, authorID, after, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.AuthorID)
	}
	users, err := s.authDB.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]PostWithAuthor, len(posts))
	for i, p := range posts {
		result[i] = PostWithAuthor{Post: p}
		if u, ok := users[p.AuthorID]; ok {
			result[i].AuthorUsername = u.Username
		}
	}
	return result, nil
}

func (s *AdminService) DeletePost(ctx context.Context, adminID, postID uuid.UUID, note *string) error {
	if err := s.contentDB.SoftDeletePost(ctx, postID); err != nil {
		return err
	}
	// Auto-resolve any open reports for this post.
	_ = s.contentDB.ResolveReportsByPost(ctx, postID, adminID)
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "delete_post",
		TargetType: "post", TargetID: postID, Note: note,
	})
}

// ─── Users ────────────────────────────────────────────────────────────────────

type UserDetail struct {
	db.User
	PostCount   int
	ReportCount int
}

func (s *AdminService) ListUsers(ctx context.Context, search *string, trustLevel *int16, suspended *bool, after *time.Time, limit int) ([]db.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.authDB.ListUsers(ctx, search, trustLevel, suspended, after, limit)
}

func (s *AdminService) GetUser(ctx context.Context, id uuid.UUID) (*UserDetail, error) {
	u, err := s.authDB.GetUserByID(ctx, id)
	if err != nil || u == nil {
		return nil, err
	}
	detail := &UserDetail{User: *u}
	detail.PostCount, _ = s.contentDB.CountUserPosts(ctx, id)
	detail.ReportCount, _ = s.contentDB.CountUserReports(ctx, id)
	return detail, nil
}

func (s *AdminService) SetTrustLevel(ctx context.Context, adminID, userID uuid.UUID, level int16, note *string) error {
	if level < 0 || level > 5 {
		return fmt.Errorf("trust level must be 0–5")
	}
	if err := s.authDB.SetTrustLevel(ctx, userID, level); err != nil {
		return err
	}
	meta := fmt.Sprintf(`{"after":%d}`, level)
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "set_trust_level",
		TargetType: "user", TargetID: userID, Note: note, Metadata: &meta,
	})
}

func (s *AdminService) SuspendUser(ctx context.Context, adminID, userID uuid.UUID, note *string) error {
	if err := s.authDB.SetSuspended(ctx, userID, true); err != nil {
		return err
	}
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "suspend_user",
		TargetType: "user", TargetID: userID, Note: note,
	})
}

func (s *AdminService) UnsuspendUser(ctx context.Context, adminID, userID uuid.UUID, note *string) error {
	if err := s.authDB.SetSuspended(ctx, userID, false); err != nil {
		return err
	}
	return s.authDB.CreateAuditEntry(ctx, db.CreateAuditParams{
		AdminID: adminID, Action: "unsuspend_user",
		TargetType: "user", TargetID: userID, Note: note,
	})
}

// ─── Audit log ────────────────────────────────────────────────────────────────

func (s *AdminService) ListAuditLog(ctx context.Context, adminID *uuid.UUID, targetType *string, targetID *uuid.UUID, after *time.Time, limit int) ([]db.AuditEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.authDB.ListAuditLog(ctx, adminID, targetType, targetID, after, limit)
}
