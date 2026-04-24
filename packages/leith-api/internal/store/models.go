package store

import (
	"database/sql"
	"time"
)

// TrustTier represents the L1-L4 user trust levels
type TrustTier int

const (
	L0_GUEST     TrustTier = 0 // OAuth readers (Zero voting weight)
	L1_DEVICE    TrustTier = 1 // Basic hardware validation
	L2_SOCIAL    TrustTier = 2 // Social guarantees
	L3_TEMPORAL  TrustTier = 3 // Time accumulated
	L4_AUTHORITY TrustTier = 4 // Authority credential
)

// User represents the identity and passkey information
type User struct {
	ID                 int64     `db:"id" json:"id"`
	DID                string    `db:"did" json:"did"`      // Primary identity e.g., did:vflow:{pubkey} or oauth:google:{id}
	OAuthID            string    `db:"oauth_id" json:"-"`   // Only used if L0 Guest
	PublicKey          []byte    `db:"public_key" json:"-"` // Ed25519 public key (empty if L0)
	AuthnUserID        []byte    `db:"authn_user_id" json:"-"`
	PasskeyCredentials []byte    `db:"passkey_credentials" json:"-"`
	TrustTier          TrustTier `db:"trust_tier" json:"trustTier"`
	CreatedAt          time.Time `db:"created_at" json:"createdAt"`
}

// Post represents a signed content payload
type Post struct {
	ID              int64         `db:"id" json:"id"`
	Body            string        `db:"body" json:"body"`
	MediaHashes     []string      `db:"media_hashes" json:"mediaHashes"`     // pHash array
	ParentID        sql.NullInt64 `db:"parent_id" json:"parentId,omitempty"` // For threads/vault links
	Timestamp       int64         `db:"timestamp" json:"timestamp"`          // Unix Epoch from client
	AuthorDID       string        `db:"author_did" json:"authorDid"`
	Signature       string        `db:"signature" json:"signature"` // Hex signature
	VisibilityScore float64       `db:"visibility_score" json:"visibilityScore"`
	CreatedAt       time.Time     `db:"created_at" json:"createdAt"`
}

type ContentMode string

const (
	ModeMurmur     ContentMode = "murmur"
	ModeIdea       ContentMode = "idea"
	ModeDiscussion ContentMode = "discussion"
)

type ContentStatus string

const (
	StatusDraft    ContentStatus = "draft"
	StatusActive   ContentStatus = "active"
	StatusArchived ContentStatus = "archived"
)

type ContentVisibility string

const (
	VisibilityPrivate  ContentVisibility = "private"
	VisibilityUnlisted ContentVisibility = "unlisted"
	VisibilityPublic   ContentVisibility = "public"
)

type ParticipationPolicy string

const (
	ParticipationReadOnly ParticipationPolicy = "read_only"
	ParticipationComment  ParticipationPolicy = "comment"
	ParticipationDebate   ParticipationPolicy = "debate"
)

type DiscussionShape string

const (
	DiscussionThread      DiscussionShape = "thread"
	DiscussionStanceMap   DiscussionShape = "stance_map"
	DiscussionDebateCards DiscussionShape = "debate_cards"
)

