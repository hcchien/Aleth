package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SQLStore struct {
	db         *sql.DB
	driverName string
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{
		db:         db,
		driverName: "sqlite3",
	}
}

func NewSQLStoreWithDriver(db *sql.DB, driverName string) *SQLStore {
	return &SQLStore{
		db:         db,
		driverName: driverName,
	}
}

func OpenSQLStore(driverName, dsn string) (*SQLStore, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sql store: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sql store: %w", err)
	}
	store := NewSQLStoreWithDriver(db, driverName)
	if err := store.InitSchema(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLStore) InitSchema() error {
	return s.runMigrations()
}

func (s *SQLStore) CreateUser(user *User) error {
	if user == nil || user.DID == "" {
		return fmt.Errorf("invalid user")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	ph := s.placeholder
	insertUser := ""
	if s.isPostgres() {
		insertUser = fmt.Sprintf(`
INSERT INTO users (did, oauth_id, public_key, authn_user_id, passkey_credentials, trust_tier, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s)
ON CONFLICT (did) DO UPDATE SET
  oauth_id = EXCLUDED.oauth_id,
  public_key = EXCLUDED.public_key,
  authn_user_id = EXCLUDED.authn_user_id,
  passkey_credentials = EXCLUDED.passkey_credentials,
  trust_tier = EXCLUDED.trust_tier`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7))
	} else {
		insertUser = fmt.Sprintf(`
INSERT INTO users (did, oauth_id, public_key, authn_user_id, passkey_credentials, trust_tier, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s)
ON CONFLICT(did) DO UPDATE SET
  oauth_id = excluded.oauth_id,
  public_key = excluded.public_key,
  authn_user_id = excluded.authn_user_id,
  passkey_credentials = excluded.passkey_credentials,
  trust_tier = excluded.trust_tier`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7))
	}

	if _, err := s.db.Exec(insertUser, user.DID, user.OAuthID, user.PublicKey, user.AuthnUserID, user.PasskeyCredentials, int(user.TrustTier), user.CreatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	getUser := fmt.Sprintf(`
SELECT id, did, oauth_id, public_key, authn_user_id, passkey_credentials, trust_tier, created_at
FROM users
WHERE did = %s`, ph(1))
	var trustTier int
	if err := s.db.QueryRow(getUser, user.DID).Scan(
		&user.ID,
		&user.DID,
		&user.OAuthID,
		&user.PublicKey,
		&user.AuthnUserID,
		&user.PasskeyCredentials,
		&trustTier,
		&user.CreatedAt,
	); err != nil {
		return fmt.Errorf("load user after create: %w", err)
	}
	user.TrustTier = TrustTier(trustTier)
	return nil
}

