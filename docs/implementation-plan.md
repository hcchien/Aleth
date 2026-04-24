# Aleth Implementation Plan

## Goal

Deliver a working prototype that upgrades the current trust-based forum into a
three-mode discourse system centered on:

- certified humans and trust tiers
- differentiated content modes: `murmur`, `idea`, `discussion`
- AI transformation hooks between modes
- structured public discussion rather than a flat post feed

This plan is intentionally implementation-oriented. It defines the smallest
useful vertical slice, then sequences the rest of the work into phases that can
ship incrementally without losing the core product direction.

## Delivery Strategy

The repo should evolve in layers rather than through a rewrite:

1. Preserve current identity and trust direction.
2. Introduce a generalized content model under `v2`.
3. Keep current `/posts` flow as compatibility surface while new frontend
   experiences move to `/v2`.
4. Prioritize a working three-mode prototype before deeper AI or governance
   systems.

## MVP Slice

The first complete vertical slice should support:

1. Authenticated user can load `me` and see trust tier.
2. Authenticated user can create content in three modes:
   - `murmur`
   - `idea`
   - `discussion`
3. Frontend can list content by mode.
4. Discussion items can have structured nodes.
5. Frontend can read and append discussion nodes.
6. UI clearly differentiates murmur, idea, and discussion views.

This is the minimum product that proves:

- Aleth is no longer a single generic feed.
- trust-based identity still anchors the system.
- discussion can become structured.

## Phase Plan

### Phase 1: Foundation and Compatibility

Status target:
Must be completed in this implementation cycle.

Scope:

- add implementation plan and system docs
- add `v2` API surface
- add base domain model for content modes
- add in-memory store support for new models
- keep `/posts` compatibility behavior

Deliverables:

- `/v2/me`
- `/v2/content-items`
- `/v2/content-items/{id}`
- `/v2/discussions/{id}/nodes`

### Phase 2: Three-Mode Frontend Shell

Status target:
Must be completed in this implementation cycle.

Scope:

- replace single feed-first mental model with mode-aware shell
- create three top-level perspectives
- allow content creation in each mode
- add discussion node view and reply flow

Deliverables:

- mode nav for `Murmur`, `Idea`, `Discussion`
- content cards specialized per mode
- simple composer
- discussion detail panel with structured replies

### Phase 3: Ownership and Projection

Status target:
Should begin in this cycle if time permits, otherwise remain designed but not
fully shipped.

Scope:

- model content lineage
- explicit projection flow from idea to discussion
- ownership transfer confirmation

Deliverables:

- `ContentRelation`
- projection endpoint
- frontend ownership warning dialog

### Phase 4: AI Transformation

Status target:
Design for now, partial stubs acceptable.

Scope:

- transformation job model
- provider selection
- draft review state

Deliverables:

- transformation job API stubs
- frontend transform affordance placeholders

### Phase 5: Trust-Aware Governance

Status target:
Design and preserve compatibility with current verifier direction.

Scope:

- richer moderation actions
- capability gating by trust tier
- fork policy and structured public persistence

Deliverables:

- capability snapshots in API
- clearer trust-based action gating

### Phase 6: L2-L4 Trust and Verifier System

Status target:
Next implementation cycle.

Scope:

- distinguish DID continuity from higher trust
- add verifier-backed trust assessments
- add verification cases for L2/L3
- add manual L4 verifier bootstrap
- add audit log for trust-impacting actions

Deliverables:

- `Credential`
- `TrustAssessment`
- `Verifier`
- `VerificationCase`
- `VerifierDecision`
- `TrustAuditLog`
- `/v2/trust/me`
- `/v2/trust/verification-cases`
- verifier review endpoints
- real Verifier Console backed by API

See [Trust and Verifier System](./trust-and-verifier-system.md).

## Backend Work Breakdown

### Step A: Domain Model Upgrade

Add:

- `ContentMode`
- `ContentItem`
- `DiscussionNode`
- `ContentPolicySnapshot`

Keep:

- existing `User`
- existing `TrustTier`
- existing `Post` compatibility path

### Step B: Store Layer

Implement in `MemoryStore`:

- create/list/get content items
- create/list discussion nodes

Implement in `SQLStore`:

- minimal schema support for content items and discussion nodes
- preserve current post behavior

### Step C: API Layer

Add `v2` endpoints for:

- session identity
- content items
- discussion nodes

Compatibility decision:

- old `/posts` remains mapped to public `discussion` content semantics

## Frontend Work Breakdown

### Step A: Shared State

Introduce:

- session state
- content lists by mode
- discussion node state
- active mode and selected content

### Step B: Mode Shell

Create:

- left nav or top nav for three modes
- per-mode list rendering
- compact identity and trust indicator

### Step C: Composition

Create:

- one generic composer with mode presets
- different placeholder and help text for each mode

### Step D: Discussion Detail

Create:

- node list
- reply composer
- node type and stance controls

## Technical Assumptions

- Authentication remains mock/session-header-based for prototype purposes.
- In-memory store is sufficient for the first working slice.
- SQL schema may be partially upgraded for future persistence, but prototype
  success does not depend on every advanced table being fully wired.
- AI transformation can remain stubbed if the API and UX seams are in place.

## Concrete Implementation Sequence

1. Write this implementation plan.
2. Extend OpenAPI with `v2` schemas and endpoints.
3. Regenerate API bindings.
4. Extend backend store model and handlers.
5. Keep old routes working.
6. Replace the frontend home page with a three-mode shell.
7. Add mode-aware content create/list behavior.
8. Add discussion node detail and reply.
9. Run tests and build verification.

## Done Criteria for This Cycle

This implementation cycle is successful if all of the following are true:

- the repo has a clear implementation plan
- backend exposes usable `v2` three-mode endpoints
- frontend renders distinct murmur, idea, and discussion surfaces
- user can create items in different modes
- user can open a discussion and add structured nodes
- old compatibility flow is not catastrophically broken

## Deferred Work

These items are important but not required to claim completion of the current
cycle:

- full projection workflow
- full transformation job execution against real LLM providers
- full moderation workflow
- forked discussion UX
- advanced ranking and trust-aware weighting
- local-first IndexedDB storage for personal-only content

## Risks

### Risk: Expanding too much at once

Mitigation:
Prefer working vertical slices over modeling every endpoint from day one.

### Risk: Breaking current API consumers

Mitigation:
Keep `/posts` and existing auth endpoints intact.

### Risk: Frontend complexity grows faster than backend readiness

Mitigation:
Use progressive enhancement. Start with mode-aware list and detail views before
adding full transformations and projections.

## Immediate Work Authorization

The next execution step after creating this plan is:

- implement Phase 1 and Phase 2 completely
- implement as much of Phase 3 as can fit without destabilizing the prototype
