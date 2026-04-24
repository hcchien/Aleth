# Aleth Database Schema Draft

## Purpose

This document proposes a relational schema for Aleth's trust-based,
three-mode discourse system. It extends the current `users + posts` model into
an identity-centric schema that can support:

- certified identities and trust tiers
- `murmur`, `idea`, and `discussion` content modes
- AI-assisted transformations
- public debate graphs
- ownership transfer and moderation

The schema is written as a conceptual relational design, with notes for SQLite
and Postgres compatibility.

## Design Principles

- Preserve backward compatibility where possible.
- Normalize identity, content, lineage, and governance concerns separately.
- Keep the main content table generic, then attach mode-specific extension
  tables.
- Store discussion structure explicitly rather than inferring it from flat posts.

## Current Schema

The repo currently has:

- `users`
- `posts`

Recommended migration path:

1. Evolve `users` into `identities`
2. Evolve `posts` into `content_items`
3. Add extension and relation tables without forcing a big-bang rewrite

## Table Overview

### Identity Domain

- `identities`
- `identity_trust_tiers`
- `credentials`
- `identity_sessions`

### Content Domain

- `subjects`
- `content_items`
- `murmurs`
- `ideas`
- `discussions`
- `content_relations`
- `ownership_policies`

### Transformation Domain

- `transformation_jobs`
- `transformation_sources`
- `projections`
- `ai_context_policies`

### Discussion Domain

- `discussion_nodes`
- `discussion_forks`
- `consensus_snapshots`
- `consensus_snapshot_sources`
- `interactions`
- `moderation_actions`

## Tables

### `identities`

Purpose:
Canonical participant record.

Suggested columns:

```sql
CREATE TABLE identities (
  id TEXT PRIMARY KEY,
  did TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  avatar_url TEXT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);
```

Notes:

- In SQLite, `TEXT` ids are fine for UUID/ULID-style application ids.
- `did` should remain unique and indexed.

### `identity_trust_tiers`

Purpose:
Track current and historical trust tier state.

```sql
CREATE TABLE identity_trust_tiers (
  id TEXT PRIMARY KEY,
  identity_id TEXT NOT NULL REFERENCES identities(id),
  tier INTEGER NOT NULL,
  score DOUBLE PRECISION NULL,
  effective_from TIMESTAMPTZ NOT NULL,
  effective_to TIMESTAMPTZ NULL
);
```

Indexes:

- `(identity_id, effective_to)`

Notes:

- The active trust tier row is the one with `effective_to IS NULL`.
- This preserves history for audits and future product logic.

### `credentials`

Purpose:
Store proofs attached to identities.

```sql
CREATE TABLE credentials (
  id TEXT PRIMARY KEY,
  identity_id TEXT NOT NULL REFERENCES identities(id),
  type TEXT NOT NULL,
  issuer TEXT NOT NULL,
  status TEXT NOT NULL,
  public_key BYTEA NULL,
  oauth_subject TEXT NULL,
  metadata JSONB NULL,
  issued_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);
```

SQLite note:

- Use `BLOB` for `public_key`
- Use `TEXT` storing JSON for `metadata`

### `identity_sessions`

Purpose:
Represent authenticated sessions if server-managed sessions are used.

```sql
CREATE TABLE identity_sessions (
  id TEXT PRIMARY KEY,
  identity_id TEXT NOT NULL REFERENCES identities(id),
  session_token_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NULL
);
```

## Content Domain

### `subjects`

Purpose:
Group related discourse under one topic or question.

```sql
CREATE TABLE subjects (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  slug TEXT NULL UNIQUE,
  summary TEXT NULL,
  created_by_identity_id TEXT NULL REFERENCES identities(id),
  created_at TIMESTAMPTZ NOT NULL
);
```

### `content_items`

Purpose:
Core abstraction for all authored content.

```sql
CREATE TABLE content_items (
  id TEXT PRIMARY KEY,
  author_identity_id TEXT NOT NULL REFERENCES identities(id),
  subject_id TEXT NULL REFERENCES subjects(id),
  mode TEXT NOT NULL,
  title TEXT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL,
  visibility TEXT NOT NULL,
  trust_tier_required INTEGER NULL,
  signature TEXT NULL,
  published_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);
```

