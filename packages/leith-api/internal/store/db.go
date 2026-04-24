package store

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store implementation for local development.
type MemoryStore struct {
	mu                  sync.RWMutex
	nextUID             int64
	nextPID             int64
	nextContentID       int64
	nextNodeID          int64
	users               map[string]*User
	posts               map[int64]*Post
	postList            []int64
	contentItems        map[string]*ContentItem
	contentItemList     []string
	contentRelations    []ContentRelation
	projections         []Projection
	transformationJobs  map[string]*TransformationJob
	discussionForks     []DiscussionFork
	moderationActions   []ModerationAction
	discussionNodes     map[string][]DiscussionNode
	credentials         map[string]*Credential
	trustAssessments    map[string]*TrustAssessment
	verifiers           map[string]*Verifier
	trustedIssuers      map[string]*TrustedIssuer
	walletRequests      map[string]*WalletPresentationRequest
	walletVerifications map[string][]WalletPresentationVerification
	verificationCases   map[string]*VerificationCase
	verifierDecisions   map[string][]VerifierDecision
	trustAuditLogs      []TrustAuditLog
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextUID:             1,
		nextPID:             1,
		nextContentID:       1,
		nextNodeID:          1,
		users:               make(map[string]*User),
		posts:               make(map[int64]*Post),
		contentItems:        make(map[string]*ContentItem),
		contentRelations:    []ContentRelation{},
		projections:         []Projection{},
		transformationJobs:  make(map[string]*TransformationJob),
		discussionForks:     []DiscussionFork{},
		moderationActions:   []ModerationAction{},
		discussionNodes:     make(map[string][]DiscussionNode),
		credentials:         make(map[string]*Credential),
		trustAssessments:    make(map[string]*TrustAssessment),
		verifiers:           make(map[string]*Verifier),
		trustedIssuers:      make(map[string]*TrustedIssuer),
		walletRequests:      make(map[string]*WalletPresentationRequest),
		walletVerifications: make(map[string][]WalletPresentationVerification),
		verificationCases:   make(map[string]*VerificationCase),
		verifierDecisions:   make(map[string][]VerifierDecision),
		trustAuditLogs:      []TrustAuditLog{},
	}
}