func (s *SQLStore) GetUserByDID(did string) (*User, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, did, oauth_id, public_key, authn_user_id, passkey_credentials, trust_tier, created_at
FROM users
WHERE did = %s`, ph(1))
	var (
		u         User
		trustTier int
	)
	err := s.db.QueryRow(q, did).Scan(
		&u.ID,
		&u.DID,
		&u.OAuthID,
		&u.PublicKey,
		&u.AuthnUserID,
		&u.PasskeyCredentials,
		&trustTier,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by did: %w", err)
	}
	u.TrustTier = TrustTier(trustTier)
	return &u, nil
}

func (s *SQLStore) CreatePost(post *Post) error {
	if post == nil {
		return fmt.Errorf("invalid post")
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}

	mediaHashes, err := json.Marshal(post.MediaHashes)
	if err != nil {
		return fmt.Errorf("marshal media hashes: %w", err)
	}

	ph := s.placeholder
	if s.isPostgres() {
		q := fmt.Sprintf(`
INSERT INTO posts (body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
		if err := s.db.QueryRow(
			q,
			post.Body,
			string(mediaHashes),
			nullInt64Value(post.ParentID),
			post.Timestamp,
			post.AuthorDID,
			post.Signature,
			post.VisibilityScore,
			post.CreatedAt,
		).Scan(&post.ID); err != nil {
			return fmt.Errorf("create post: %w", err)
		}
		return nil
	}

	q := fmt.Sprintf(`
INSERT INTO posts (body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
	res, err := s.db.Exec(
		q,
		post.Body,
		string(mediaHashes),
		nullInt64Value(post.ParentID),
		post.Timestamp,
		post.AuthorDID,
		post.Signature,
		post.VisibilityScore,
		post.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("fetch inserted post id: %w", err)
	}
	post.ID = id
	return nil
}

func (s *SQLStore) GetPosts(limit, offset int) ([]Post, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, body, media_hashes, parent_id, timestamp, author_did, signature, visibility_score, created_at
FROM posts
ORDER BY visibility_score DESC, created_at DESC
LIMIT %s OFFSET %s`, ph(1), ph(2))
	rows, err := s.db.Query(q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var (
			p             Post
			mediaHashesJS string
			parentID      sql.NullInt64
		)
		if err := rows.Scan(
			&p.ID,
			&p.Body,
			&mediaHashesJS,
			&parentID,
			&p.Timestamp,
			&p.AuthorDID,
			&p.Signature,
			&p.VisibilityScore,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		if err := json.Unmarshal([]byte(mediaHashesJS), &p.MediaHashes); err != nil {
			return nil, fmt.Errorf("decode media hashes: %w", err)
		}
		p.ParentID = parentID
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return posts, nil
}

func (s *SQLStore) UpdateVisibilityScore(postID int64, score float64) error {
	ph := s.placeholder
	q := fmt.Sprintf(`
UPDATE posts
SET visibility_score = visibility_score + %s
WHERE id = %s`, ph(1), ph(2))
	res, err := s.db.Exec(q, score, postID)
	if err != nil {
		return fmt.Errorf("update visibility score: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

func (s *SQLStore) CheckRateLimit(_ string, _ TrustTier) (bool, error) {
	return true, nil
}

func (s *SQLStore) CreateContentItem(item *ContentItem) error {
	if item == nil || item.ID == "" || item.AuthorDID == "" || item.Body == "" {
		return fmt.Errorf("invalid content item")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}

	sourceIDs, err := json.Marshal(item.SourceContentIDs)
	if err != nil {
		return fmt.Errorf("marshal source content ids: %w", err)
	}

	ph := s.placeholder
	q := fmt.Sprintf(`
INSERT INTO content_items (
  id, author_did, title, body, mode, status, visibility, trust_tier,
  participation_policy, discussion_shape, source_content_ids, published_at, created_at, updated_at
) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12), ph(13), ph(14))
	_, err = s.db.Exec(q,
		item.ID,
		item.AuthorDID,
		item.Title,
		item.Body,
		string(item.Mode),
		string(item.Status),
		string(item.Visibility),
		int(item.TrustTier),
		string(item.ParticipationPolicy),
		string(item.DiscussionShape),
		string(sourceIDs),
		item.PublishedAt,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create content item: %w", err)
	}
	return nil
}

func (s *SQLStore) GetContentItemByID(id string) (*ContentItem, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, author_did, title, body, mode, status, visibility, trust_tier,
       participation_policy, discussion_shape, source_content_ids, published_at, created_at, updated_at
FROM content_items
WHERE id = %s`, ph(1))
	var (
		item         ContentItem
		trustTier    int
		sourceIDsRaw string
		publishedAt  sql.NullTime
	)
	err := s.db.QueryRow(q, id).Scan(
		&item.ID,
		&item.AuthorDID,
		&item.Title,
		&item.Body,
		&item.Mode,
		&item.Status,
		&item.Visibility,
		&trustTier,
		&item.ParticipationPolicy,
		&item.DiscussionShape,
		&sourceIDsRaw,
		&publishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get content item by id: %w", err)
	}
	if sourceIDsRaw != "" {
		if err := json.Unmarshal([]byte(sourceIDsRaw), &item.SourceContentIDs); err != nil {
			return nil, fmt.Errorf("decode source content ids: %w", err)
		}
	}
	item.TrustTier = TrustTier(trustTier)
	if publishedAt.Valid {
		t := publishedAt.Time
		item.PublishedAt = &t
	}
	return &item, nil
}

func (s *SQLStore) GetContentItems(mode *ContentMode, visibility *ContentVisibility, authorDID *string, limit, offset int) ([]ContentItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	ph := s.placeholder
	args := []any{}
	conds := []string{}
	idx := 1
	if mode != nil {
		conds = append(conds, fmt.Sprintf("mode = %s", ph(idx)))
		args = append(args, string(*mode))
		idx++
	}
	if visibility != nil {
		conds = append(conds, fmt.Sprintf("visibility = %s", ph(idx)))
		args = append(args, string(*visibility))
		idx++
	}
	if authorDID != nil {
		conds = append(conds, fmt.Sprintf("author_did = %s", ph(idx)))
		args = append(args, *authorDID)
		idx++
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	q := fmt.Sprintf(`
SELECT id, author_did, title, body, mode, status, visibility, trust_tier,
       participation_policy, discussion_shape, source_content_ids, published_at, created_at, updated_at
FROM content_items
%s
ORDER BY created_at DESC
LIMIT %s OFFSET %s`, where, ph(idx), ph(idx+1))
	args = append(args, limit, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get content items: %w", err)
	}
	defer rows.Close()

	var items []ContentItem
	for rows.Next() {
		var (
			item         ContentItem
			trustTier    int
			sourceIDsRaw string
			publishedAt  sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&item.AuthorDID,
			&item.Title,
			&item.Body,
			&item.Mode,
			&item.Status,
			&item.Visibility,
			&trustTier,
			&item.ParticipationPolicy,
			&item.DiscussionShape,
			&sourceIDsRaw,
			&publishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan content item: %w", err)
		}
		if sourceIDsRaw != "" {
			if err := json.Unmarshal([]byte(sourceIDsRaw), &item.SourceContentIDs); err != nil {
				return nil, fmt.Errorf("decode source content ids: %w", err)
			}
		}
		item.TrustTier = TrustTier(trustTier)
		if publishedAt.Valid {
			t := publishedAt.Time
			item.PublishedAt = &t
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content items: %w", err)
	}
	return items, nil
}

func (s *SQLStore) CreateDiscussionNode(node *DiscussionNode) error {
	if node == nil || node.ID == "" || node.DiscussionID == "" || node.AuthorDID == "" || node.Body == "" {
		return fmt.Errorf("invalid discussion node")
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}
	ph := s.placeholder
	q := fmt.Sprintf(`
INSERT INTO discussion_nodes (id, discussion_id, parent_node_id, author_did, node_type, stance, body, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
	_, err := s.db.Exec(q, node.ID, node.DiscussionID, node.ParentNodeID, node.AuthorDID, string(node.NodeType), string(node.Stance), node.Body, node.CreatedAt)
	if err != nil {
		return fmt.Errorf("create discussion node: %w", err)
	}
	return nil
}

func (s *SQLStore) GetDiscussionNodes(discussionID string) ([]DiscussionNode, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, discussion_id, parent_node_id, author_did, node_type, stance, body, created_at
FROM discussion_nodes
WHERE discussion_id = %s
ORDER BY created_at ASC`, ph(1))
	rows, err := s.db.Query(q, discussionID)
	if err != nil {
		return nil, fmt.Errorf("get discussion nodes: %w", err)
	}
	defer rows.Close()

	var nodes []DiscussionNode
	for rows.Next() {
		var (
			node         DiscussionNode
			parentNodeID sql.NullString
		)
		if err := rows.Scan(&node.ID, &node.DiscussionID, &parentNodeID, &node.AuthorDID, &node.NodeType, &node.Stance, &node.Body, &node.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan discussion node: %w", err)
		}
		if parentNodeID.Valid {
			node.ParentNodeID = &parentNodeID.String
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discussion nodes: %w", err)
	}
	return nodes, nil
}

func (s *SQLStore) CreateProjection(projection *Projection) error {
	if projection == nil || projection.ID == "" {
		return fmt.Errorf("invalid projection")
	}
	ph := s.placeholder
	q := fmt.Sprintf(`
INSERT INTO projections (id, source_idea_id, target_discussion_id, projected_excerpt, participation_policy, ownership_transfer_acknowledged, created_by_did, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
	_, err := s.db.Exec(q, projection.ID, projection.SourceIdeaID, projection.TargetDiscussionID, projection.ProjectedExcerpt, string(projection.ParticipationPolicy), projection.OwnershipTransferAcknowledged, projection.CreatedByDID, projection.CreatedAt)
	if err != nil {
		return fmt.Errorf("create projection: %w", err)
	}
	return nil
}

func (s *SQLStore) GetProjectionsBySourceIdeaID(sourceIdeaID string) ([]Projection, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, source_idea_id, target_discussion_id, projected_excerpt, participation_policy, ownership_transfer_acknowledged, created_by_did, created_at
FROM projections
WHERE source_idea_id = %s
ORDER BY created_at DESC`, ph(1))
	rows, err := s.db.Query(q, sourceIdeaID)
	if err != nil {
		return nil, fmt.Errorf("get projections: %w", err)
	}
	defer rows.Close()
	var projections []Projection
	for rows.Next() {
		var projection Projection
		if err := rows.Scan(&projection.ID, &projection.SourceIdeaID, &projection.TargetDiscussionID, &projection.ProjectedExcerpt, &projection.ParticipationPolicy, &projection.OwnershipTransferAcknowledged, &projection.CreatedByDID, &projection.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan projection: %w", err)
		}
		projections = append(projections, projection)
	}
	return projections, rows.Err()
}

func (s *SQLStore) CreateContentRelation(relation *ContentRelation) error {
	if relation == nil || relation.ID == "" {
		return fmt.Errorf("invalid content relation")
	}
	ph := s.placeholder
	q := fmt.Sprintf(`
INSERT INTO content_relations (id, from_content_id, to_content_id, relation_type, created_at)
VALUES (%s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5))
	_, err := s.db.Exec(q, relation.ID, relation.FromContentID, relation.ToContentID, string(relation.RelationType), relation.CreatedAt)
	if err != nil {
		return fmt.Errorf("create content relation: %w", err)
	}
	return nil
}

func (s *SQLStore) GetContentRelations(contentID string) ([]ContentRelation, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, from_content_id, to_content_id, relation_type, created_at
FROM content_relations
WHERE from_content_id = %s OR to_content_id = %s
ORDER BY created_at DESC`, ph(1), ph(2))
	rows, err := s.db.Query(q, contentID, contentID)
	if err != nil {
		return nil, fmt.Errorf("get content relations: %w", err)
	}
	defer rows.Close()
	var relations []ContentRelation
	for rows.Next() {
		var relation ContentRelation
		if err := rows.Scan(&relation.ID, &relation.FromContentID, &relation.ToContentID, &relation.RelationType, &relation.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan content relation: %w", err)
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

func (s *SQLStore) CreateTransformationJob(job *TransformationJob) error {
	if job == nil || job.ID == "" {
		return fmt.Errorf("invalid transformation job")
	}
	sourceIDs, err := json.Marshal(job.SourceContentIDs)
	if err != nil {
		return fmt.Errorf("marshal source content ids: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`
INSERT INTO transformation_jobs (
  id, requested_by_did, source_content_ids, target_mode, provider_type,
  prompt_profile, status, output_title, output_body, published_content_id, created_at, completed_at
) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12))
	_, err = s.db.Exec(q, job.ID, job.RequestedByDID, string(sourceIDs), string(job.TargetMode), string(job.ProviderType), job.PromptProfile, string(job.Status), job.OutputTitle, job.OutputBody, job.PublishedContentID, job.CreatedAt, job.CompletedAt)
	if err != nil {
		return fmt.Errorf("create transformation job: %w", err)
	}
	return nil
}

func (s *SQLStore) GetTransformationJobByID(id string) (*TransformationJob, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`
SELECT id, requested_by_did, source_content_ids, target_mode, provider_type,
       prompt_profile, status, output_title, output_body, published_content_id, created_at, completed_at
FROM transformation_jobs
WHERE id = %s`, ph(1))
	var (
		job          TransformationJob
		sourceIDsRaw string
		publishedID  sql.NullString
		completedAt  sql.NullTime
	)
	err := s.db.QueryRow(q, id).Scan(&job.ID, &job.RequestedByDID, &sourceIDsRaw, &job.TargetMode, &job.ProviderType, &job.PromptProfile, &job.Status, &job.OutputTitle, &job.OutputBody, &publishedID, &job.CreatedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get transformation job by id: %w", err)
	}
	if sourceIDsRaw != "" {
		if err := json.Unmarshal([]byte(sourceIDsRaw), &job.SourceContentIDs); err != nil {
			return nil, fmt.Errorf("decode source content ids: %w", err)
		}
	}
	if publishedID.Valid {
		job.PublishedContentID = &publishedID.String
	}
	if completedAt.Valid {
		t := completedAt.Time
		job.CompletedAt = &t
	}
	return &job, nil
}

func (s *SQLStore) UpdateTransformationJob(job *TransformationJob) error {
	if job == nil || job.ID == "" {
		return fmt.Errorf("invalid transformation job")
	}
	sourceIDs, err := json.Marshal(job.SourceContentIDs)
	if err != nil {
		return fmt.Errorf("marshal source content ids: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`
UPDATE transformation_jobs
SET requested_by_did = %s, source_content_ids = %s, target_mode = %s, provider_type = %s,
    prompt_profile = %s, status = %s, output_title = %s, output_body = %s, published_content_id = %s,
    created_at = %s, completed_at = %s
WHERE id = %s`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12))
	res, err := s.db.Exec(q, job.RequestedByDID, string(sourceIDs), string(job.TargetMode), string(job.ProviderType), job.PromptProfile, string(job.Status), job.OutputTitle, job.OutputBody, job.PublishedContentID, job.CreatedAt, job.CompletedAt, job.ID)
	if err != nil {
		return fmt.Errorf("update transformation job: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("transformation job not found")
	}
	return nil
}

func (s *SQLStore) CreateDiscussionFork(fork *DiscussionFork) error {
	if fork == nil || fork.ID == "" {
		return fmt.Errorf("invalid discussion fork")
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO discussion_forks (id, source_discussion_id, fork_discussion_id, created_by_did, reason, created_at)
VALUES (%s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6))
	_, err := s.db.Exec(q, fork.ID, fork.SourceDiscussionID, fork.ForkDiscussionID, fork.CreatedByDID, fork.Reason, fork.CreatedAt)
	if err != nil {
		return fmt.Errorf("create discussion fork: %w", err)
	}
	return nil
}

func (s *SQLStore) GetDiscussionForks(sourceDiscussionID string) ([]DiscussionFork, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, source_discussion_id, fork_discussion_id, created_by_did, reason, created_at
FROM discussion_forks WHERE source_discussion_id = %s ORDER BY created_at DESC`, ph(1))
	rows, err := s.db.Query(q, sourceDiscussionID)
	if err != nil {
		return nil, fmt.Errorf("get discussion forks: %w", err)
	}
	defer rows.Close()
	var forks []DiscussionFork
	for rows.Next() {
		var fork DiscussionFork
		if err := rows.Scan(&fork.ID, &fork.SourceDiscussionID, &fork.ForkDiscussionID, &fork.CreatedByDID, &fork.Reason, &fork.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan discussion fork: %w", err)
		}
		forks = append(forks, fork)
	}
	return forks, rows.Err()
}

func (s *SQLStore) CreateModerationAction(action *ModerationAction) error {
	if action == nil || action.ID == "" {
		return fmt.Errorf("invalid moderation action")
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO moderation_actions (id, target_content_id, action_type, reason, initiated_by_did, required_tier, status, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8))
	_, err := s.db.Exec(q, action.ID, action.TargetContentID, string(action.ActionType), action.Reason, action.InitiatedByDID, int(action.RequiredTier), string(action.Status), action.CreatedAt)
	if err != nil {
		return fmt.Errorf("create moderation action: %w", err)
	}
	return nil
}

func (s *SQLStore) GetModerationActions(targetContentID string) ([]ModerationAction, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, target_content_id, action_type, reason, initiated_by_did, required_tier, status, created_at
FROM moderation_actions WHERE target_content_id = %s ORDER BY created_at DESC`, ph(1))
	rows, err := s.db.Query(q, targetContentID)
	if err != nil {
		return nil, fmt.Errorf("get moderation actions: %w", err)
	}
	defer rows.Close()
	var actions []ModerationAction
	for rows.Next() {
		var action ModerationAction
		var requiredTier int
		if err := rows.Scan(&action.ID, &action.TargetContentID, &action.ActionType, &action.Reason, &action.InitiatedByDID, &requiredTier, &action.Status, &action.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan moderation action: %w", err)
		}
		action.RequiredTier = TrustTier(requiredTier)
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (s *SQLStore) CreateCredential(credential *Credential) error {
	if credential == nil || credential.ID == "" || credential.SubjectDID == "" || credential.IssuerDID == "" {
		return fmt.Errorf("invalid credential")
	}
	if credential.Status == "" {
		credential.Status = CredentialActive
	}
	if credential.IssuanceSource == "" {
		credential.IssuanceSource = CredentialInternalVerifierIssued
	}
	if credential.IssuedAt.IsZero() {
		credential.IssuedAt = time.Now().UTC()
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO credentials (id, subject_did, issuer_did, external_issuer_did, credential_type, claims_json, proof, status, issuance_source, issued_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12))
	_, err := s.db.Exec(q, credential.ID, credential.SubjectDID, credential.IssuerDID, nullStringValue(credential.ExternalIssuerDID), credential.CredentialType, credential.ClaimsJSON, credential.Proof, string(credential.Status), string(credential.IssuanceSource), credential.IssuedAt, credential.ExpiresAt, credential.RevokedAt)
	if err != nil {
		return fmt.Errorf("create credential: %w", err)
	}
	return nil
}

func (s *SQLStore) GetCredentialsBySubjectDID(subjectDID string) ([]Credential, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, subject_did, issuer_did, COALESCE(external_issuer_did, ''), credential_type, claims_json, proof, status, issuance_source, issued_at, expires_at, revoked_at
FROM credentials WHERE subject_did = %s ORDER BY issued_at DESC`, ph(1))
	rows, err := s.db.Query(q, subjectDID)
	if err != nil {
		return nil, fmt.Errorf("get credentials by subject did: %w", err)
	}
	defer rows.Close()
	var credentials []Credential
	for rows.Next() {
		var credential Credential
		var expiresAt, revokedAt sql.NullTime
		if err := rows.Scan(&credential.ID, &credential.SubjectDID, &credential.IssuerDID, &credential.ExternalIssuerDID, &credential.CredentialType, &credential.ClaimsJSON, &credential.Proof, &credential.Status, &credential.IssuanceSource, &credential.IssuedAt, &expiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			credential.ExpiresAt = &t
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			credential.RevokedAt = &t
		}
		credentials = append(credentials, credential)
	}
	return credentials, rows.Err()
}

func (s *SQLStore) CreateTrustAssessment(assessment *TrustAssessment) error {
	if assessment == nil || assessment.ID == "" || assessment.SubjectDID == "" {
		return fmt.Errorf("invalid trust assessment")
	}
	if assessment.EffectiveAt.IsZero() {
		assessment.EffectiveAt = time.Now().UTC()
	}
	if assessment.Source == "" {
		assessment.Source = TrustSourceVerifier
	}
	evidenceRefs, err := json.Marshal(assessment.EvidenceRefs)
	if err != nil {
		return fmt.Errorf("marshal evidence refs: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO trust_assessments (id, subject_did, tier, score, source, evidence_refs, issued_by_did, effective_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10))
	if _, err := s.db.Exec(q, assessment.ID, assessment.SubjectDID, int(assessment.Tier), assessment.Score, string(assessment.Source), string(evidenceRefs), assessment.IssuedByDID, assessment.EffectiveAt, assessment.ExpiresAt, assessment.RevokedAt); err != nil {
		return fmt.Errorf("create trust assessment: %w", err)
	}
	if assessment.RevokedAt == nil {
		updateUser := fmt.Sprintf(`UPDATE users SET trust_tier = %s WHERE did = %s AND trust_tier < %s`, ph(1), ph(2), ph(3))
		if _, err := s.db.Exec(updateUser, int(assessment.Tier), assessment.SubjectDID, int(assessment.Tier)); err != nil {
			return fmt.Errorf("update user trust tier from assessment: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) GetTrustAssessmentsBySubjectDID(subjectDID string) ([]TrustAssessment, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, subject_did, tier, score, source, evidence_refs, issued_by_did, effective_at, expires_at, revoked_at
FROM trust_assessments WHERE subject_did = %s ORDER BY effective_at DESC`, ph(1))
	rows, err := s.db.Query(q, subjectDID)
	if err != nil {
		return nil, fmt.Errorf("get trust assessments by subject did: %w", err)
	}
	defer rows.Close()
	var assessments []TrustAssessment
	for rows.Next() {
		var assessment TrustAssessment
		var tier int
		var evidenceRefsRaw string
		var expiresAt, revokedAt sql.NullTime
		if err := rows.Scan(&assessment.ID, &assessment.SubjectDID, &tier, &assessment.Score, &assessment.Source, &evidenceRefsRaw, &assessment.IssuedByDID, &assessment.EffectiveAt, &expiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan trust assessment: %w", err)
		}
		assessment.Tier = TrustTier(tier)
		if evidenceRefsRaw != "" {
			_ = json.Unmarshal([]byte(evidenceRefsRaw), &assessment.EvidenceRefs)
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			assessment.ExpiresAt = &t
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			assessment.RevokedAt = &t
		}
		assessments = append(assessments, assessment)
	}
	return assessments, rows.Err()
}

func (s *SQLStore) CreateVerifier(verifier *Verifier) error {
	if verifier == nil || verifier.ID == "" || verifier.VerifierDID == "" {
		return fmt.Errorf("invalid verifier")
	}
	if verifier.CreatedAt.IsZero() {
		verifier.CreatedAt = time.Now().UTC()
	}
	if verifier.Status == "" {
		verifier.Status = VerifierActive
	}
	if verifier.AuthorityLevel == 0 {
		verifier.AuthorityLevel = L4_AUTHORITY
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO verifiers (id, verifier_did, verifier_type, scope, authority_level, status, appointed_by_did, created_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
ON CONFLICT (verifier_did) DO UPDATE SET verifier_type = EXCLUDED.verifier_type, scope = EXCLUDED.scope, authority_level = EXCLUDED.authority_level, status = EXCLUDED.status, appointed_by_did = EXCLUDED.appointed_by_did, expires_at = EXCLUDED.expires_at, revoked_at = EXCLUDED.revoked_at`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10))
	if !s.isPostgres() {
		q = fmt.Sprintf(`INSERT INTO verifiers (id, verifier_did, verifier_type, scope, authority_level, status, appointed_by_did, created_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
ON CONFLICT(verifier_did) DO UPDATE SET verifier_type = excluded.verifier_type, scope = excluded.scope, authority_level = excluded.authority_level, status = excluded.status, appointed_by_did = excluded.appointed_by_did, expires_at = excluded.expires_at, revoked_at = excluded.revoked_at`,
			ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10))
	}
	if _, err := s.db.Exec(q, verifier.ID, verifier.VerifierDID, verifier.VerifierType, verifier.Scope, int(verifier.AuthorityLevel), string(verifier.Status), verifier.AppointedByDID, verifier.CreatedAt, verifier.ExpiresAt, verifier.RevokedAt); err != nil {
		return fmt.Errorf("create verifier: %w", err)
	}
	if verifier.Status == VerifierActive {
		updateUser := fmt.Sprintf(`UPDATE users SET trust_tier = %s WHERE did = %s AND trust_tier < %s`, ph(1), ph(2), ph(3))
		if _, err := s.db.Exec(updateUser, int(L4_AUTHORITY), verifier.VerifierDID, int(L4_AUTHORITY)); err != nil {
			return fmt.Errorf("update user trust tier from verifier: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) GetVerifierByDID(did string) (*Verifier, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, verifier_did, verifier_type, scope, authority_level, status, appointed_by_did, created_at, expires_at, revoked_at
FROM verifiers WHERE verifier_did = %s`, ph(1))
	var verifier Verifier
	var authorityLevel int
	var expiresAt, revokedAt sql.NullTime
	err := s.db.QueryRow(q, did).Scan(&verifier.ID, &verifier.VerifierDID, &verifier.VerifierType, &verifier.Scope, &authorityLevel, &verifier.Status, &verifier.AppointedByDID, &verifier.CreatedAt, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get verifier by did: %w", err)
	}
	verifier.AuthorityLevel = TrustTier(authorityLevel)
	if expiresAt.Valid {
		t := expiresAt.Time
		verifier.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		verifier.RevokedAt = &t
	}
	return &verifier, nil
}

func (s *SQLStore) GetVerifiers() ([]Verifier, error) {
	rows, err := s.db.Query(`SELECT id, verifier_did, verifier_type, scope, authority_level, status, appointed_by_did, created_at, expires_at, revoked_at FROM verifiers ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("get verifiers: %w", err)
	}
	defer rows.Close()
	var verifiers []Verifier
	for rows.Next() {
		var verifier Verifier
		var authorityLevel int
		var expiresAt, revokedAt sql.NullTime
		if err := rows.Scan(&verifier.ID, &verifier.VerifierDID, &verifier.VerifierType, &verifier.Scope, &authorityLevel, &verifier.Status, &verifier.AppointedByDID, &verifier.CreatedAt, &expiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan verifier: %w", err)
		}
		verifier.AuthorityLevel = TrustTier(authorityLevel)
		if expiresAt.Valid {
			t := expiresAt.Time
			verifier.ExpiresAt = &t
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			verifier.RevokedAt = &t
		}
		verifiers = append(verifiers, verifier)
	}
	return verifiers, rows.Err()
}

func (s *SQLStore) CreateTrustedIssuer(issuer *TrustedIssuer) error {
	if issuer == nil || issuer.ID == "" || issuer.IssuerDID == "" {
		return fmt.Errorf("invalid trusted issuer")
	}
	if issuer.CreatedAt.IsZero() {
		issuer.CreatedAt = time.Now().UTC()
	}
	if issuer.Status == "" {
		issuer.Status = TrustedIssuerActive
	}
	if issuer.MaxTrustTierIssued == 0 {
		issuer.MaxTrustTierIssued = L3_TEMPORAL
	}
	if issuer.Status == TrustedIssuerRevoked && issuer.RevokedAt == nil {
		now := time.Now().UTC()
		issuer.RevokedAt = &now
	} else if issuer.Status != TrustedIssuerRevoked {
		issuer.RevokedAt = nil
	}
	scopesJSON, err := json.Marshal(issuer.Scopes)
	if err != nil {
		return fmt.Errorf("marshal issuer scopes: %w", err)
	}
	credentialTypesJSON, err := json.Marshal(issuer.CredentialTypes)
	if err != nil {
		return fmt.Errorf("marshal issuer credential types: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO trusted_issuers (id, issuer_did, issuer_name, status, scopes, credential_types, max_trust_tier_issued, appointed_by_did, created_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
ON CONFLICT (issuer_did) DO UPDATE SET issuer_name = EXCLUDED.issuer_name, status = EXCLUDED.status, scopes = EXCLUDED.scopes, credential_types = EXCLUDED.credential_types, max_trust_tier_issued = EXCLUDED.max_trust_tier_issued, appointed_by_did = EXCLUDED.appointed_by_did, expires_at = EXCLUDED.expires_at, revoked_at = EXCLUDED.revoked_at`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11))
	if !s.isPostgres() {
		q = fmt.Sprintf(`INSERT INTO trusted_issuers (id, issuer_did, issuer_name, status, scopes, credential_types, max_trust_tier_issued, appointed_by_did, created_at, expires_at, revoked_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
ON CONFLICT(issuer_did) DO UPDATE SET issuer_name = excluded.issuer_name, status = excluded.status, scopes = excluded.scopes, credential_types = excluded.credential_types, max_trust_tier_issued = excluded.max_trust_tier_issued, appointed_by_did = excluded.appointed_by_did, expires_at = excluded.expires_at, revoked_at = excluded.revoked_at`,
			ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11))
	}
	if _, err := s.db.Exec(q, issuer.ID, issuer.IssuerDID, issuer.IssuerName, string(issuer.Status), string(scopesJSON), string(credentialTypesJSON), int(issuer.MaxTrustTierIssued), issuer.AppointedByDID, issuer.CreatedAt, issuer.ExpiresAt, issuer.RevokedAt); err != nil {
		return fmt.Errorf("create trusted issuer: %w", err)
	}
	return nil
}

func (s *SQLStore) GetTrustedIssuerByDID(did string) (*TrustedIssuer, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, issuer_did, issuer_name, status, scopes, credential_types, max_trust_tier_issued, appointed_by_did, created_at, expires_at, revoked_at
FROM trusted_issuers WHERE issuer_did = %s`, ph(1))
	var issuer TrustedIssuer
	var scopesRaw, credentialTypesRaw string
	var maxTier int
	var expiresAt, revokedAt sql.NullTime
	err := s.db.QueryRow(q, did).Scan(&issuer.ID, &issuer.IssuerDID, &issuer.IssuerName, &issuer.Status, &scopesRaw, &credentialTypesRaw, &maxTier, &issuer.AppointedByDID, &issuer.CreatedAt, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trusted issuer by did: %w", err)
	}
	issuer.MaxTrustTierIssued = TrustTier(maxTier)
	_ = json.Unmarshal([]byte(scopesRaw), &issuer.Scopes)
	_ = json.Unmarshal([]byte(credentialTypesRaw), &issuer.CredentialTypes)
	if expiresAt.Valid {
		t := expiresAt.Time
		issuer.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		issuer.RevokedAt = &t
	}
	return &issuer, nil
}

func (s *SQLStore) GetTrustedIssuers() ([]TrustedIssuer, error) {
	rows, err := s.db.Query(`SELECT id, issuer_did, issuer_name, status, scopes, credential_types, max_trust_tier_issued, appointed_by_did, created_at, expires_at, revoked_at FROM trusted_issuers ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("get trusted issuers: %w", err)
	}
	defer rows.Close()
	var issuers []TrustedIssuer
	for rows.Next() {
		var issuer TrustedIssuer
		var scopesRaw, credentialTypesRaw string
		var maxTier int
		var expiresAt, revokedAt sql.NullTime
		if err := rows.Scan(&issuer.ID, &issuer.IssuerDID, &issuer.IssuerName, &issuer.Status, &scopesRaw, &credentialTypesRaw, &maxTier, &issuer.AppointedByDID, &issuer.CreatedAt, &expiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan trusted issuer: %w", err)
		}
		issuer.MaxTrustTierIssued = TrustTier(maxTier)
		_ = json.Unmarshal([]byte(scopesRaw), &issuer.Scopes)
		_ = json.Unmarshal([]byte(credentialTypesRaw), &issuer.CredentialTypes)
		if expiresAt.Valid {
			t := expiresAt.Time
			issuer.ExpiresAt = &t
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			issuer.RevokedAt = &t
		}
		issuers = append(issuers, issuer)
	}
	return issuers, rows.Err()
}

func (s *SQLStore) CreateWalletPresentationRequest(request *WalletPresentationRequest) error {
	if request == nil || request.ID == "" || request.SubjectDID == "" {
		return fmt.Errorf("invalid wallet presentation request")
	}
	if request.CreatedAt.IsZero() {
		request.CreatedAt = time.Now().UTC()
	}
	if request.Status == "" {
		request.Status = WalletRequestPending
	}
	allowedIssuerDIDsJSON, err := json.Marshal(request.AllowedIssuerDIDs)
	if err != nil {
		return fmt.Errorf("marshal allowed issuer dids: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO wallet_presentation_requests (id, subject_did, verifier_did, requested_tier, credential_type, purpose, allowed_issuer_dids, challenge, request_uri, qr_payload, status, created_at, expires_at, completed_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12), ph(13), ph(14))
	if _, err := s.db.Exec(q, request.ID, request.SubjectDID, request.VerifierDID, int(request.RequestedTier), request.CredentialType, request.Purpose, string(allowedIssuerDIDsJSON), request.Challenge, request.RequestURI, request.QRPayload, string(request.Status), request.CreatedAt, request.ExpiresAt, request.CompletedAt); err != nil {
		return fmt.Errorf("create wallet presentation request: %w", err)
	}
	return nil
}

func (s *SQLStore) GetWalletPresentationRequestByID(id string) (*WalletPresentationRequest, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, subject_did, verifier_did, requested_tier, credential_type, purpose, allowed_issuer_dids, challenge, request_uri, qr_payload, status, created_at, expires_at, completed_at
FROM wallet_presentation_requests WHERE id = %s`, ph(1))
	var request WalletPresentationRequest
	var requestedTier int
	var allowedIssuerDIDsRaw string
	var expiresAt, completedAt sql.NullTime
	err := s.db.QueryRow(q, id).Scan(&request.ID, &request.SubjectDID, &request.VerifierDID, &requestedTier, &request.CredentialType, &request.Purpose, &allowedIssuerDIDsRaw, &request.Challenge, &request.RequestURI, &request.QRPayload, &request.Status, &request.CreatedAt, &expiresAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get wallet presentation request by id: %w", err)
	}
	request.RequestedTier = TrustTier(requestedTier)
	_ = json.Unmarshal([]byte(allowedIssuerDIDsRaw), &request.AllowedIssuerDIDs)
	if expiresAt.Valid {
		t := expiresAt.Time
		request.ExpiresAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		request.CompletedAt = &t
	}
	return &request, nil
}

func (s *SQLStore) GetWalletPresentationRequests(subjectDID *string) ([]WalletPresentationRequest, error) {
	q := `SELECT id, subject_did, verifier_did, requested_tier, credential_type, purpose, allowed_issuer_dids, challenge, request_uri, qr_payload, status, created_at, expires_at, completed_at
FROM wallet_presentation_requests`
	args := []any{}
	clauses := []string{}
	ph := s.placeholder
	if subjectDID != nil {
		args = append(args, *subjectDID)
		clauses = append(clauses, fmt.Sprintf("subject_did = %s", ph(len(args))))
	}
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get wallet presentation requests: %w", err)
	}
	defer rows.Close()
	var requests []WalletPresentationRequest
	for rows.Next() {
		var request WalletPresentationRequest
		var requestedTier int
		var allowedIssuerDIDsRaw string
		var expiresAt, completedAt sql.NullTime
		if err := rows.Scan(&request.ID, &request.SubjectDID, &request.VerifierDID, &requestedTier, &request.CredentialType, &request.Purpose, &allowedIssuerDIDsRaw, &request.Challenge, &request.RequestURI, &request.QRPayload, &request.Status, &request.CreatedAt, &expiresAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan wallet presentation request: %w", err)
		}
		request.RequestedTier = TrustTier(requestedTier)
		_ = json.Unmarshal([]byte(allowedIssuerDIDsRaw), &request.AllowedIssuerDIDs)
		if expiresAt.Valid {
			t := expiresAt.Time
			request.ExpiresAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			request.CompletedAt = &t
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *SQLStore) UpdateWalletPresentationRequest(request *WalletPresentationRequest) error {
	if request == nil || request.ID == "" {
		return fmt.Errorf("invalid wallet presentation request")
	}
	allowedIssuerDIDsJSON, err := json.Marshal(request.AllowedIssuerDIDs)
	if err != nil {
		return fmt.Errorf("marshal allowed issuer dids: %w", err)
	}
	ph := s.placeholder
	q := fmt.Sprintf(`UPDATE wallet_presentation_requests
SET subject_did = %s, verifier_did = %s, requested_tier = %s, credential_type = %s, purpose = %s,
    allowed_issuer_dids = %s, challenge = %s, request_uri = %s, qr_payload = %s, status = %s,
    created_at = %s, expires_at = %s, completed_at = %s
WHERE id = %s`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12), ph(13), ph(14))
	res, err := s.db.Exec(q, request.SubjectDID, request.VerifierDID, int(request.RequestedTier), request.CredentialType, request.Purpose, string(allowedIssuerDIDsJSON), request.Challenge, request.RequestURI, request.QRPayload, string(request.Status), request.CreatedAt, request.ExpiresAt, request.CompletedAt, request.ID)
	if err != nil {
		return fmt.Errorf("update wallet presentation request: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("wallet presentation request not found")
	}
	return nil
}

func (s *SQLStore) CreateWalletPresentationVerification(verification *WalletPresentationVerification) error {
	if verification == nil || verification.ID == "" || verification.RequestID == "" {
		return fmt.Errorf("invalid wallet presentation verification")
	}
	if verification.CreatedAt.IsZero() {
		verification.CreatedAt = time.Now().UTC()
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO wallet_presentation_verifications (id, request_id, subject_did, issuer_did, credential_type, presentation_format, claims_json, proof, audience, nonce, status, trusted_issuer_did, notes, issued_credential_id, issued_assessment_id, created_at, verified_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11), ph(12), ph(13), ph(14), ph(15), ph(16), ph(17))
	if _, err := s.db.Exec(q, verification.ID, verification.RequestID, verification.SubjectDID, verification.IssuerDID, verification.CredentialType, verification.PresentationFormat, verification.ClaimsJSON, verification.Proof, verification.Audience, verification.Nonce, string(verification.Status), nullStringValue(verification.TrustedIssuerDID), verification.Notes, verification.IssuedCredentialID, verification.IssuedAssessmentID, verification.CreatedAt, verification.VerifiedAt); err != nil {
		return fmt.Errorf("create wallet presentation verification: %w", err)
	}
	return nil
}

func (s *SQLStore) GetLatestWalletPresentationVerification(requestID string) (*WalletPresentationVerification, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, request_id, subject_did, issuer_did, credential_type, presentation_format, claims_json, proof, audience, nonce, status, COALESCE(trusted_issuer_did, ''), notes, issued_credential_id, issued_assessment_id, created_at, verified_at
FROM wallet_presentation_verifications WHERE request_id = %s ORDER BY created_at DESC LIMIT 1`, ph(1))
	var verification WalletPresentationVerification
	var trustedIssuerDID, notes string
	var issuedCredentialID, issuedAssessmentID sql.NullString
	var verifiedAt sql.NullTime
	err := s.db.QueryRow(q, requestID).Scan(&verification.ID, &verification.RequestID, &verification.SubjectDID, &verification.IssuerDID, &verification.CredentialType, &verification.PresentationFormat, &verification.ClaimsJSON, &verification.Proof, &verification.Audience, &verification.Nonce, &verification.Status, &trustedIssuerDID, &notes, &issuedCredentialID, &issuedAssessmentID, &verification.CreatedAt, &verifiedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest wallet presentation verification: %w", err)
	}
	verification.TrustedIssuerDID = trustedIssuerDID
	verification.Notes = notes
	if issuedCredentialID.Valid {
		verification.IssuedCredentialID = &issuedCredentialID.String
	}
	if issuedAssessmentID.Valid {
		verification.IssuedAssessmentID = &issuedAssessmentID.String
	}
	if verifiedAt.Valid {
		t := verifiedAt.Time
		verification.VerifiedAt = &t
	}
	return &verification, nil
}

func (s *SQLStore) CreateVerificationCase(verificationCase *VerificationCase) error {
	if verificationCase == nil || verificationCase.ID == "" || verificationCase.SubjectDID == "" {
		return fmt.Errorf("invalid verification case")
	}
	if verificationCase.CreatedAt.IsZero() {
		verificationCase.CreatedAt = time.Now().UTC()
	}
	if verificationCase.Status == "" {
		verificationCase.Status = VerificationSubmitted
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO verification_cases (id, subject_did, requested_tier, credential_type, evidence_json, status, assigned_verifier_did, decision, decision_reason, created_at, decided_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11))
	if _, err := s.db.Exec(q, verificationCase.ID, verificationCase.SubjectDID, int(verificationCase.RequestedTier), verificationCase.CredentialType, verificationCase.EvidenceJSON, string(verificationCase.Status), nullStringValue(verificationCase.AssignedVerifierDID), verificationCase.Decision, verificationCase.DecisionReason, verificationCase.CreatedAt, verificationCase.DecidedAt); err != nil {
		return fmt.Errorf("create verification case: %w", err)
	}
	return nil
}

func (s *SQLStore) GetVerificationCaseByID(id string) (*VerificationCase, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, subject_did, requested_tier, credential_type, evidence_json, status, assigned_verifier_did, decision, decision_reason, created_at, decided_at
FROM verification_cases WHERE id = %s`, ph(1))
	var verificationCase VerificationCase
	var requestedTier int
	var assignedVerifier sql.NullString
	var decidedAt sql.NullTime
	err := s.db.QueryRow(q, id).Scan(&verificationCase.ID, &verificationCase.SubjectDID, &requestedTier, &verificationCase.CredentialType, &verificationCase.EvidenceJSON, &verificationCase.Status, &assignedVerifier, &verificationCase.Decision, &verificationCase.DecisionReason, &verificationCase.CreatedAt, &decidedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get verification case by id: %w", err)
	}
	verificationCase.RequestedTier = TrustTier(requestedTier)
	if assignedVerifier.Valid {
		verificationCase.AssignedVerifierDID = assignedVerifier.String
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		verificationCase.DecidedAt = &t
	}
	return &verificationCase, nil
}

func (s *SQLStore) GetVerificationCases(subjectDID *string, assignedVerifierDID *string) ([]VerificationCase, error) {
	q := `SELECT id, subject_did, requested_tier, credential_type, evidence_json, status, assigned_verifier_did, decision, decision_reason, created_at, decided_at FROM verification_cases`
	args := []any{}
	clauses := []string{}
	ph := s.placeholder
	if subjectDID != nil {
		args = append(args, *subjectDID)
		clauses = append(clauses, fmt.Sprintf("subject_did = %s", ph(len(args))))
	}
	if assignedVerifierDID != nil {
		args = append(args, *assignedVerifierDID)
		clauses = append(clauses, fmt.Sprintf("assigned_verifier_did = %s", ph(len(args))))
	}
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get verification cases: %w", err)
	}
	defer rows.Close()
	var cases []VerificationCase
	for rows.Next() {
		var verificationCase VerificationCase
		var requestedTier int
		var assignedVerifier sql.NullString
		var decidedAt sql.NullTime
		if err := rows.Scan(&verificationCase.ID, &verificationCase.SubjectDID, &requestedTier, &verificationCase.CredentialType, &verificationCase.EvidenceJSON, &verificationCase.Status, &assignedVerifier, &verificationCase.Decision, &verificationCase.DecisionReason, &verificationCase.CreatedAt, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan verification case: %w", err)
		}
		verificationCase.RequestedTier = TrustTier(requestedTier)
		if assignedVerifier.Valid {
			verificationCase.AssignedVerifierDID = assignedVerifier.String
		}
		if decidedAt.Valid {
			t := decidedAt.Time
			verificationCase.DecidedAt = &t
		}
		cases = append(cases, verificationCase)
	}
	return cases, rows.Err()
}

func (s *SQLStore) UpdateVerificationCase(verificationCase *VerificationCase) error {
	if verificationCase == nil || verificationCase.ID == "" {
		return fmt.Errorf("invalid verification case")
	}
	ph := s.placeholder
	q := fmt.Sprintf(`UPDATE verification_cases
SET subject_did = %s, requested_tier = %s, credential_type = %s, evidence_json = %s, status = %s,
    assigned_verifier_did = %s, decision = %s, decision_reason = %s, created_at = %s, decided_at = %s
WHERE id = %s`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10), ph(11))
	res, err := s.db.Exec(q, verificationCase.SubjectDID, int(verificationCase.RequestedTier), verificationCase.CredentialType, verificationCase.EvidenceJSON, string(verificationCase.Status), nullStringValue(verificationCase.AssignedVerifierDID), verificationCase.Decision, verificationCase.DecisionReason, verificationCase.CreatedAt, verificationCase.DecidedAt, verificationCase.ID)
	if err != nil {
		return fmt.Errorf("update verification case: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("verification case not found")
	}
	return nil
}

func (s *SQLStore) CreateVerifierDecision(decision *VerifierDecision) error {
	if decision == nil || decision.ID == "" || decision.CaseID == "" || decision.VerifierDID == "" {
		return fmt.Errorf("invalid verifier decision")
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now().UTC()
	}
	if decision.CredentialIssuanceSource == "" {
		decision.CredentialIssuanceSource = CredentialInternalVerifierIssued
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO verifier_decisions (id, case_id, verifier_did, decision, reason, credential_issuance_source, external_issuer_did, issued_credential_id, issued_assessment_id, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10))
	if _, err := s.db.Exec(q, decision.ID, decision.CaseID, decision.VerifierDID, string(decision.Decision), decision.Reason, string(decision.CredentialIssuanceSource), nullStringValue(decision.ExternalIssuerDID), decision.IssuedCredentialID, decision.IssuedAssessmentID, decision.CreatedAt); err != nil {
		return fmt.Errorf("create verifier decision: %w", err)
	}
	return nil
}

func (s *SQLStore) GetVerifierDecisionsByCaseID(caseID string) ([]VerifierDecision, error) {
	ph := s.placeholder
	q := fmt.Sprintf(`SELECT id, case_id, verifier_did, decision, reason, credential_issuance_source, COALESCE(external_issuer_did, ''), issued_credential_id, issued_assessment_id, created_at
FROM verifier_decisions WHERE case_id = %s ORDER BY created_at DESC`, ph(1))
	rows, err := s.db.Query(q, caseID)
	if err != nil {
		return nil, fmt.Errorf("get verifier decisions by case id: %w", err)
	}
	defer rows.Close()
	var decisions []VerifierDecision
	for rows.Next() {
		var decision VerifierDecision
		var credentialID, assessmentID sql.NullString
		if err := rows.Scan(&decision.ID, &decision.CaseID, &decision.VerifierDID, &decision.Decision, &decision.Reason, &decision.CredentialIssuanceSource, &decision.ExternalIssuerDID, &credentialID, &assessmentID, &decision.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan verifier decision: %w", err)
		}
		if credentialID.Valid {
			decision.IssuedCredentialID = &credentialID.String
		}
		if assessmentID.Valid {
			decision.IssuedAssessmentID = &assessmentID.String
		}
		decisions = append(decisions, decision)
	}
	return decisions, rows.Err()
}

func (s *SQLStore) CreateTrustAuditLog(entry *TrustAuditLog) error {
	if entry == nil || entry.ID == "" || entry.ActorDID == "" || entry.ActionType == "" {
		return fmt.Errorf("invalid trust audit log")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	ph := s.placeholder
	q := fmt.Sprintf(`INSERT INTO trust_audit_logs (id, actor_did, action_type, target_did, target_resource_id, metadata_json, created_at)
VALUES (%s, %s, %s, %s, %s, %s, %s)`, ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7))
	if _, err := s.db.Exec(q, entry.ID, entry.ActorDID, entry.ActionType, entry.TargetDID, entry.TargetResourceID, entry.MetadataJSON, entry.CreatedAt); err != nil {
		return fmt.Errorf("create trust audit log: %w", err)
	}
	return nil
}

func (s *SQLStore) GetTrustAuditLogs(subjectDID *string, limit int) ([]TrustAuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id, actor_did, action_type, target_did, target_resource_id, metadata_json, created_at FROM trust_audit_logs`
	args := []any{}
	if subjectDID != nil {
		args = append(args, *subjectDID, *subjectDID)
		ph := s.placeholder
		q += fmt.Sprintf(" WHERE actor_did = %s OR target_did = %s", ph(1), ph(2))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %s", s.placeholder(len(args)))
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("get trust audit logs: %w", err)
	}
	defer rows.Close()
	var logs []TrustAuditLog
	for rows.Next() {
		var entry TrustAuditLog
		if err := rows.Scan(&entry.ID, &entry.ActorDID, &entry.ActionType, &entry.TargetDID, &entry.TargetResourceID, &entry.MetadataJSON, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trust audit log: %w", err)
		}
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}

func (s *SQLStore) isPostgres() bool {
	name := strings.ToLower(strings.TrimSpace(s.driverName))
	return strings.Contains(name, "pgx") || strings.Contains(name, "postgres")
}

func (s *SQLStore) placeholder(n int) string {
	if s.isPostgres() {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func nullInt64Value(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullStringValue(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
