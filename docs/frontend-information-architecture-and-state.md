# Aleth Frontend Information Architecture and State

## Purpose

This document proposes the frontend structure for Aleth as a trust-based forum
with three content modes:

- `Murmur`
- `Idea`
- `Discussion`

The goal is not just to add tabs to the current feed, but to make each mode
feel like a different reading and participation perspective while preserving one
shared identity and content lineage model.

## Frontend Principles

- The same person should feel present across all three modes.
- The same underlying topic may appear in different shapes across modes.
- LLM actions should feel like assisted transformations, not magical rewrites.
- Ownership and permission changes must be visible in the UI.
- Trust tier should be legible without dominating the experience.

## Top-Level App Structure

Recommended application shells:

1. `Personal Workspace`
2. `Public Forum`
3. `Shared Utilities`

### Personal Workspace

Contains:

- murmur capture and browsing
- idea drafting and editing
- private AI transformations
- projection review before publishing to discussion

### Public Forum

Contains:

- discussion discovery
- public reading surfaces
- debate graph views
- consensus summaries
- moderation and fork actions where permitted

### Shared Utilities

Contains:

- identity and trust status
- assistant/provider controls
- search
- notifications
- lineage inspector

## Primary Navigation

Recommended primary nav:

- `Murmur`
- `Idea`
- `Discussion`
- `Inbox`
- `Profile`

Recommended global utility area:

- identity chip
- trust tier badge
- provider selector
- search

## Route Proposal

### Core Routes

- `/murmur`
- `/murmur/:id`
- `/idea`
- `/idea/:id`
- `/discussion`
- `/discussion/:id`
- `/discussion/:id/map`
- `/discussion/:id/forks`

### Creation and Transformation Routes

- `/compose/murmur`
- `/compose/idea`
- `/transform/:jobId`
- `/project/:ideaId`

### Identity and Utilities

- `/me`
- `/settings/providers`
- `/settings/privacy`
- `/settings/identity`

## Information Architecture by Mode

### Murmur View

User need:
Low-pressure capture and lightweight review of early thoughts.

Recommended layout:

- left rail: filters, topic clusters, private tags
- center column: murmur stream
- right rail: theme emergence, AI suggestions, transform actions

Core UI objects:

- murmur card
- topic emergence chip
- private tag marker
- transform-to-idea CTA

Interaction model:

- fast create
- low visual ceremony
- small cards
- lightweight grouping by topic or time

Key distinction from current repo:

- This should not look like a public post feed.

### Idea View

User need:
Structure and refine authorship.

Recommended layout:

- left rail: draft list, related murmurs, subject links
- center: editor surface
- right rail: outline, persona controls, AI transform panel, projection panel

Core UI objects:

- idea editor
- linked murmurs panel
- thesis/outline inspector
- assistant persona switcher
- public projection preview

Interaction model:

- drafting and editing are central
- AI output appears as suggestions or generated draft sections
- the user always remains the final editor

Key distinction from current repo:

- This is not a modal on top of a feed; it is a dedicated authored workspace.

### Discussion View

User need:
Read and participate in public debate with structure.

Recommended layout:

- top bar: subject title, trust filters, summary controls
- center: discussion content
- optional side panel: consensus summary, stance legend, lineage, forks

Supported reading modes:

- thread mode
- stance map mode
- debate cards mode

Core UI objects:

- discussion header
- claim/rebuttal nodes
- support/oppose markers
- fork button
- consensus summary panel

Interaction model:

- more structured than flat comments
- identity visibility matters more here
- permissions depend on trust tier and discussion policy

Key distinction from current repo:

- The public layer should no longer be a simple card list plus modal.

## Cross-Mode Flows

### Flow 1: Murmur -> Idea

1. User selects one or more murmurs.
2. User chooses assistant provider and persona.
3. Frontend creates `TransformationJob`.
4. Result opens in transform review state.
5. User accepts output into new or existing idea draft.

Required UI states:

- source selection
- provider selection
- generating
- diff/review
- accept or discard

### Flow 2: Idea -> Discussion

1. User selects excerpt or thesis from an idea.
2. User chooses discussion shape and participation policy.
3. UI presents ownership transfer confirmation.
4. Frontend creates `Projection`.
5. User lands in new public discussion surface.

Required UI states:

- excerpt selection
- discussion policy setup
- ownership warning modal
- publish confirmation

### Flow 3: Discussion -> Personal Interpretation

1. User opens a public discussion.
2. User requests summary or analysis.
3. User chooses `system summary` or `private interpretation`.
4. Private interpretation result is stored in personal workspace if applicable.

Required UI states:

- public summary panel
- private analysis panel
- source disclosure
- save-to-idea or save-to-murmur action

## Trust Tier UX

Trust must be visible, but not feel like a gameified badge wall.

Recommended surfaces:

- identity chip near author names
- compact tier badges on content
- capability explanations when actions are disabled

Examples:

- "L1 required to create ideas"
- "L2 required to start a debate branch"
- "L4 required to initiate slashing review"

Recommended behavior:

- prefer clear explanations over opaque disabled buttons
- show why a capability is blocked and how trust matters

## Ownership UX