Indexes:

- `(mode, visibility, status, published_at DESC)`
- `(author_identity_id, created_at DESC)`
- `(subject_id, mode, created_at DESC)`

Notes:

- This table is the generalized successor of `posts`.
- Current `visibility_score` should move out into interaction or ranking logic.

### `murmurs`

Purpose:
Mode-specific murmur fields.

```sql
CREATE TABLE murmurs (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id),
  tone TEXT NULL,
  source_type TEXT NULL,
  is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
  private_tags JSONB NULL
);
```

SQLite note:

- Use `INTEGER` for booleans and `TEXT` for JSON.

### `ideas`

Purpose:
Mode-specific idea fields.

```sql
CREATE TABLE ideas (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id),
  thesis TEXT NULL,
  outline JSONB NULL,
  assistant_persona TEXT NULL
);
```

### `discussions`

Purpose:
Mode-specific discussion configuration.

```sql
CREATE TABLE discussions (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id),
  discussion_shape TEXT NOT NULL,
  participation_policy TEXT NOT NULL,
  fork_policy TEXT NOT NULL,
  consensus_state TEXT NOT NULL DEFAULT 'none'
);
```

### `content_relations`

Purpose:
Track lineage and semantic links between content items.

```sql
CREATE TABLE content_relations (
  id TEXT PRIMARY KEY,
  from_content_item_id TEXT NOT NULL REFERENCES content_items(id),
  to_content_item_id TEXT NOT NULL REFERENCES content_items(id),
  relation_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

Indexes:

- `(from_content_item_id, relation_type)`
- `(to_content_item_id, relation_type)`

### `ownership_policies`

Purpose:
Store permission behavior after publication or projection.

```sql
CREATE TABLE ownership_policies (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id),
  owner_identity_id TEXT NOT NULL REFERENCES identities(id),
  edit_policy TEXT NOT NULL,
  delete_policy TEXT NOT NULL,
  comment_policy TEXT NOT NULL,
  fork_policy TEXT NOT NULL,
  moderation_policy TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);