func (m *MemoryStore) CreateUser(user *User) error {
	if user == nil || user.DID == "" {
		return fmt.Errorf("invalid user")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.users[user.DID]; ok {
		if user.OAuthID != "" {
			existing.OAuthID = user.OAuthID
		}
		if len(user.PublicKey) > 0 {
			existing.PublicKey = append([]byte(nil), user.PublicKey...)
		}
		if len(user.AuthnUserID) > 0 {
			existing.AuthnUserID = append([]byte(nil), user.AuthnUserID...)
		}
		if len(user.PasskeyCredentials) > 0 {
			existing.PasskeyCredentials = append([]byte(nil), user.PasskeyCredentials...)
		}
		if user.TrustTier != 0 || existing.TrustTier == 0 {
			existing.TrustTier = user.TrustTier
		}
		user.ID = existing.ID
		user.CreatedAt = existing.CreatedAt
		user.PublicKey = append([]byte(nil), existing.PublicKey...)
		user.AuthnUserID = append([]byte(nil), existing.AuthnUserID...)
		user.PasskeyCredentials = append([]byte(nil), existing.PasskeyCredentials...)
		user.TrustTier = existing.TrustTier
		return nil
	}

	copyUser := *user
	copyUser.ID = m.nextUID
	m.nextUID++
	if copyUser.CreatedAt.IsZero() {
		copyUser.CreatedAt = time.Now()
	}
	m.users[user.DID] = &copyUser
	user.ID = copyUser.ID
	user.CreatedAt = copyUser.CreatedAt
	return nil
}

func (m *MemoryStore) GetUserByDID(did string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[did]
	if !ok {
		return nil, nil
	}
	copyUser := *u
	copyUser.PublicKey = append([]byte(nil), u.PublicKey...)
	copyUser.AuthnUserID = append([]byte(nil), u.AuthnUserID...)
	copyUser.PasskeyCredentials = append([]byte(nil), u.PasskeyCredentials...)
	return &copyUser, nil
}

func (m *MemoryStore) CreatePost(post *Post) error {
	if post == nil {
		return fmt.Errorf("post is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	copyPost := *post
	copyPost.ID = m.nextPID
	m.nextPID++
	if copyPost.CreatedAt.IsZero() {
		copyPost.CreatedAt = time.Now()
	}

	m.posts[copyPost.ID] = &copyPost
	m.postList = append(m.postList, copyPost.ID)

	post.ID = copyPost.ID
	post.CreatedAt = copyPost.CreatedAt
	return nil
}

func (m *MemoryStore) GetPosts(limit, offset int) ([]Post, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]int64, 0, len(m.postList))
	ids = append(ids, m.postList...)
	sort.Slice(ids, func(i, j int) bool {
		a := m.posts[ids[i]]
		b := m.posts[ids[j]]
		if a.VisibilityScore == b.VisibilityScore {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return a.VisibilityScore > b.VisibilityScore
	})

	if offset >= len(ids) {
		return []Post{}, nil
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}

	res := make([]Post, 0, end-offset)
	for _, id := range ids[offset:end] {
		p := m.posts[id]
		res = append(res, *p)
	}
	return res, nil
}

func (m *MemoryStore) UpdateVisibilityScore(postID int64, score float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.posts[postID]
	if !ok {
		return fmt.Errorf("post not found")
	}
	p.VisibilityScore += score
	return nil
}

func (m *MemoryStore) CheckRateLimit(_ string, _ TrustTier) (bool, error) {
	return true, nil
}

func (m *MemoryStore) CreateContentItem(item *ContentItem) error {
	if item == nil || item.AuthorDID == "" || item.Body == "" {
		return fmt.Errorf("invalid content item")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	copyItem := *item
	if copyItem.ID == "" {
		copyItem.ID = fmt.Sprintf("cnt_%d", m.nextContentID)
		m.nextContentID++
	}
	now := time.Now()
	if copyItem.CreatedAt.IsZero() {
		copyItem.CreatedAt = now
	}
	if copyItem.UpdatedAt.IsZero() {
		copyItem.UpdatedAt = copyItem.CreatedAt
	}
	if copyItem.Status == "" {
		copyItem.Status = StatusDraft
	}
	if copyItem.Visibility == "" {
		if copyItem.Mode == ModeDiscussion {
			copyItem.Visibility = VisibilityPublic
		} else {
			copyItem.Visibility = VisibilityPrivate
		}
	}
	if copyItem.Mode == ModeDiscussion {
		if copyItem.ParticipationPolicy == "" {
			copyItem.ParticipationPolicy = ParticipationComment
		}
		if copyItem.DiscussionShape == "" {
			copyItem.DiscussionShape = DiscussionThread
		}
	}

	m.contentItems[copyItem.ID] = &copyItem
	m.contentItemList = append(m.contentItemList, copyItem.ID)

	item.ID = copyItem.ID
	item.CreatedAt = copyItem.CreatedAt
	item.UpdatedAt = copyItem.UpdatedAt
	item.Status = copyItem.Status
	item.Visibility = copyItem.Visibility
	item.ParticipationPolicy = copyItem.ParticipationPolicy
	item.DiscussionShape = copyItem.DiscussionShape
	return nil
}

func (m *MemoryStore) GetContentItemByID(id string) (*ContentItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.contentItems[id]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (m *MemoryStore) GetContentItems(mode *ContentMode, visibility *ContentVisibility, authorDID *string, limit, offset int) ([]ContentItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]ContentItem, 0, len(m.contentItemList))
	for i := len(m.contentItemList) - 1; i >= 0; i-- {
		id := m.contentItemList[i]
		item := m.contentItems[id]
		if mode != nil && item.Mode != *mode {
			continue
		}
		if visibility != nil && item.Visibility != *visibility {
			continue
		}
		if authorDID != nil && item.AuthorDID != *authorDID {
			continue
		}
		items = append(items, *item)
	}

	if offset >= len(items) {
		return []ContentItem{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}

func (m *MemoryStore) CreateDiscussionNode(node *DiscussionNode) error {
	if node == nil || node.DiscussionID == "" || node.AuthorDID == "" || node.Body == "" {
		return fmt.Errorf("invalid discussion node")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contentItems[node.DiscussionID]; !ok {
		return fmt.Errorf("discussion not found")
	}

	copyNode := *node
	if copyNode.ID == "" {
		copyNode.ID = fmt.Sprintf("node_%d", m.nextNodeID)
		m.nextNodeID++
	}
	if copyNode.CreatedAt.IsZero() {
		copyNode.CreatedAt = time.Now()
	}

	m.discussionNodes[copyNode.DiscussionID] = append(m.discussionNodes[copyNode.DiscussionID], copyNode)
	*node = copyNode
	return nil
}

func (m *MemoryStore) GetDiscussionNodes(discussionID string) ([]DiscussionNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := m.discussionNodes[discussionID]
	res := make([]DiscussionNode, 0, len(nodes))
	for _, node := range nodes {
		res = append(res, node)
	}
	return res, nil
}

func (m *MemoryStore) CreateProjection(projection *Projection) error {
	if projection == nil || projection.SourceIdeaID == "" || projection.TargetDiscussionID == "" {
		return fmt.Errorf("invalid projection")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyProjection := *projection
	m.projections = append(m.projections, copyProjection)
	*projection = copyProjection
	return nil
}

func (m *MemoryStore) GetProjectionsBySourceIdeaID(sourceIdeaID string) ([]Projection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []Projection{}
	for _, projection := range m.projections {
		if projection.SourceIdeaID == sourceIdeaID {
			res = append(res, projection)
		}
	}
	return res, nil
}

func (m *MemoryStore) CreateContentRelation(relation *ContentRelation) error {
	if relation == nil || relation.FromContentID == "" || relation.ToContentID == "" {
		return fmt.Errorf("invalid content relation")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyRelation := *relation
	m.contentRelations = append(m.contentRelations, copyRelation)
	*relation = copyRelation
	return nil
}

func (m *MemoryStore) GetContentRelations(contentID string) ([]ContentRelation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []ContentRelation{}
	for _, relation := range m.contentRelations {
		if relation.FromContentID == contentID || relation.ToContentID == contentID {
			res = append(res, relation)
		}
	}
	return res, nil
}

func (m *MemoryStore) CreateTransformationJob(job *TransformationJob) error {
	if job == nil || job.ID == "" {
		return fmt.Errorf("invalid transformation job")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyJob := *job
	m.transformationJobs[copyJob.ID] = &copyJob
	*job = copyJob
	return nil
}

func (m *MemoryStore) GetTransformationJobByID(id string) (*TransformationJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.transformationJobs[id]
	if !ok {
		return nil, nil
	}
	copyJob := *job
	return &copyJob, nil
}

func (m *MemoryStore) UpdateTransformationJob(job *TransformationJob) error {
	if job == nil || job.ID == "" {
		return fmt.Errorf("invalid transformation job")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.transformationJobs[job.ID]; !ok {
		return fmt.Errorf("transformation job not found")
	}
	copyJob := *job
	m.transformationJobs[job.ID] = &copyJob
	return nil
}

func (m *MemoryStore) CreateDiscussionFork(fork *DiscussionFork) error {
	if fork == nil || fork.ID == "" {
		return fmt.Errorf("invalid discussion fork")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyFork := *fork
	m.discussionForks = append(m.discussionForks, copyFork)
	*fork = copyFork
	return nil
}

func (m *MemoryStore) GetDiscussionForks(sourceDiscussionID string) ([]DiscussionFork, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []DiscussionFork{}
	for _, fork := range m.discussionForks {
		if fork.SourceDiscussionID == sourceDiscussionID {
			res = append(res, fork)
		}
	}
	return res, nil
}

func (m *MemoryStore) CreateModerationAction(action *ModerationAction) error {
	if action == nil || action.ID == "" {
		return fmt.Errorf("invalid moderation action")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyAction := *action
	m.moderationActions = append(m.moderationActions, copyAction)
	*action = copyAction
	return nil
}

func (m *MemoryStore) GetModerationActions(targetContentID string) ([]ModerationAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []ModerationAction{}
	for _, action := range m.moderationActions {
		if action.TargetContentID == targetContentID {
			res = append(res, action)
		}
	}
	return res, nil
}

func (m *MemoryStore) CreateCredential(credential *Credential) error {
	if credential == nil || credential.ID == "" || credential.SubjectDID == "" || credential.IssuerDID == "" {
		return fmt.Errorf("invalid credential")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyCredential := *credential
	if copyCredential.Status == "" {
		copyCredential.Status = CredentialActive
	}
	if copyCredential.IssuanceSource == "" {
		copyCredential.IssuanceSource = CredentialInternalVerifierIssued
	}
	if copyCredential.IssuedAt.IsZero() {
		copyCredential.IssuedAt = time.Now().UTC()
	}
	m.credentials[copyCredential.ID] = &copyCredential
	*credential = copyCredential
	return nil
}

func (m *MemoryStore) GetCredentialsBySubjectDID(subjectDID string) ([]Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []Credential{}
	for _, credential := range m.credentials {
		if credential.SubjectDID == subjectDID {
			res = append(res, *credential)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].IssuedAt.After(res[j].IssuedAt)
	})
	return res, nil
}

func (m *MemoryStore) CreateTrustAssessment(assessment *TrustAssessment) error {
	if assessment == nil || assessment.ID == "" || assessment.SubjectDID == "" {
		return fmt.Errorf("invalid trust assessment")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyAssessment := *assessment
	if copyAssessment.EffectiveAt.IsZero() {
		copyAssessment.EffectiveAt = time.Now().UTC()
	}
	if copyAssessment.Source == "" {
		copyAssessment.Source = TrustSourceVerifier
	}
	m.trustAssessments[copyAssessment.ID] = &copyAssessment
	if user, ok := m.users[copyAssessment.SubjectDID]; ok && copyAssessment.RevokedAt == nil && copyAssessment.Tier > user.TrustTier {
		user.TrustTier = copyAssessment.Tier
	}
	*assessment = copyAssessment
	return nil
}

func (m *MemoryStore) GetTrustAssessmentsBySubjectDID(subjectDID string) ([]TrustAssessment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []TrustAssessment{}
	for _, assessment := range m.trustAssessments {
		if assessment.SubjectDID == subjectDID {
			res = append(res, *assessment)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].EffectiveAt.After(res[j].EffectiveAt)
	})
	return res, nil
}

func (m *MemoryStore) CreateVerifier(verifier *Verifier) error {
	if verifier == nil || verifier.ID == "" || verifier.VerifierDID == "" {
		return fmt.Errorf("invalid verifier")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyVerifier := *verifier
	if copyVerifier.CreatedAt.IsZero() {
		copyVerifier.CreatedAt = time.Now().UTC()
	}
	if copyVerifier.Status == "" {
		copyVerifier.Status = VerifierActive
	}
	if copyVerifier.AuthorityLevel == 0 {
		copyVerifier.AuthorityLevel = L4_AUTHORITY
	}
	m.verifiers[copyVerifier.VerifierDID] = &copyVerifier
	if user, ok := m.users[copyVerifier.VerifierDID]; ok && copyVerifier.Status == VerifierActive && user.TrustTier < L4_AUTHORITY {
		user.TrustTier = L4_AUTHORITY
	}
	*verifier = copyVerifier
	return nil
}

func (m *MemoryStore) GetVerifierByDID(did string) (*Verifier, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	verifier, ok := m.verifiers[did]
	if !ok {
		return nil, nil
	}
	copyVerifier := *verifier
	return &copyVerifier, nil
}

func (m *MemoryStore) GetVerifiers() ([]Verifier, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []Verifier{}
	for _, verifier := range m.verifiers {
		res = append(res, *verifier)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res, nil
}

func (m *MemoryStore) CreateTrustedIssuer(issuer *TrustedIssuer) error {
	if issuer == nil || issuer.ID == "" || issuer.IssuerDID == "" {
		return fmt.Errorf("invalid trusted issuer")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyIssuer := *issuer
	if existing, ok := m.trustedIssuers[copyIssuer.IssuerDID]; ok {
		copyIssuer.ID = existing.ID
		copyIssuer.CreatedAt = existing.CreatedAt
		if copyIssuer.ExpiresAt == nil {
			copyIssuer.ExpiresAt = existing.ExpiresAt
		}
	}
	if copyIssuer.CreatedAt.IsZero() {
		copyIssuer.CreatedAt = time.Now().UTC()
	}
	if copyIssuer.Status == "" {
		copyIssuer.Status = TrustedIssuerActive
	}
	if copyIssuer.MaxTrustTierIssued == 0 {
		copyIssuer.MaxTrustTierIssued = L3_TEMPORAL
	}
	if copyIssuer.Status == TrustedIssuerRevoked && copyIssuer.RevokedAt == nil {
		now := time.Now().UTC()
		copyIssuer.RevokedAt = &now
	} else if copyIssuer.Status != TrustedIssuerRevoked {
		copyIssuer.RevokedAt = nil
	}
	copyIssuer.Scopes = append([]string(nil), issuer.Scopes...)
	copyIssuer.CredentialTypes = append([]string(nil), issuer.CredentialTypes...)
	m.trustedIssuers[copyIssuer.IssuerDID] = &copyIssuer
	*issuer = copyIssuer
	return nil
}

func (m *MemoryStore) GetTrustedIssuerByDID(did string) (*TrustedIssuer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	issuer, ok := m.trustedIssuers[did]
	if !ok {
		return nil, nil
	}
	copyIssuer := *issuer
	copyIssuer.Scopes = append([]string(nil), issuer.Scopes...)
	copyIssuer.CredentialTypes = append([]string(nil), issuer.CredentialTypes...)
	return &copyIssuer, nil
}

func (m *MemoryStore) GetTrustedIssuers() ([]TrustedIssuer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []TrustedIssuer{}
	for _, issuer := range m.trustedIssuers {
		copyIssuer := *issuer
		copyIssuer.Scopes = append([]string(nil), issuer.Scopes...)
		copyIssuer.CredentialTypes = append([]string(nil), issuer.CredentialTypes...)
		res = append(res, copyIssuer)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res, nil
}

func (m *MemoryStore) CreateWalletPresentationRequest(request *WalletPresentationRequest) error {
	if request == nil || request.ID == "" || request.SubjectDID == "" {
		return fmt.Errorf("invalid wallet presentation request")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyRequest := *request
	if copyRequest.CreatedAt.IsZero() {
		copyRequest.CreatedAt = time.Now().UTC()
	}
	if copyRequest.Status == "" {
		copyRequest.Status = WalletRequestPending
	}
	copyRequest.AllowedIssuerDIDs = append([]string(nil), request.AllowedIssuerDIDs...)
	m.walletRequests[copyRequest.ID] = &copyRequest
	*request = copyRequest
	return nil
}

func (m *MemoryStore) GetWalletPresentationRequestByID(id string) (*WalletPresentationRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	request, ok := m.walletRequests[id]
	if !ok {
		return nil, nil
	}
	copyRequest := *request
	copyRequest.AllowedIssuerDIDs = append([]string(nil), request.AllowedIssuerDIDs...)
	return &copyRequest, nil
}

func (m *MemoryStore) GetWalletPresentationRequests(subjectDID *string) ([]WalletPresentationRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []WalletPresentationRequest{}
	for _, request := range m.walletRequests {
		if subjectDID != nil && request.SubjectDID != *subjectDID {
			continue
		}
		copyRequest := *request
		copyRequest.AllowedIssuerDIDs = append([]string(nil), request.AllowedIssuerDIDs...)
		res = append(res, copyRequest)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res, nil
}

func (m *MemoryStore) UpdateWalletPresentationRequest(request *WalletPresentationRequest) error {
	if request == nil || request.ID == "" {
		return fmt.Errorf("invalid wallet presentation request")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.walletRequests[request.ID]; !ok {
		return fmt.Errorf("wallet presentation request not found")
	}
	copyRequest := *request
	copyRequest.AllowedIssuerDIDs = append([]string(nil), request.AllowedIssuerDIDs...)
	m.walletRequests[copyRequest.ID] = &copyRequest
	return nil
}

func (m *MemoryStore) CreateWalletPresentationVerification(verification *WalletPresentationVerification) error {
	if verification == nil || verification.ID == "" || verification.RequestID == "" {
		return fmt.Errorf("invalid wallet presentation verification")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyVerification := *verification
	if copyVerification.CreatedAt.IsZero() {
		copyVerification.CreatedAt = time.Now().UTC()
	}
	m.walletVerifications[copyVerification.RequestID] = append(m.walletVerifications[copyVerification.RequestID], copyVerification)
	*verification = copyVerification
	return nil
}

func (m *MemoryStore) GetLatestWalletPresentationVerification(requestID string) (*WalletPresentationVerification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	verifications := m.walletVerifications[requestID]
	if len(verifications) == 0 {
		return nil, nil
	}
	latest := verifications[0]
	for _, verification := range verifications[1:] {
		if verification.CreatedAt.After(latest.CreatedAt) {
			latest = verification
		}
	}
	copyVerification := latest
	return &copyVerification, nil
}

func (m *MemoryStore) CreateVerificationCase(verificationCase *VerificationCase) error {
	if verificationCase == nil || verificationCase.ID == "" || verificationCase.SubjectDID == "" {
		return fmt.Errorf("invalid verification case")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyCase := *verificationCase
	if copyCase.CreatedAt.IsZero() {
		copyCase.CreatedAt = time.Now().UTC()
	}
	if copyCase.Status == "" {
		copyCase.Status = VerificationSubmitted
	}
	m.verificationCases[copyCase.ID] = &copyCase
	*verificationCase = copyCase
	return nil
}

func (m *MemoryStore) GetVerificationCaseByID(id string) (*VerificationCase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	verificationCase, ok := m.verificationCases[id]
	if !ok {
		return nil, nil
	}
	copyCase := *verificationCase
	return &copyCase, nil
}

func (m *MemoryStore) GetVerificationCases(subjectDID *string, assignedVerifierDID *string) ([]VerificationCase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []VerificationCase{}
	for _, verificationCase := range m.verificationCases {
		if subjectDID != nil && verificationCase.SubjectDID != *subjectDID {
			continue
		}
		if assignedVerifierDID != nil && verificationCase.AssignedVerifierDID != *assignedVerifierDID {
			continue
		}
		res = append(res, *verificationCase)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res, nil
}

func (m *MemoryStore) UpdateVerificationCase(verificationCase *VerificationCase) error {
	if verificationCase == nil || verificationCase.ID == "" {
		return fmt.Errorf("invalid verification case")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.verificationCases[verificationCase.ID]; !ok {
		return fmt.Errorf("verification case not found")
	}
	copyCase := *verificationCase
	m.verificationCases[copyCase.ID] = &copyCase
	return nil
}

func (m *MemoryStore) CreateVerifierDecision(decision *VerifierDecision) error {
	if decision == nil || decision.ID == "" || decision.CaseID == "" || decision.VerifierDID == "" {
		return fmt.Errorf("invalid verifier decision")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyDecision := *decision
	if copyDecision.CreatedAt.IsZero() {
		copyDecision.CreatedAt = time.Now().UTC()
	}
	if copyDecision.CredentialIssuanceSource == "" {
		copyDecision.CredentialIssuanceSource = CredentialInternalVerifierIssued
	}
	m.verifierDecisions[copyDecision.CaseID] = append(m.verifierDecisions[copyDecision.CaseID], copyDecision)
	*decision = copyDecision
	return nil
}

func (m *MemoryStore) GetVerifierDecisionsByCaseID(caseID string) ([]VerifierDecision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	decisions := m.verifierDecisions[caseID]
	res := make([]VerifierDecision, 0, len(decisions))
	for _, decision := range decisions {
		res = append(res, decision)
	}
	return res, nil
}

func (m *MemoryStore) CreateTrustAuditLog(entry *TrustAuditLog) error {
	if entry == nil || entry.ID == "" || entry.ActorDID == "" || entry.ActionType == "" {
		return fmt.Errorf("invalid trust audit log")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copyEntry := *entry
	if copyEntry.CreatedAt.IsZero() {
		copyEntry.CreatedAt = time.Now().UTC()
	}
	m.trustAuditLogs = append(m.trustAuditLogs, copyEntry)
	*entry = copyEntry
	return nil
}

func (m *MemoryStore) GetTrustAuditLogs(subjectDID *string, limit int) ([]TrustAuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := []TrustAuditLog{}
	for i := len(m.trustAuditLogs) - 1; i >= 0 && len(res) < limit; i-- {
		entry := m.trustAuditLogs[i]
		if subjectDID != nil && entry.TargetDID != *subjectDID && entry.ActorDID != *subjectDID {
			continue
		}
		res = append(res, entry)
	}
	return res, nil
}