Ownership changes are a core product behavior and need explicit UI support.

### Creator-Owned States

Applies mostly to:

- murmurs
- ideas

UI implications:

- edit and delete affordances are obvious
- AI transformations stay private by default
- publication actions are explicit and reversible before publish

### Community-Constrained States

Applies mostly to:

- discussions
- forks

UI implications:

- edit and delete may be limited or unavailable
- fork availability should be visible
- author should understand that public discourse is persistent

Required UX element:

- ownership transfer confirmation dialog with concrete consequences

Suggested copy categories:

- what becomes public
- what control is reduced
- what remains attributable to the author

## Frontend State Model

Use state separation by concern, not by page only.

### 1. Session State

Purpose:
Identity and capabilities.

Suggested shape:

```ts
type SessionState = {
  identity: Identity | null;
  capabilities: {
    canCreateMurmur: boolean;
    canCreateIdea: boolean;
    canProjectToDiscussion: boolean;
    canForkDiscussion: boolean;
    canInitiateModeration: boolean;
  } | null;
  status: 'loading' | 'authenticated' | 'anonymous';
};
```

### 2. Content State

Purpose:
Fetched content, filters, and current item context.

Suggested shape:

```ts
type ContentState = {
  byId: Record<string, ContentItem>;
  lists: {
    murmurs: string[];
    ideas: string[];
    discussions: string[];
  };
  activeContentId: string | null;
  filters: {
    subjectId?: string;
    authorIdentityId?: string;
    sort?: string;
  };
};
```

### 3. Transformation State

Purpose:
Track LLM-assisted generation flows.

Suggested shape:

```ts
type TransformationState = {
  byId: Record<string, TransformationJob>;
  activeJobId: string | null;
  draftOutput: {
    contentItemDraft?: Partial<ContentItem>;
    notes?: string[];
  } | null;
};
```

### 4. Discussion State

Purpose:
Track debate nodes, summaries, and forks.

Suggested shape:

```ts
type DiscussionState = {
  nodesByDiscussionId: Record<string, DiscussionNode[]>;
  consensusByDiscussionId: Record<string, ConsensusSnapshot[]>;
  forksByDiscussionId: Record<string, Fork[]>;
  readingModeByDiscussionId: Record<string, 'thread' | 'map' | 'cards'>;
};
```

### 5. Draft and Editor State

Purpose:
Local editing state before save or publish.

Suggested shape:

```ts
type DraftState = {
  activeDraftMode: 'murmur' | 'idea' | null;
  draftTitle: string;
  draftBody: string;
  linkedSourceIds: string[];
  pendingProjection: {
    sourceIdeaId: string;
    excerpt: string;
    participationPolicy: 'read_only' | 'comment' | 'debate';
  } | null;
};
```

## Local vs Remote State Boundaries

Aleth needs explicit boundaries even before full local-first execution is built.

### Local-Only State

- unsaved drafts
- provider selection UI
- transform review state
- temporary node expansion/collapse state
- private interpretation notes

### Server-Synced State

- identity
- trust tier
- content items
- discussion nodes
- interactions
- moderation actions

### Future Local-First Extension

If the product later restores stronger personal/private storage:

- murmurs may live primarily in IndexedDB
- local transformations may run in-browser
- projection remains the consent bridge into server state

## Component Architecture Proposal

Recommended shared component groups:

- `identity/*`
- `content/*`
- `murmur/*`
- `idea/*`
- `discussion/*`
- `transform/*`
- `policy/*`

Examples:

- `identity/IdentityChip`
- `identity/TrustTierBadge`
- `content/LineageTrail`
- `murmur/MurmurComposer`
- `idea/IdeaEditor`
- `discussion/DiscussionMap`
- `discussion/DiscussionNodeCard`
- `transform/TransformationReview`
- `policy/OwnershipTransferDialog`

## Page-Level Data Loading Strategy

Recommended loading approach:

- route-level fetch for main resources
- local optimistic state for editor interactions
- polling or streaming for transformation jobs
- progressive loading for large discussion trees

Specific notes:

- discussion maps should lazily load deeper branches
- consensus summaries can be fetched independently from node trees
- capability snapshots should be fetched with session and content payloads

## Suggested Frontend Milestones

### Milestone 1

- replace single public feed mental model with three primary routes
- add basic `mode` support to UI rendering
- keep backend simple

### Milestone 2

- implement idea editor and transformation review flow
- add projection confirmation UX

### Milestone 3

- implement structured discussion nodes
- add thread/map/cards reading toggle
- add consensus summary panel

### Milestone 4

- deepen trust-tier gating
- add moderation and fork UX
- refine lineage browsing across modes

## Recommended Mapping to Current Repo

Current frontend files map roughly to these future areas:

- current main page becomes discussion-focused shell
- `MediaLab` is conceptually closer to a specialized composer, not the main
  authoring model
- `VaultTransition` can evolve into idea/discussion detail views
- `VerifierConsole` can evolve into moderation tools
- `useAuth` should evolve into session/capability state instead of mock-only auth

The main frontend shift is to stop centering one generic feed and instead build
three differentiated surfaces with explicit transformation flows between them.
