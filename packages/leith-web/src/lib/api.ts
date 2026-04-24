import type { User } from '@/types/auth';

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface Post {
  id: string;
  body: string;
  mediaHashes?: string[];
  parentId?: string;
  timestamp: number;
  authorDid: string;
  signature: string;
  visibilityScore?: number;
}

export interface CreatePostRequest {
  body: string;
  mediaHashes: string[];
  parentId?: string;
  timestamp: number;
  authorDid: string;
  signature: string;
}

export type ContentMode = 'murmur' | 'idea' | 'discussion';
export type ContentVisibility = 'private' | 'unlisted' | 'public';
export type ParticipationPolicy = 'read_only' | 'comment' | 'debate';
export type DiscussionShape = 'thread' | 'stance_map' | 'debate_cards';

export interface CapabilitySnapshot {
  canCreateMurmur: boolean;
  canCreateIdea: boolean;
  canCreateDiscussion: boolean;
  canReplyToDiscussion: boolean;
  canForkDiscussion: boolean;
  canFlagContent: boolean;
  canSlashContent: boolean;
  canModerate: boolean;
  canRequestVerification: boolean;
  canReviewVerificationCases: boolean;
}

export interface Identity {
  did: string;
  displayName: string;
  trustTier: number;
  authMethod?: string;
}

export interface MeResponse {
  identity: Identity;
  capabilities: CapabilitySnapshot;
}

export interface PasskeyCeremonyOptions {
  sessionId: string;
  did?: string;
  publicKey: Record<string, unknown>;
}

export interface PasskeyCeremonyFinishRequest {
  sessionId: string;
  credential: Record<string, unknown>;
}

export interface ContentItem {
  id: string;
  authorDid: string;
  title?: string;
  body: string;
  mode: ContentMode;
  status: 'draft' | 'active' | 'archived';
  visibility: ContentVisibility;
  trustTier?: number;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string;
  participationPolicy?: ParticipationPolicy;
  discussionShape?: DiscussionShape;
  sourceContentIds?: string[];
}

export interface CreateContentItemRequest {
  title?: string;
  body: string;
  mode: ContentMode;
  visibility?: ContentVisibility;
  participationPolicy?: ParticipationPolicy;
  discussionShape?: DiscussionShape;
  sourceContentIds?: string[];
}

export interface DiscussionNode {
  id: string;
  discussionId: string;
  parentNodeId?: string;
  authorDid: string;
  nodeType: 'claim' | 'question' | 'evidence' | 'rebuttal' | 'summary';
  stance: 'support' | 'oppose' | 'clarify' | 'neutral';
  body: string;
  createdAt: string;
}

export interface Projection {
  id: string;
  sourceIdeaId: string;
  targetDiscussionId: string;
  projectedExcerpt: string;
  participationPolicy: ParticipationPolicy;
  ownershipTransferAcknowledged: boolean;
  createdByDid: string;
  createdAt: string;
}

export interface CreateProjectionRequest {
  sourceIdeaId: string;
  projectedExcerpt: string;
  participationPolicy: ParticipationPolicy;
  ownershipTransferAcknowledged: boolean;
  discussionShape?: DiscussionShape;
}

export type TransformationProviderType = 'local_llm' | 'byok' | 'system_llm';
export type TransformationStatus = 'queued' | 'running' | 'completed' | 'failed' | 'discarded' | 'published';

export interface TransformationJob {
  id: string;
  requestedByDid: string;
  sourceContentIds: string[];
  targetMode: ContentMode;
  providerType: TransformationProviderType;
  promptProfile: string;
  status: TransformationStatus;
  outputTitle?: string;
  outputBody?: string;
  createdAt: string;
  completedAt?: string;
  publishedContentId?: string;
}

export interface CreateTransformationJobRequest {
  sourceContentIds: string[];
  targetMode: ContentMode;
  providerType: TransformationProviderType;
  promptProfile?: string;
}

