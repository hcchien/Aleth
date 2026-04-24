# Aleth Trust and Verifier System

## Purpose

Aleth is built around certified human participation. DID and passkeys provide a
cryptographic identity anchor, but they do not by themselves prove real-world
trust, professional standing, social legitimacy, or governance authority.

This document defines how Aleth should evolve from device-bound identity into a
multi-layer trust system with verifiers, credentials, auditability, and
progressive capabilities.

## Core Principle

```text
DID proves continuity.
Credentials provide evidence.
Verifiers make accountable assessments.
Trust tiers grant scoped capability.
```

DID is the substrate, not the full trust model.

## Trust Layers

### L0: Guest / OAuth

Meaning:

- User has a temporary or low-assurance identity.
- Identity may be backed by OAuth or a mock session.

Capabilities:

- read public content
- create low-impact public content if rate-limited
- cannot verify others
- cannot open high-trust Messenger flows

### L1: Device-Bound DID / Passkey

Meaning:

- User controls a stable DID or passkey-bound account.
- The system can recognize the same cryptographic identity over time.

Capabilities:

- create private/personal content
- create basic public content
- message existing friends only
- request higher verification

Limitations:

- does not prove real-world personhood
- does not prove professional credentials
- cannot perform governance actions

### L2: Community Known

Meaning:

- User has accumulated enough social or community evidence to be treated as a
  known participant.

Evidence examples:

- accepted friend relationships
- shared board membership
- positive contribution history
- endorsements from L2+ users
- low abuse/report history

Capabilities:

- request messages with same-board members
- fork public discussions
- flag public content
- participate in richer board workflows

### L3: Credentialed

Meaning:

- User has one or more verified credentials or stronger evidence from a trusted
  issuer or verifier.

Evidence examples:

- organization membership
- professional license
- domain ownership
- institution affiliation
- board-specific expertise proof
- external verifiable credential

Capabilities:

- create expert-context rooms
- higher weighting in board-specific discourse
- issue limited endorsements if policy allows
- access board workflows requiring credentialed participation

### L4: Verifier / Governance Actor

Meaning:

- User is authorized to review verification cases, issue trust assessments, or
  perform scoped governance actions.

Capabilities:

- review verification cases
- approve or reject L2/L3 requests
- issue platform credentials or trust assessments
- create governance rooms
- propose high-impact moderation actions such as slash, lock, or hide

Limitations:

- verifier authority must be scoped
- verifier actions must be audited
- verifier status can expire or be revoked
- verifier cannot be treated as an unrestricted superuser

## DID vs Trust

### DID can prove

- stable cryptographic identity
- continuity across sessions
- ownership of a private key or passkey credential
- ability to sign future actions

### DID cannot prove alone

- that the user is a unique human
- that the user has a profession or institution role
- that the user deserves governance authority
- that the account has not been socially abusive
- that a credential issuer is legitimate

Therefore, every trust tier above L1 should reference evidence beyond the DID
itself.

## Domain Model

### `Identity`

Represents the base account or participant.

Fields:

- `id`
- `did`
- `display_name`
- `avatar_url`
- `account_status`: `active | suspended | disabled`
- `created_at`
- `updated_at`

### `DeviceCredential`

Represents a device-bound cryptographic credential.

Fields:

- `id`
- `identity_did`
- `credential_type`: `passkey | did_key | recovery_key`
- `public_key`
- `device_label`
- `created_at`
- `last_used_at`
- `revoked_at`

### `Credential`

Represents evidence attached to an identity.

Fields:

- `id`
- `subject_did`
- `issuer_did`
- `credential_type`
- `claims_json`
- `proof`
- `status`: `active | expired | revoked | rejected`
- `issued_at`
- `expires_at`
- `revoked_at`

Examples:

- `personhood_attestation`
- `organization_membership`
- `professional_license`
- `domain_ownership`
- `board_expertise`
- `community_endorsement`

### `TrustAssessment`

Represents a decision that contributes to a user's trust tier.

Fields:

- `id`
- `subject_did`
- `tier`
- `score`
- `source`: `system | verifier | credential | social_graph | moderation`
- `evidence_refs`
- `issued_by_did`
- `effective_at`
- `expires_at`
- `revoked_at`

Notes:

- A user can have many assessments.
- The current effective trust tier is derived from active assessments and
  policy.

### `Verifier`

Represents a DID authorized to verify other users or perform governance actions.

Fields:

- `id`
- `verifier_did`
- `verifier_type`: `platform | board | credential_issuer | community`
- `scope`
- `authority_level`
- `status`: `active | suspended | revoked | expired`
- `appointed_by_did`
- `created_at`
- `expires_at`
- `revoked_at`

Scope examples:

- `identity`
- `personhood`
- `professional`
- `board:ai-ethics`
- `board:governance`
- `moderation`

### `VerificationCase`

Represents a request to move a user into a higher trust tier or credential
status.

Fields:

- `id`
- `subject_did`
- `requested_tier`
- `credential_type`
- `evidence_json`
- `status`: `submitted | assigned | approved | rejected | appealed | expired`
- `assigned_verifier_did`
- `decision`
- `decision_reason`
- `created_at`
- `decided_at`

### `VerifierDecision`

Represents the concrete decision made on a case.

Fields:

- `id`
- `case_id`
- `verifier_did`
- `decision`: `approve | reject | request_more_evidence`
- `reason`
- `issued_credential_id`
- `issued_assessment_id`
- `created_at`

### `TrustAuditLog`

Append-only audit trail for trust-impacting actions.

Fields:

- `id`
- `actor_did`
- `action_type`
- `target_did`
- `target_resource_id`
- `metadata_json`
- `created_at`

Examples:

- `verification_case.submitted`
- `verification_case.assigned`
- `verification_case.approved`
- `verification_case.rejected`
- `verifier.appointed`
- `verifier.revoked`
- `credential.revoked`
- `trust_tier.changed`

### `Endorsement`

Represents social trust evidence.

Fields:

- `id`
- `endorser_did`
- `subject_did`
- `context`
- `weight`
- `status`: `active | revoked | disputed`
- `created_at`
- `revoked_at`

Notes:

- Endorsements can help L2, but should not alone grant L3/L4.
- Abuse-resistant endorsement policy is required before heavy weighting.

## Trust Tier Derivation

The displayed tier should be derived, not manually overwritten forever.

Inputs:

- device continuity
- credential strength
- social graph quality
- moderation history
- board contribution
- verifier assessment
- account age
- account risk signals

Initial MVP rule:

```text
L0: OAuth or guest only
L1: active passkey/DID device credential
L2: active L2 TrustAssessment
L3: active L3 Credential or TrustAssessment
L4: active Verifier record with valid scope
```

Later rule:

```text
TrustTier = policy(
  active_device_credentials,
  active_credentials,
  active_trust_assessments,
  endorsements,
  moderation_history,
  verifier_status
)
```

## Verifier Lifecycle

### Appointment

L4 verifier status can begin as a platform-admin action.

Flow:

```text
1. Existing platform authority selects a DID.
2. System creates Verifier record.
3. System creates L4 TrustAssessment.
4. Audit log records appointment.
```

### Scope

Verifier authority must be scoped. A verifier with `board:ai-ethics` scope
cannot approve unrelated professional credentials unless policy grants it.

### Expiry

Verifier status should expire or require periodic renewal.

### Revocation

Verifier status can be revoked for:

- abuse
- inactivity
- conflict of interest
- compromised device/account
- policy change

### Conflict of Interest

Verifier must not review:

- their own verification case
- cases for accounts they directly control
- cases where policy detects close conflicting relationship

## Verification Case Flow

```text
1. User reaches L1 through passkey/DID.
2. User requests L2 or L3 verification.
3. User submits evidence.
4. System creates VerificationCase.
5. Case is assigned to an eligible verifier.
6. Verifier reviews evidence.
7. Verifier approves, rejects, or requests more evidence.
8. Approval creates Credential and/or TrustAssessment.
9. Effective trust tier is recalculated.
10. Audit log records all actions.
```

## Minimum Viable Verifier System

The first implementation should be intentionally simple and auditable.

### V1: Manual Verifier

Deliver:

- platform can appoint a verifier DID
- verifier can list assigned cases
- verifier can approve or reject L2/L3 requests
- approval creates TrustAssessment
- all actions create audit logs

### V2: Evidence-Based Verification

Deliver:

- users can submit evidence
- verifiers can request more evidence
- approvals can issue Credentials
- credentials can expire
- trust tier derives from active credentials/assessments

### V3: External Verifiable Credentials

Deliver:

- support external issuers
- verify credential signatures
- issuer trust registry
- revocation/status list support
- board-specific issuer policy

## API Plan

### User-facing trust endpoints

```text
GET  /v2/trust/me
POST /v2/trust/verification-cases
GET  /v2/trust/verification-cases/{id}
GET  /v2/trust/credentials/me
GET  /v2/trust/audit-log/me
```