type ContentItem struct {
	ID                  string              `json:"id"`
	AuthorDID           string              `json:"authorDid"`
	Title               string              `json:"title,omitempty"`
	Body                string              `json:"body"`
	Mode                ContentMode         `json:"mode"`
	Status              ContentStatus       `json:"status"`
	Visibility          ContentVisibility   `json:"visibility"`
	TrustTier           TrustTier           `json:"trustTier"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
	PublishedAt         *time.Time          `json:"publishedAt,omitempty"`
	ParticipationPolicy ParticipationPolicy `json:"participationPolicy,omitempty"`
	DiscussionShape     DiscussionShape     `json:"discussionShape,omitempty"`
	SourceContentIDs    []string            `json:"sourceContentIds,omitempty"`
}

type DiscussionNodeType string

const (
	NodeClaim    DiscussionNodeType = "claim"
	NodeQuestion DiscussionNodeType = "question"
	NodeEvidence DiscussionNodeType = "evidence"
	NodeRebuttal DiscussionNodeType = "rebuttal"
	NodeSummary  DiscussionNodeType = "summary"
)

type DiscussionStance string

const (
	StanceSupport DiscussionStance = "support"
	StanceOppose  DiscussionStance = "oppose"
	StanceClarify DiscussionStance = "clarify"
	StanceNeutral DiscussionStance = "neutral"
)

type DiscussionNode struct {
	ID           string             `json:"id"`
	DiscussionID string             `json:"discussionId"`
	ParentNodeID *string            `json:"parentNodeId,omitempty"`
	AuthorDID    string             `json:"authorDid"`
	NodeType     DiscussionNodeType `json:"nodeType"`
	Stance       DiscussionStance   `json:"stance"`
	Body         string             `json:"body"`
	CreatedAt    time.Time          `json:"createdAt"`
}

type ContentRelationType string

const (
	RelationProjectedFrom ContentRelationType = "projected_from"
	RelationExpandedFrom  ContentRelationType = "expanded_from"
	RelationForkedFrom    ContentRelationType = "forked_from"
)

type ContentRelation struct {
	ID            string              `json:"id"`
	FromContentID string              `json:"fromContentId"`
	ToContentID   string              `json:"toContentId"`
	RelationType  ContentRelationType `json:"relationType"`
	CreatedAt     time.Time           `json:"createdAt"`
}

type Projection struct {
	ID                            string              `json:"id"`
	SourceIdeaID                  string              `json:"sourceIdeaId"`
	TargetDiscussionID            string              `json:"targetDiscussionId"`
	ProjectedExcerpt              string              `json:"projectedExcerpt"`
	ParticipationPolicy           ParticipationPolicy `json:"participationPolicy"`
	OwnershipTransferAcknowledged bool                `json:"ownershipTransferAcknowledged"`
	CreatedByDID                  string              `json:"createdByDid"`
	CreatedAt                     time.Time           `json:"createdAt"`
}

type TransformationProviderType string

const (
	ProviderLocalLLM  TransformationProviderType = "local_llm"
	ProviderBYOK      TransformationProviderType = "byok"
	ProviderSystemLLM TransformationProviderType = "system_llm"
)

type TransformationStatus string

const (
	TransformationQueued    TransformationStatus = "queued"
	TransformationRunning   TransformationStatus = "running"
	TransformationCompleted TransformationStatus = "completed"
	TransformationFailed    TransformationStatus = "failed"
	TransformationDiscarded TransformationStatus = "discarded"
	TransformationPublished TransformationStatus = "published"
)

type TransformationJob struct {
	ID                 string                     `json:"id"`
	RequestedByDID     string                     `json:"requestedByDid"`
	SourceContentIDs   []string                   `json:"sourceContentIds"`
	TargetMode         ContentMode                `json:"targetMode"`
	ProviderType       TransformationProviderType `json:"providerType"`
	PromptProfile      string                     `json:"promptProfile"`
	Status             TransformationStatus       `json:"status"`
	OutputTitle        string                     `json:"outputTitle,omitempty"`
	OutputBody         string                     `json:"outputBody,omitempty"`
	CreatedAt          time.Time                  `json:"createdAt"`
	CompletedAt        *time.Time                 `json:"completedAt,omitempty"`
	PublishedContentID *string                    `json:"publishedContentId,omitempty"`
}

type DiscussionFork struct {
	ID                 string    `json:"id"`
	SourceDiscussionID string    `json:"sourceDiscussionId"`
	ForkDiscussionID   string    `json:"forkDiscussionId"`
	CreatedByDID       string    `json:"createdByDid"`
	Reason             string    `json:"reason"`
	CreatedAt          time.Time `json:"createdAt"`
}

type ModerationActionType string

const (
	ModerationFlag  ModerationActionType = "flag"
	ModerationHide  ModerationActionType = "hide"
	ModerationLock  ModerationActionType = "lock"
	ModerationSlash ModerationActionType = "slash"
)

type ModerationActionStatus string

const (
	ModerationOpen     ModerationActionStatus = "open"
	ModerationApproved ModerationActionStatus = "approved"
)

type ModerationAction struct {
	ID              string                 `json:"id"`
	TargetContentID string                 `json:"targetContentId"`
	ActionType      ModerationActionType   `json:"actionType"`
	Reason          string                 `json:"reason"`
	InitiatedByDID  string                 `json:"initiatedByDid"`
	RequiredTier    TrustTier              `json:"requiredTier"`
	Status          ModerationActionStatus `json:"status"`
	CreatedAt       time.Time              `json:"createdAt"`
}

type CredentialStatus string

const (
	CredentialActive   CredentialStatus = "active"
	CredentialExpired  CredentialStatus = "expired"
	CredentialRevoked  CredentialStatus = "revoked"
	CredentialRejected CredentialStatus = "rejected"
)

type CredentialIssuanceSource string

const (
	CredentialInternalVerifierIssued CredentialIssuanceSource = "internal_verifier_issued"
	CredentialExternalIssuerVerified CredentialIssuanceSource = "external_issuer_verified"
)

type Credential struct {
	ID                string                   `json:"id"`
	SubjectDID        string                   `json:"subjectDid"`
	IssuerDID         string                   `json:"issuerDid"`
	ExternalIssuerDID string                   `json:"externalIssuerDid,omitempty"`
	CredentialType    string                   `json:"credentialType"`
	ClaimsJSON        string                   `json:"claimsJson"`
	Proof             string                   `json:"proof,omitempty"`
	Status            CredentialStatus         `json:"status"`
	IssuanceSource    CredentialIssuanceSource `json:"issuanceSource"`
	IssuedAt          time.Time                `json:"issuedAt"`
	ExpiresAt         *time.Time               `json:"expiresAt,omitempty"`
	RevokedAt         *time.Time               `json:"revokedAt,omitempty"`
}

type TrustAssessmentSource string

const (
	TrustSourceSystem      TrustAssessmentSource = "system"
	TrustSourceVerifier    TrustAssessmentSource = "verifier"
	TrustSourceCredential  TrustAssessmentSource = "credential"
	TrustSourceSocialGraph TrustAssessmentSource = "social_graph"
	TrustSourceModeration  TrustAssessmentSource = "moderation"
)

type TrustAssessment struct {
	ID           string                `json:"id"`
	SubjectDID   string                `json:"subjectDid"`
	Tier         TrustTier             `json:"tier"`
	Score        float64               `json:"score"`
	Source       TrustAssessmentSource `json:"source"`
	EvidenceRefs []string              `json:"evidenceRefs,omitempty"`
	IssuedByDID  string                `json:"issuedByDid"`
	EffectiveAt  time.Time             `json:"effectiveAt"`
	ExpiresAt    *time.Time            `json:"expiresAt,omitempty"`
	RevokedAt    *time.Time            `json:"revokedAt,omitempty"`
}

type VerifierStatus string

const (
	VerifierActive    VerifierStatus = "active"
	VerifierSuspended VerifierStatus = "suspended"
	VerifierRevoked   VerifierStatus = "revoked"
	VerifierExpired   VerifierStatus = "expired"
)

type Verifier struct {
	ID             string         `json:"id"`
	VerifierDID    string         `json:"verifierDid"`
	VerifierType   string         `json:"verifierType"`
	Scope          string         `json:"scope"`
	AuthorityLevel TrustTier      `json:"authorityLevel"`
	Status         VerifierStatus `json:"status"`
	AppointedByDID string         `json:"appointedByDid"`
	CreatedAt      time.Time      `json:"createdAt"`
	ExpiresAt      *time.Time     `json:"expiresAt,omitempty"`
	RevokedAt      *time.Time     `json:"revokedAt,omitempty"`
}

type TrustedIssuerStatus string

const (
	TrustedIssuerActive    TrustedIssuerStatus = "active"
	TrustedIssuerSuspended TrustedIssuerStatus = "suspended"
	TrustedIssuerRevoked   TrustedIssuerStatus = "revoked"
	TrustedIssuerExpired   TrustedIssuerStatus = "expired"
)

type TrustedIssuer struct {
	ID                 string              `json:"id"`
	IssuerDID          string              `json:"issuerDid"`
	IssuerName         string              `json:"issuerName"`
	Status             TrustedIssuerStatus `json:"status"`
	Scopes             []string            `json:"scopes"`
	CredentialTypes    []string            `json:"credentialTypes"`
	MaxTrustTierIssued TrustTier           `json:"maxTrustTierIssued"`
	AppointedByDID     string              `json:"appointedByDid"`
	CreatedAt          time.Time           `json:"createdAt"`
	ExpiresAt          *time.Time          `json:"expiresAt,omitempty"`
	RevokedAt          *time.Time          `json:"revokedAt,omitempty"`
}

type WalletPresentationRequestStatus string

const (
	WalletRequestPending   WalletPresentationRequestStatus = "pending"
	WalletRequestCompleted WalletPresentationRequestStatus = "completed"
	WalletRequestVerified  WalletPresentationRequestStatus = "verified"
	WalletRequestRejected  WalletPresentationRequestStatus = "rejected"
	WalletRequestExpired   WalletPresentationRequestStatus = "expired"
)

type WalletPresentationRequest struct {
	ID                string                          `json:"id"`
	SubjectDID        string                          `json:"subjectDid"`
	VerifierDID       string                          `json:"verifierDid"`
	RequestedTier     TrustTier                       `json:"requestedTier"`
	CredentialType    string                          `json:"credentialType"`
	Purpose           string                          `json:"purpose,omitempty"`
	AllowedIssuerDIDs []string                        `json:"allowedIssuerDids,omitempty"`
	Challenge         string                          `json:"challenge"`
	RequestURI        string                          `json:"requestUri"`
	QRPayload         string                          `json:"qrPayload"`
	Status            WalletPresentationRequestStatus `json:"status"`
	CreatedAt         time.Time                       `json:"createdAt"`
	ExpiresAt         *time.Time                      `json:"expiresAt,omitempty"`
	CompletedAt       *time.Time                      `json:"completedAt,omitempty"`
}

type WalletPresentationVerificationStatus string

const (
	WalletVerificationVerified WalletPresentationVerificationStatus = "verified"
	WalletVerificationRejected WalletPresentationVerificationStatus = "rejected"
)

type WalletPresentationVerification struct {
	ID                 string                               `json:"id"`
	RequestID          string                               `json:"requestId"`
	SubjectDID         string                               `json:"subjectDid"`
	IssuerDID          string                               `json:"issuerDid"`
	CredentialType     string                               `json:"credentialType"`
	PresentationFormat string                               `json:"presentationFormat"`
	ClaimsJSON         string                               `json:"claimsJson"`
	Proof              string                               `json:"proof"`
	Audience           string                               `json:"audience"`
	Nonce              string                               `json:"nonce"`
	Status             WalletPresentationVerificationStatus `json:"status"`
	TrustedIssuerDID   string                               `json:"trustedIssuerDid,omitempty"`
	Notes              string                               `json:"notes,omitempty"`
	IssuedCredentialID *string                              `json:"issuedCredentialId,omitempty"`
	IssuedAssessmentID *string                              `json:"issuedAssessmentId,omitempty"`
	CreatedAt          time.Time                            `json:"createdAt"`
	VerifiedAt         *time.Time                           `json:"verifiedAt,omitempty"`
}

type VerificationCaseStatus string

const (
	VerificationSubmitted VerificationCaseStatus = "submitted"
	VerificationAssigned  VerificationCaseStatus = "assigned"
	VerificationApproved  VerificationCaseStatus = "approved"
	VerificationRejected  VerificationCaseStatus = "rejected"
	VerificationAppealed  VerificationCaseStatus = "appealed"
	VerificationExpired   VerificationCaseStatus = "expired"
)

type VerificationCase struct {
	ID                  string                 `json:"id"`
	SubjectDID          string                 `json:"subjectDid"`
	RequestedTier       TrustTier              `json:"requestedTier"`
	CredentialType      string                 `json:"credentialType"`
	EvidenceJSON        string                 `json:"evidenceJson"`
	Status              VerificationCaseStatus `json:"status"`
	AssignedVerifierDID string                 `json:"assignedVerifierDid,omitempty"`
	Decision            string                 `json:"decision,omitempty"`
	DecisionReason      string                 `json:"decisionReason,omitempty"`
	CreatedAt           time.Time              `json:"createdAt"`
	DecidedAt           *time.Time             `json:"decidedAt,omitempty"`
}

type VerifierDecisionType string

const (
	VerifierDecisionApprove             VerifierDecisionType = "approve"
	VerifierDecisionReject              VerifierDecisionType = "reject"
	VerifierDecisionRequestMoreEvidence VerifierDecisionType = "request_more_evidence"
)

type VerifierDecision struct {
	ID                       string                   `json:"id"`
	CaseID                   string                   `json:"caseId"`
	VerifierDID              string                   `json:"verifierDid"`
	Decision                 VerifierDecisionType     `json:"decision"`
	Reason                   string                   `json:"reason"`
	CredentialIssuanceSource CredentialIssuanceSource `json:"credentialIssuanceSource,omitempty"`
	ExternalIssuerDID        string                   `json:"externalIssuerDid,omitempty"`
	IssuedCredentialID       *string                  `json:"issuedCredentialId,omitempty"`
	IssuedAssessmentID       *string                  `json:"issuedAssessmentId,omitempty"`
	CreatedAt                time.Time                `json:"createdAt"`
}

type TrustAuditLog struct {
	ID               string    `json:"id"`
	ActorDID         string    `json:"actorDid"`
	ActionType       string    `json:"actionType"`
	TargetDID        string    `json:"targetDid,omitempty"`
	TargetResourceID string    `json:"targetResourceId,omitempty"`
	MetadataJSON     string    `json:"metadataJson,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Store Interface to define DB interactions
type Store interface {
	CreateUser(user *User) error
	GetUserByDID(did string) (*User, error)
	CreatePost(post *Post) error
	GetPosts(limit, offset int) ([]Post, error)
	UpdateVisibilityScore(postID int64, score float64) error
	CreateContentItem(item *ContentItem) error
	GetContentItemByID(id string) (*ContentItem, error)
	GetContentItems(mode *ContentMode, visibility *ContentVisibility, authorDID *string, limit, offset int) ([]ContentItem, error)
	CreateProjection(projection *Projection) error
	GetProjectionsBySourceIdeaID(sourceIdeaID string) ([]Projection, error)
	CreateContentRelation(relation *ContentRelation) error
	GetContentRelations(contentID string) ([]ContentRelation, error)
	CreateTransformationJob(job *TransformationJob) error
	GetTransformationJobByID(id string) (*TransformationJob, error)
	UpdateTransformationJob(job *TransformationJob) error
	CreateDiscussionFork(fork *DiscussionFork) error
	GetDiscussionForks(sourceDiscussionID string) ([]DiscussionFork, error)
	CreateModerationAction(action *ModerationAction) error
	GetModerationActions(targetContentID string) ([]ModerationAction, error)
	CreateDiscussionNode(node *DiscussionNode) error
	GetDiscussionNodes(discussionID string) ([]DiscussionNode, error)
	CreateCredential(credential *Credential) error
	GetCredentialsBySubjectDID(subjectDID string) ([]Credential, error)
	CreateTrustAssessment(assessment *TrustAssessment) error
	GetTrustAssessmentsBySubjectDID(subjectDID string) ([]TrustAssessment, error)
	CreateVerifier(verifier *Verifier) error
	GetVerifierByDID(did string) (*Verifier, error)
	GetVerifiers() ([]Verifier, error)
	CreateTrustedIssuer(issuer *TrustedIssuer) error
	GetTrustedIssuerByDID(did string) (*TrustedIssuer, error)
	GetTrustedIssuers() ([]TrustedIssuer, error)
	CreateWalletPresentationRequest(request *WalletPresentationRequest) error
	GetWalletPresentationRequestByID(id string) (*WalletPresentationRequest, error)
	GetWalletPresentationRequests(subjectDID *string) ([]WalletPresentationRequest, error)
	UpdateWalletPresentationRequest(request *WalletPresentationRequest) error
	CreateWalletPresentationVerification(verification *WalletPresentationVerification) error
	GetLatestWalletPresentationVerification(requestID string) (*WalletPresentationVerification, error)
	CreateVerificationCase(verificationCase *VerificationCase) error
	GetVerificationCaseByID(id string) (*VerificationCase, error)
	GetVerificationCases(subjectDID *string, assignedVerifierDID *string) ([]VerificationCase, error)
	UpdateVerificationCase(verificationCase *VerificationCase) error
	CreateVerifierDecision(decision *VerifierDecision) error
	GetVerifierDecisionsByCaseID(caseID string) ([]VerifierDecision, error)
	CreateTrustAuditLog(entry *TrustAuditLog) error
	GetTrustAuditLogs(subjectDID *string, limit int) ([]TrustAuditLog, error)
	// Rate Limiting helpers
	CheckRateLimit(did string, tier TrustTier) (bool, error)
}