export interface DiscussionFork {
  id: string;
  sourceDiscussionId: string;
  forkDiscussionId: string;
  createdByDid: string;
  reason: string;
  createdAt: string;
}

export interface CreateDiscussionForkRequest {
  reason: string;
}

export type ModerationActionType = 'flag' | 'hide' | 'lock' | 'slash';

export interface ModerationAction {
  id: string;
  targetContentId: string;
  actionType: ModerationActionType;
  reason: string;
  initiatedByDid: string;
  requiredTier: number;
  status: 'open' | 'approved';
  createdAt: string;
}

export interface CreateModerationActionRequest {
  targetContentId: string;
  actionType: ModerationActionType;
  reason: string;
}

export type CredentialStatus = 'active' | 'expired' | 'revoked' | 'rejected';

export interface Credential {
  id: string;
  subjectDid: string;
  issuerDid: string;
  externalIssuerDid?: string;
  credentialType: string;
  claimsJson: string;
  status: CredentialStatus;
  issuanceSource: 'internal_verifier_issued' | 'external_issuer_verified';
  proof?: string;
  issuedAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface TrustAssessment {
  id: string;
  subjectDid: string;
  tier: number;
  source: 'system' | 'verifier' | 'credential' | 'social_graph' | 'moderation';
  score: number;
  evidenceRefs?: string[];
  issuedByDid: string;
  effectiveAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface Verifier {
  id: string;
  verifierDid: string;
  verifierType: string;
  scope: string;
  authorityLevel: number;
  status: 'active' | 'suspended' | 'revoked' | 'expired';
  appointedByDid: string;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface VerificationCase {
  id: string;
  subjectDid: string;
  requestedTier: number;
  credentialType: string;
  evidenceJson: string;
  status: 'submitted' | 'assigned' | 'approved' | 'rejected' | 'appealed' | 'expired';
  assignedVerifierDid?: string;
  decision?: string;
  decisionReason?: string;
  createdAt: string;
  decidedAt?: string;
}

export interface CreateVerificationCaseRequest {
  requestedTier: number;
  credentialType: string;
  evidenceJson: string;
}

export type VerifierDecisionValue = 'approve' | 'reject' | 'request_more_evidence';

export interface VerifierDecision {
  id: string;
  caseId: string;
  verifierDid: string;
  decision: VerifierDecisionValue;
  reason: string;
  credentialIssuanceSource: 'internal_verifier_issued' | 'external_issuer_verified';
  externalIssuerDid?: string;
  issuedAssessmentId?: string;
  issuedCredentialId?: string;
  createdAt: string;
}

export interface CreateVerifierDecisionRequest {
  decision: VerifierDecisionValue;
  reason: string;
  credentialIssuanceSource: 'internal_verifier_issued' | 'external_issuer_verified';
  externalIssuerDid?: string;
}

export interface TrustAuditLog {
  id: string;
  actorDid: string;
  targetDid?: string;
  targetResourceId?: string;
  actionType: string;
  metadataJson?: string;
  createdAt: string;
}

export interface TrustProfile {
  identity: Identity;
  capabilities: CapabilitySnapshot;
  credentials: Credential[];
  trustAssessments: TrustAssessment[];
  verificationCases: VerificationCase[];
  auditLogs: TrustAuditLog[];
  verifier?: Verifier;
}

export interface TrustedIssuer {
  id: string;
  issuerDid: string;
  issuerName: string;
  status: 'active' | 'suspended' | 'revoked' | 'expired';
  scopes: string[];
  credentialTypes: string[];
  maxTrustTierIssued: number;
  appointedByDid: string;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface CreateTrustedIssuerRequest {
  issuerDid: string;
  issuerName: string;
  scopes: string[];
  credentialTypes: string[];
  maxTrustTierIssued: number;
  status?: TrustedIssuer['status'];
  expiresAt?: string;
}

export interface WalletPresentationVerification {
  id: string;
  requestId: string;
  subjectDid: string;
  issuerDid: string;
  credentialType: string;
  presentationFormat: string;
  claimsJson: string;
  proof: string;
  audience: string;
  nonce: string;
  status: 'verified' | 'rejected';
  trustedIssuerDid?: string;
  notes?: string;
  issuedCredentialId?: string;
  issuedAssessmentId?: string;
  createdAt: string;
  verifiedAt?: string;
}

export interface WalletPresentationRequest {
  id: string;
  subjectDid: string;
  verifierDid: string;
  requestedTier: number;
  credentialType: string;
  purpose?: string;
  allowedIssuerDids: string[];
  challenge: string;
  requestUri: string;
  qrPayload: string;
  status: 'pending' | 'completed' | 'verified' | 'rejected' | 'expired';
  createdAt: string;
  expiresAt?: string;
  completedAt?: string;
  verification?: WalletPresentationVerification;
}

export interface CreateWalletPresentationRequest {
  requestedTier: number;
  credentialType: string;
  purpose?: string;
  allowedIssuerDids?: string[];
  expiresInMinutes?: number;
}

export interface CompleteWalletPresentationRequest {
  issuerDid: string;
  credentialType: string;
  presentationFormat: string;
  claimsJson: string;
  proof: string;
  audience: string;
  nonce: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface DiscussionNodesResponse {
  discussionId: string;
  nodes: DiscussionNode[];
}

export interface CreateDiscussionNodeRequest {
  parentNodeId?: string;
  nodeType: DiscussionNode['nodeType'];
  stance: DiscussionNode['stance'];
  body: string;
}

export function buildAuthHeaders(user?: User | null): HeadersInit {
  if (!user) {
    return {};
  }
  const trustTierHeader = { 'X-Mock-Trust-Tier': String(user.level) };
  if (user.authMethod === 'oauth' && user.oauthToken) {
    return {
      'X-Mock-OAuth-Token': user.oauthToken,
      ...trustTierHeader,
    };
  }
  return {
    'X-Mock-DID': user.address,
    ...trustTierHeader,
  };
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    cache: 'no-store',
    ...init,
  });
  if (!res.ok) {
    throw new Error(`API request failed: ${res.status}`);
  }
  return res.json();
}

export async function loginAsGuestOAuth(provider: string, token: string): Promise<{ did: string; trustTier: number }> {
  return fetchJSON('/auth/oauth', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ provider, token }),
  });
}