```

Notes:

- The API should still compute user-specific capability snapshots at runtime.
- This table stores the canonical policy source.

## Transformation Domain

### `transformation_jobs`

Purpose:
Track AI-assisted content conversion.

```sql
CREATE TABLE transformation_jobs (
  id TEXT PRIMARY KEY,
  requested_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  target_mode TEXT NOT NULL,
  provider_type TEXT NOT NULL,
  provider_config_ref TEXT NULL,
  prompt_profile TEXT NULL,
  status TEXT NOT NULL,
  input_snapshot JSONB NULL,
  output_snapshot JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NULL
);
```

### `transformation_sources`

Purpose:
Join table from transformations to source content.

```sql
CREATE TABLE transformation_sources (
  transformation_job_id TEXT NOT NULL REFERENCES transformation_jobs(id),
  content_item_id TEXT NOT NULL REFERENCES content_items(id),
  PRIMARY KEY (transformation_job_id, content_item_id)
);
```

### `projections`

Purpose:
Record creator-approved transfers from idea space into public discussion.

```sql
CREATE TABLE projections (
  id TEXT PRIMARY KEY,
  source_idea_id TEXT NOT NULL REFERENCES content_items(id),
  target_discussion_id TEXT NOT NULL REFERENCES content_items(id),
  projected_excerpt TEXT NOT NULL,
  participation_policy TEXT NOT NULL,
  ownership_transfer_acknowledged BOOLEAN NOT NULL,
  created_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  created_at TIMESTAMPTZ NOT NULL
);
```

### `ai_context_policies`

Purpose:
Declare which providers may access which data classes.

```sql
CREATE TABLE ai_context_policies (
  id TEXT PRIMARY KEY,
  provider_type TEXT NOT NULL,
  allowed_visibilities JSONB NOT NULL,
  allowed_content_modes JSONB NOT NULL,
  can_access_private_tags BOOLEAN NOT NULL,
  can_access_public_discussion BOOLEAN NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

Notes:

- This can start as configuration rather than a runtime-managed table.
- It is still useful to model it early.

## Discussion Domain

### `discussion_nodes`

Purpose:
Represent structured debate nodes.

```sql
CREATE TABLE discussion_nodes (
  id TEXT PRIMARY KEY,
  discussion_id TEXT NOT NULL REFERENCES content_items(id),
  parent_node_id TEXT NULL REFERENCES discussion_nodes(id),
  author_identity_id TEXT NOT NULL REFERENCES identities(id),
  node_type TEXT NOT NULL,
  stance TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);
```

Indexes:

- `(discussion_id, parent_node_id)`
- `(discussion_id, created_at)`

### `discussion_forks`

Purpose:
Track community-owned derivative discussions.

```sql
CREATE TABLE discussion_forks (
  id TEXT PRIMARY KEY,
  source_discussion_id TEXT NOT NULL REFERENCES content_items(id),
  root_node_id TEXT NOT NULL REFERENCES discussion_nodes(id),
  fork_discussion_id TEXT NOT NULL REFERENCES content_items(id),
  created_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  reason TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

### `consensus_snapshots`

Purpose:
Store public summaries of agreement and disagreement.

```sql
CREATE TABLE consensus_snapshots (
  id TEXT PRIMARY KEY,
  discussion_id TEXT NOT NULL REFERENCES content_items(id),
  generated_by TEXT NOT NULL,
  summary TEXT NOT NULL,
  agreements JSONB NOT NULL,
  disagreements JSONB NOT NULL,
  open_questions JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

### `consensus_snapshot_sources`

Purpose:
Track which nodes were used to generate a snapshot.

```sql
CREATE TABLE consensus_snapshot_sources (
  consensus_snapshot_id TEXT NOT NULL REFERENCES consensus_snapshots(id),
  discussion_node_id TEXT NOT NULL REFERENCES discussion_nodes(id),
  PRIMARY KEY (consensus_snapshot_id, discussion_node_id)
);
```

### `interactions`

Purpose:
Record weighted actions that influence ranking and discourse visibility.

```sql
CREATE TABLE interactions (
  id TEXT PRIMARY KEY,
  actor_identity_id TEXT NOT NULL REFERENCES identities(id),
  target_content_item_id TEXT NOT NULL REFERENCES content_items(id),
  interaction_type TEXT NOT NULL,
  trust_tier_at_time INTEGER NOT NULL,
  weight DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

Indexes:

- `(target_content_item_id, interaction_type)`
- `(actor_identity_id, created_at DESC)`

### `moderation_actions`

Purpose:
Support flags, slashes, locks, and other moderation events.

```sql
CREATE TABLE moderation_actions (
  id TEXT PRIMARY KEY,
  target_content_item_id TEXT NULL REFERENCES content_items(id),
  target_identity_id TEXT NULL REFERENCES identities(id),
  action_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  initiated_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  required_trust_tier INTEGER NOT NULL,
  status TEXT NOT NULL,
  metadata JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL,
  resolved_at TIMESTAMPTZ NULL
);
```

## Derived Read Models

The system will likely need read-optimized projections for UI performance.

Suggested derived views or materialized projections:

- `public_discussion_feed`
- `idea_feed`
- `identity_capability_snapshot`
- `discussion_graph_summary`
- `content_visibility_score`

These do not need to exist on day one, but the schema should allow them.

## Migration Strategy from Current Tables

### `users -> identities`

Map:

- `users.did` -> `identities.did`
- `users.trust_tier` -> active row in `identity_trust_tiers`
- `users.public_key` and `users.oauth_id` -> `credentials`

### `posts -> content_items + discussions`

Recommended first migration:

- create `content_items`
- copy existing posts as `mode='discussion'`
- create `discussions` rows with defaults
- map `author_did` to `author_identity_id`

Suggested bridge defaults:

- `status='active'`
- `visibility='public'`
- `discussion_shape='thread'`
- `participation_policy='comment'`
- `fork_policy='allowed'`

## First Implementation Slice

To keep implementation small, add these tables first:

1. `identities`
2. `identity_trust_tiers`
3. `content_items`
4. `ideas`
5. `discussions`
6. `content_relations`
7. `ownership_policies`
8. `discussion_nodes`
9. `transformation_jobs`
10. `transformation_sources`
11. `projections`

This is enough to support:

- trust-backed identity
- three content modes
- AI transformation tracking
- public debate nodes
- ownership transfer