### Verifier endpoints

```text
GET  /v2/trust/verifier/cases
POST /v2/trust/verifier/cases/{id}/assign
POST /v2/trust/verifier/cases/{id}/decision
GET  /v2/trust/verifier/audit-log
```

### Admin/bootstrap endpoints

```text
POST /v2/trust/verifiers
POST /v2/trust/verifiers/{did}/revoke
GET  /v2/trust/verifiers
```

Admin endpoints should be protected by a separate policy from normal L4
governance actions.

## Capability Mapping

Initial mapping:

```text
L0:
- read public content
- create low-impact content with strict limits

L1:
- create murmur and idea
- create basic discussion
- message existing friends

L2:
- fork public discussion
- flag public content
- request messages with same-board members

L3:
- create expert rooms
- higher contribution weight in scoped boards
- request advanced board privileges

L4:
- review verification cases
- issue trust assessments within scope
- create governance rooms
- propose slash / lock / hide moderation actions
```

## Messenger Integration

Messenger consumes Core trust state; it does not own it.

Messenger should ask Core:

```text
canMessage(senderDid, recipientDid)
canCreateGroup(userDid)
canCreateGovernanceRoom(userDid)
canInviteToVerifierRoom(userDid, inviteeDid)
```

Messenger receives Core events:

```text
trust_tier.changed
verifier.appointed
verifier.revoked
block.created
block.removed
account.suspended
```

If a user loses required trust status, Messenger should stop new privileged
actions immediately. Existing encrypted messages remain subject to retention and
local device state.

## Frontend Surfaces

### Trust Profile

Shows:

- current trust tier
- active credentials
- pending verification cases
- verifier status if any
- audit history relevant to the user

### Verification Request Flow

Allows:

- choose target tier
- choose credential type
- submit evidence
- track review status

### Verifier Console

Allows:

- list assigned cases
- inspect evidence
- approve/reject/request more evidence
- see prior audit trail
- understand scope and conflict warnings

## Governance and Safety

Verifier powers must be constrained by:

- scoped authority
- audit logs
- expiration
- revocation
- appeals
- conflict-of-interest checks
- quorum for high-impact decisions

Future policies:

- L3 approval requires one verifier.
- L4 appointment requires admin or multi-verifier quorum.
- Slash/lock/hide requires L4 scope and may require quorum.

## Implementation Phases

### Phase T1: Data Model and Capability Snapshot

Deliver:

- `Credential`
- `TrustAssessment`
- `Verifier`
- `VerificationCase`
- `VerifierDecision`
- `TrustAuditLog`
- capability snapshot includes L2-L4 derived permissions

### Phase T2: Manual L4 Bootstrap

Deliver:

- bootstrap verifier DID through env/admin endpoint
- verifier record creation
- audit log
- L4 capability recognition

Initial implementation note:

- `LEITH_BOOTSTRAP_VERIFIER_DIDS` seeds comma-separated L4 verifier DIDs at API startup.
- The startup path creates or updates the user, verifier record, system trust assessment, and audit log.

### Phase T3: L2/L3 Verification Cases

Deliver:

- user submits verification case
- verifier reviews case
- approve/reject decision
- approval creates trust assessment
- trust tier recalculates

Initial implementation note:

- `POST /v2/trust/verification-cases` accepts L2/L3 requests from L1+ users.
- `GET /v2/trust/verifier/cases` and `POST /v2/trust/verifier/cases/{id}/decision` back the L4 verifier queue.
- L2 approval creates a trust assessment; L3 approval also issues a credential.

### Phase T4: Frontend Trust Surfaces

Deliver:

- trust profile
- verification request form
- verifier console backed by real API

Initial implementation note:

- The discourse sidebar can submit L2/L3 verification cases.
- The L4 console renders only when the capability snapshot allows verifier review.

### Phase T5: Credential Expiry and Revocation

Deliver:

- credential expiration
- verifier revocation
- trust assessment revocation
- audit-visible trust tier changes

### Phase T6: External VC Support

Deliver:

- issuer registry
- credential signature verification
- revocation/status list
- board-specific accepted issuers

## Open Questions

- What initial evidence should grant L2?
- Should L2 require social endorsements, verifier approval, or both?
- Who can bootstrap the first L4 verifier?
- Should verifier appointment be environment-configured, admin API-based, or
  both?
- Should trust tier be global, board-scoped, or both?
- Which credential types are needed for the first real user testing group?
- How public should verifier audit logs be?