export async function beginPasskeyRegistration(): Promise<PasskeyCeremonyOptions> {
  return fetchJSON('/auth/register/options');
}

export async function finishPasskeyRegistration(req: PasskeyCeremonyFinishRequest): Promise<{ did: string; trustTier: number }> {
  return fetchJSON('/auth/register', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(req),
  });
}

export async function beginPasskeyLogin(did: string): Promise<PasskeyCeremonyOptions> {
  const query = new URLSearchParams({ did });
  return fetchJSON(`/auth/login/options?${query.toString()}`);
}

export async function finishPasskeyLogin(req: PasskeyCeremonyFinishRequest): Promise<{ did: string; trustTier: number }> {
  return fetchJSON('/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(req),
  });
}

export async function fetchMe(user?: User | null): Promise<MeResponse> {
  return fetchJSON('/v2/me', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function fetchContentItems(params: {
  mode?: ContentMode;
  visibility?: ContentVisibility;
  authorDid?: string;
  limit?: number;
  offset?: number;
}, user?: User | null): Promise<ContentItem[]> {
  const query = new URLSearchParams();
  if (params.mode) query.set('mode', params.mode);
  if (params.visibility) query.set('visibility', params.visibility);
  if (params.authorDid) query.set('authorDid', params.authorDid);
  query.set('limit', String(params.limit ?? 50));
  query.set('offset', String(params.offset ?? 0));
  return fetchJSON(`/v2/content-items?${query.toString()}`, {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createContentItem(req: CreateContentItemRequest, user?: User | null): Promise<ContentItem> {
  return fetchJSON('/v2/content-items', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchDiscussionNodes(discussionId: string, user?: User | null): Promise<DiscussionNodesResponse> {
  return fetchJSON(`/v2/discussions/${discussionId}/nodes`, {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createDiscussionNode(discussionId: string, req: CreateDiscussionNodeRequest, user?: User | null): Promise<DiscussionNode> {
  return fetchJSON(`/v2/discussions/${discussionId}/nodes`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function createProjection(req: CreateProjectionRequest, user?: User | null): Promise<Projection> {
  return fetchJSON('/v2/projections', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function createTransformationJob(req: CreateTransformationJobRequest, user?: User | null): Promise<TransformationJob> {
  return fetchJSON('/v2/transformation-jobs', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchTransformationJob(jobId: string, user?: User | null): Promise<TransformationJob> {
  return fetchJSON(`/v2/transformation-jobs/${jobId}`, {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function publishTransformationJob(jobId: string, user?: User | null): Promise<ContentItem> {
  return fetchJSON(`/v2/transformation-jobs/${jobId}/publish`, {
    method: 'POST',
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createDiscussionFork(discussionId: string, req: CreateDiscussionForkRequest, user?: User | null): Promise<DiscussionFork> {
  return fetchJSON(`/v2/discussions/${discussionId}/forks`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function createModerationAction(req: CreateModerationActionRequest, user?: User | null): Promise<ModerationAction> {
  return fetchJSON('/v2/moderation-actions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchTrustProfile(user?: User | null): Promise<TrustProfile> {
  return fetchJSON('/v2/trust/me', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createVerificationCase(req: CreateVerificationCaseRequest, user?: User | null): Promise<VerificationCase> {
  return fetchJSON('/v2/trust/verification-cases', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchVerifierCases(user?: User | null): Promise<VerificationCase[]> {
  return fetchJSON('/v2/trust/verifier/cases', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createVerifierDecision(caseId: string, req: CreateVerifierDecisionRequest, user?: User | null): Promise<VerifierDecision> {
  return fetchJSON(`/v2/trust/verifier/cases/${caseId}/decision`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchVerifiers(user?: User | null): Promise<Verifier[]> {
  return fetchJSON('/v2/trust/verifiers', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function fetchTrustedIssuers(user?: User | null): Promise<TrustedIssuer[]> {
  return fetchJSON('/v2/trust/issuers', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createTrustedIssuer(req: CreateTrustedIssuerRequest, user?: User | null): Promise<TrustedIssuer> {
  return fetchJSON('/v2/trust/issuers', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchWalletPresentationRequests(user?: User | null): Promise<WalletPresentationRequest[]> {
  return fetchJSON('/v2/trust/presentation-requests', {
    headers: {
      ...buildAuthHeaders(user),
    },
  });
}

export async function createWalletPresentationRequest(
  req: CreateWalletPresentationRequest,
  user?: User | null,
): Promise<WalletPresentationRequest> {
  return fetchJSON('/v2/trust/presentation-requests', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function completeWalletPresentationRequest(
  requestId: string,
  req: CompleteWalletPresentationRequest,
  user?: User | null,
): Promise<WalletPresentationVerification> {
  return fetchJSON(`/v2/trust/presentation-requests/${requestId}/complete`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
}

export async function fetchPosts(limit = 20, offset = 0): Promise<Post[]> {
  const res = await fetch(`${API_BASE_URL}/posts?limit=${limit}&offset=${offset}`, {
    cache: 'no-store',
  });
  if (!res.ok) {
    throw new Error('Failed to fetch posts');
  }
  return res.json();
}

export async function createPost(req: CreatePostRequest, user?: User | null): Promise<Post> {
  const res = await fetch(`${API_BASE_URL}/posts`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...buildAuthHeaders(user),
    },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    throw new Error('Failed to create post');
  }
  return res.json();
}
