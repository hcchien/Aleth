# Aleth Product Principles and Domain Model

## Product Thesis

Aleth is a trust-based forum centered on real humans and certified identity.
Its purpose is not to maximize reach or frictionless posting, but to create a
public discourse space that is resilient to sybil attacks, coordinated
manipulation, and low-accountability participation.

What differentiates Aleth from conventional forums and social networks is not
only identity assurance, but also content shape transformation. The same core
thought can move through three distinct modes:

- `Murmur`: an early, compact, low-pressure expression
- `Idea`: an authored and structured viewpoint
- `Discussion`: a public, community-owned debate surface
- `Messenger`: trusted private collaboration between certified participants

LLMs are used as translators between these modes. They help users reshape
content, but they do not replace authorship, consent, or accountability.

## Product Principles

### 1. Certified people matter

The forum should reward accountable participation from real humans with
certified or reputation-backed identity.

Implications:

- Identity is a first-class system primitive, not a UI add-on.
- Reputation, moderation weight, and distribution should depend on trust tier.
- Important discourse actions should cost more than casual posting and should
  prefer higher-trust participants.

### 2. Anti-sybil is a product feature

Resistance to sybil attacks and narrative manipulation is part of the user
value proposition, not only a backend security concern.

Implications:

- Feed ranking, moderation, and governance should account for trust tier.
- Participation capabilities may differ by trust tier.
- The product should make legitimacy visible without making the experience feel
  bureaucratic.

### 3. One thought, three modes

Aleth should support different stages of expression rather than treating all
content as the same kind of post.

Implications:

- Short intuitions, complete essays, and public debates should not share the
  same presentation model.
- Each mode should have distinct reading UX, writing affordances, and social
  rules.
- Content can move across modes while preserving lineage.

### 4. LLMs translate, humans decide

LLMs help users transform content between modes, summarize debates, and
structure arguments, but all publication and ownership transitions require human
consent.

Implications:

- No automatic publishing from private or authored space into public space.
- AI-generated transformations must be reviewable and editable.
- AI should expose uncertainty and source boundaries when reshaping discourse.

### 5. Ownership changes with publication context

Aleth distinguishes between creator-owned expression and community-owned
discourse.

Implications:

- A user retains strong control over `Murmur` and `Idea`.
- Once content is projected into `Discussion`, deletion and edit rights narrow.
- Forked discussion threads are owned by the discourse space, not by the
  original author.

### 6. Reading perspective is part of the product

The same underlying subject matter should feel different depending on whether a
user is browsing murmurs, ideas, or discussions.

Implications:

- `Murmur` should feel lightweight and stream-like.
- `Idea` should feel authored, coherent, and readable as an essay or argument.
- `Discussion` should feel navigable as a debate map, stance tree, or response
  graph.

### 7. Public discourse needs legible structure

Aleth should help people understand what is being argued, where agreement
exists, and where disagreement remains.

Implications:

- Discussion is not just a flat comment list.
- Public content should support stance extraction, summaries, and branch/fork
  views.
- The system should elevate reasoning clarity over engagement bait.

## Core Product Surfaces

### Murmur Layer

Purpose:
Capture low-friction observations, questions, fragments, and early intuitions.

Characteristics:

- Short-form and low-pressure
- Often personal in origin, but not necessarily private forever
- May later be refined into an idea
- Can be filtered or grouped by topic emergence

LLM roles:

- Cluster similar murmurs
- Suggest recurring themes
- Propose expansion into ideas

### Idea Layer

Purpose:
Hold authored, structured viewpoints that belong to the creator.

Characteristics:

- Long-form or semi-structured
- Editable by the author
- Can combine multiple murmurs into one coherent artifact
- Can be transformed into selectively projected public material

LLM roles:

- Expand murmurs into structured drafts
- Rewrite in different assistant personas
- Extract thesis, arguments, and supporting points

### Discussion Layer

Purpose:
Enable public, accountable, community-visible argument and response.

Characteristics:

- Public and socially consequential
- Read as a debate surface rather than a private draft
- Can contain branches, counterarguments, and forks
- Ownership is partially transferred from author to community process

LLM roles:

- Summarize consensus and disagreement
- Reframe long-form ideas into debate cards or stance trees
- Help users privately interpret public discussion

### Messenger Layer

Purpose:
Enable trusted private collaboration between certified participants without
giving the server access to message plaintext.

Characteristics:

- Independent realtime service
- Uses end-to-end encryption for message content
- Uses Core identity, friend graph, trust tier, and block policy as social truth
- Stores encrypted envelopes, receipts, device records, and prekey bundles
- Can privately coordinate around ideas or discussions

LLM roles:

- No server-side automatic reading of private messages
- Client-side or explicitly exported summaries only
- User-reviewed transformation from chat summary into idea or discussion

See [E2EE Messenger Architecture](./e2ee-messenger-architecture.md).

## Domain Model Overview

The model below separates identity, content, transformation, and governance.

### Identity and Trust

#### `Identity`

Represents a forum participant.

Fields:

- `id`
- `did`
- `display_name`
- `avatar_url`
- `status`
- `created_at`

Notes:

- `did` is the canonical identity handle.
- One identity may accumulate trust and certifications over time.

#### `TrustTier`

Represents the participant's current trust level.

Fields:

- `identity_id`
- `tier`
- `score`
- `effective_from`
- `effective_to`

Notes:

- Maps naturally to the current `L0` to `L4` model in the repo.
- `tier` affects rate limits, content powers, moderation powers, and ranking
  weight.
- DID is the identity anchor, but L2 to L4 require additional evidence,
  credentials, verifier decisions, or governance authority. See
  [Trust and Verifier System](./trust-and-verifier-system.md).

#### `Credential`

Represents proof attached to an identity.

Fields:

- `id`
- `identity_id`
- `type`
- `issuer`
- `status`
- `issued_at`
- `expires_at`
- `metadata`

Examples:

- passkey registration
- social verification
- authority credential
- temporal/reputation milestone

## Content Primitives

### `Subject`

Represents an underlying topic, question, or issue that multiple content items
may refer to.

Fields:

- `id`
- `title`
- `slug`
- `summary`
- `created_at`

Notes:

- Useful when the same topic appears as murmurs, ideas, and discussions.
- Enables cross-mode navigation around one issue.

### `ContentItem`

Base content abstraction shared by all expression modes.

Fields:

- `id`
- `author_identity_id`
- `subject_id`
- `mode`
- `title`
- `body`
- `status`
- `visibility`
- `created_at`
- `updated_at`
- `published_at`

Enums:

- `mode`: `murmur | idea | discussion`
- `status`: `draft | active | archived | removed`
- `visibility`: `private | unlisted | public`

Notes:

- This is the core abstraction that generalizes today's `Post`.
- Different modes should still have specialized child entities or metadata.

### `Murmur`

Specialized content for early-stage expression.

Fields:

- `content_item_id`
- `tone`
- `source_type`
- `is_sensitive`
- `private_tags`

Examples of `source_type`:

- typed note
- voice transcript
- imported quote

Notes:

- A murmur may remain private or later become the source for an idea.
- Sensitive tags can be used to block AI projection into public outputs.

### `Idea`

Specialized content for authored, structured expression.

Fields:

- `content_item_id`
- `thesis`
- `outline`
- `assistant_persona`
- `owner_controls`

Notes:

- An idea is still creator-owned.
- It may be composed from multiple murmurs.

### `Discussion`

Specialized content for public discourse entry points.

Fields:

- `content_item_id`
- `discussion_shape`
- `participation_policy`
- `fork_policy`
- `consensus_state`

Examples:

- `discussion_shape`: `thread | stance_map | debate_cards`
- `participation_policy`: `read_only | comment | debate`

Notes:

- A discussion item is usually public.
- It can be a projected version of an idea or a born-public discussion prompt.

## Content Lineage and Transformation

### `ContentRelation`

Represents relationships between content items.

Fields:

- `id`
- `from_content_item_id`
- `to_content_item_id`
- `relation_type`
- `created_at`

Examples of `relation_type`:

- `expanded_from`
- `projected_from`
- `summarized_from`
- `forked_from`
- `supports`
- `rebuts`
- `references`

Notes:

- This is how Aleth preserves content ancestry across modes.
- It is critical for traceability when AI helps transform content.

### `TransformationJob`

Represents an AI-assisted conversion from one content shape to another.

Fields:

- `id`
- `requested_by_identity_id`
- `source_content_item_ids`
- `target_mode`
- `provider_type`
- `provider_config_ref`
- `prompt_profile`
- `status`
- `input_snapshot`
- `output_snapshot`
- `created_at`
- `completed_at`

Enums:

- `provider_type`: `local_llm | byok | system_llm`
- `status`: `queued | running | completed | failed | discarded`

Notes:

- The output of a transformation should not be considered published by default.
- Publication is a separate user action.

### `Projection`

Represents an authored act of transferring part of an idea into public
discussion.

Fields:

- `id`
- `source_idea_id`
- `target_discussion_id`
- `projected_excerpt`
- `participation_policy`
- `ownership_transfer_acknowledged`
- `created_at`

Notes:

- Projection is not just formatting; it is a rights and context change.
- The confirmation UX should make this explicit.

## Discussion and Debate Structures

### `DiscussionNode`

Represents a node inside a public discussion graph.

Fields:

- `id`
- `discussion_id`
- `parent_node_id`
- `author_identity_id`
- `node_type`
- `stance`
- `body`
- `created_at`

Examples of `node_type`:

- `claim`
- `question`
- `evidence`
- `rebuttal`
- `summary`

Examples of `stance`:

- `support`
- `oppose`
- `clarify`
- `neutral`

Notes:

- This allows the public layer to become a tree or graph rather than a flat
  comment list.

### `Fork`

Represents a community-owned derivative debate path.

Fields:

- `id`
- `source_discussion_id`
- `root_node_id`
- `created_by_identity_id`
- `reason`
- `created_at`

Notes:

- A fork preserves the fact that someone diverged from the original line of
  reasoning.
- The original author should not control deletion of a community fork.

### `ConsensusSnapshot`

Represents a system-generated summary of agreement and disagreement.

Fields:

- `id`
- `discussion_id`
- `generated_by`
- `summary`
- `agreements`
- `disagreements`
- `open_questions`
- `source_node_ids`
- `created_at`

Notes:

- Usually generated by the `system_llm`.
- Must only use public discussion material.

## Ownership and Governance

### `OwnershipPolicy`

Defines who can edit, delete, fork, or moderate a content item.

Fields:

- `content_item_id`
- `owner_identity_id`
- `edit_policy`
- `delete_policy`
- `comment_policy`
- `fork_policy`
- `moderation_policy`

Notes:

- `Murmur` and `Idea` default to strong owner control.
- `Discussion` defaults to narrower author control and stronger community
  persistence.

### `ModerationAction`

Represents moderation or slashing decisions.

Fields:

- `id`
- `target_content_item_id`
- `target_identity_id`
- `action_type`
- `reason`
- `initiated_by_identity_id`
- `required_trust_tier`
- `status`
- `created_at`

Examples of `action_type`:

- `flag`
- `hide`
- `slash`
- `lock`
- `archive`

Notes:

- This maps well to the repo's existing verifier/slashing direction.

### `Interaction`

Represents user actions that influence ranking and discourse visibility.

Fields:

- `id`
- `actor_identity_id`
- `target_content_item_id`
- `interaction_type`
- `weight`
- `created_at`

Examples:

- endorse
- rebut
- bookmark
- cite
- follow

Notes:

- Weight should be derived from trust tier and action semantics.
- Not every interaction should be treated as pure engagement.

## Privacy and AI Boundary Model

Aleth should formalize which AI is allowed to read which data.

### `AIContextPolicy`

Defines data access boundaries for AI operations.

Fields:

- `id`
- `provider_type`
- `allowed_visibilities`
- `allowed_content_modes`
- `can_access_private_tags`
- `can_access_public_discussion`

Recommended defaults:

- `local_llm`: may access private murmurs and ideas on-device
- `byok`: may access user-selected private content with explicit consent
- `system_llm`: may access public discussion only

This distinction is important even if the first prototype does not implement
all execution paths yet.

## Suggested Aggregate Boundaries

For implementation, the domain can be grouped into these aggregates:

- `IdentityAggregate`
  - `Identity`
  - `TrustTier`
  - `Credential`

- `ContentAggregate`
  - `ContentItem`
  - `Murmur`
  - `Idea`
  - `Discussion`
  - `ContentRelation`
  - `OwnershipPolicy`

- `DiscussionAggregate`
  - `DiscussionNode`
  - `Fork`
  - `ConsensusSnapshot`
  - `Interaction`
  - `ModerationAction`

- `TransformationAggregate`
  - `TransformationJob`
  - `Projection`
  - `AIContextPolicy`

## Mapping to the Current Repo

Current code already contains partial foundations:

- `User` maps toward `Identity`
- `TrustTier` already exists and should remain central
- `Post` should evolve into `ContentItem` plus specialized mode data
- `visibility_score` can evolve into a richer ranking/interaction model
- verifier console and slashing can evolve into `ModerationAction`

The main architectural change is to stop treating all content as a single post
type and instead model:

- expression stage
- ownership boundary
- AI transformation lineage
- debate structure

## Immediate Modeling Recommendations

For the next implementation phase, prioritize these additions first:

1. Add `mode` to the core content model.
2. Add `ContentRelation` so murmurs, ideas, and discussions can be linked.
3. Add `OwnershipPolicy` so publication context affects permissions.
4. Add `TransformationJob` to track LLM-assisted reshaping.
5. Add `DiscussionNode` so the public layer can become a structured debate
   surface.

These five changes are enough to move the repo from a trust-based feed prototype
to a trust-based discourse system with differentiated content modes.
