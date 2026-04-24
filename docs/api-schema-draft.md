# Aleth API Schema Draft

## Purpose

This document proposes the next API surface for Aleth based on the three-mode
content model:

- `murmur`
- `idea`
- `discussion`

The design keeps the current trust-based identity direction, while expanding
the API beyond a single `posts` feed into a discourse system with content
lineage, AI transformations, and public debate structure.

## API Design Principles

- Keep `identity` and `trust tier` as first-class concerns.
- Separate creator-owned content from community-owned discourse.
- Make AI transformation explicit and reviewable.
- Preserve content lineage across mode changes.
- Allow incremental migration from the current `posts` endpoints.

## Versioning Recommendation

- Keep current endpoints under `v1` compatibility behavior.
- Introduce new endpoints under `/v2`.
- Treat current `Post` as a legacy `ContentItem` projection.

## Resource Overview

### Identity Resources

- `/v2/auth/*`
- `/v2/me`
- `/v2/identities/{identityId}`
- `/v2/credentials`

### Content Resources

- `/v2/subjects`
- `/v2/content-items`
- `/v2/murmurs`
- `/v2/ideas`
- `/v2/discussions`
- `/v2/content-relations`

### Transformation Resources

- `/v2/transformation-jobs`
- `/v2/projections`
- `/v2/summaries`

### Discussion Resources

- `/v2/discussions/{discussionId}/nodes`
- `/v2/discussions/{discussionId}/forks`
- `/v2/discussions/{discussionId}/consensus-snapshots`
- `/v2/interactions`
- `/v2/moderation-actions`

## Core Schemas

### Identity

```yaml
Identity:
  type: object
  required: [id, did, displayName, trustTier, status, createdAt]
  properties:
    id:
      type: string
    did:
      type: string
    displayName:
      type: string
    avatarUrl:
      type: string
      nullable: true
    trustTier:
      type: integer
      minimum: 0
      maximum: 4
    trustScore:
      type: number
      format: float
      nullable: true
    status:
      type: string
      enum: [active, suspended, deleted]
    createdAt:
      type: string
      format: date-time
```

### Credential

```yaml
Credential:
  type: object
  required: [id, identityId, type, issuer, status, issuedAt]
  properties:
    id:
      type: string
    identityId:
      type: string
    type:
      type: string
      enum: [passkey, oauth, social_vouch, authority, temporal_reputation]
    issuer:
      type: string
    status:
      type: string
      enum: [active, revoked, expired]
    issuedAt:
      type: string
      format: date-time
    expiresAt:
      type: string
      format: date-time
      nullable: true
    metadata:
      type: object
      additionalProperties: true
```

### Subject

```yaml
Subject:
  type: object
  required: [id, title, createdAt]
  properties:
    id:
      type: string
    title:
      type: string
    slug:
      type: string
      nullable: true
    summary:
      type: string
      nullable: true
    createdAt:
      type: string
      format: date-time
```

### ContentItem

```yaml
ContentItem:
  type: object
  required:
    [id, authorIdentityId, mode, status, visibility, createdAt, updatedAt]
  properties:
    id:
      type: string
    authorIdentityId:
      type: string
    subjectId:
      type: string
      nullable: true
    mode:
      type: string
      enum: [murmur, idea, discussion]
    title:
      type: string
      nullable: true
    body:
      type: string
    status:
      type: string
      enum: [draft, active, archived, removed]
    visibility:
      type: string
      enum: [private, unlisted, public]
    trustTierRequired:
      type: integer
      minimum: 0
      maximum: 4
      nullable: true
    participationPolicy:
      type: string
      enum: [read_only, comment, debate]
      nullable: true
    createdAt:
      type: string
      format: date-time
    updatedAt:
      type: string
      format: date-time
    publishedAt:
      type: string
      format: date-time
      nullable: true
```

### Murmur

```yaml
Murmur:
  allOf:
    - $ref: '#/components/schemas/ContentItem'
    - type: object
      properties:
        tone:
          type: string
          enum: [note, question, intuition, reaction]
        sourceType:
          type: string
          enum: [typed, voice, import]
        isSensitive:
          type: boolean
        privateTags:
          type: array
          items:
            type: string
```

### Idea

```yaml
Idea:
  allOf:
    - $ref: '#/components/schemas/ContentItem'
    - type: object
      properties:
        thesis:
          type: string
          nullable: true
        outline:
          type: array
          items:
            type: string
        assistantPersona:
          type: string
          nullable: true
```

### Discussion

```yaml
Discussion:
  allOf:
    - $ref: '#/components/schemas/ContentItem'
    - type: object
      properties:
        discussionShape:
          type: string
          enum: [thread, stance_map, debate_cards]
        forkPolicy:
          type: string
          enum: [disabled, allowed, encouraged]
        consensusState:
          type: string
          enum: [none, emerging, contested, stable]
```

### ContentRelation

```yaml
ContentRelation:
  type: object
  required: [id, fromContentItemId, toContentItemId, relationType, createdAt]
  properties:
    id:
      type: string
    fromContentItemId:
      type: string
    toContentItemId:
      type: string
    relationType:
      type: string
      enum:
        [expanded_from, projected_from, summarized_from, forked_from, supports, rebuts, references]
    createdAt:
      type: string
      format: date-time
```

### TransformationJob

```yaml
TransformationJob:
  type: object
  required:
    [id, requestedByIdentityId, targetMode, providerType, status, createdAt]
  properties:
    id:
      type: string
    requestedByIdentityId:
      type: string
    sourceContentItemIds:
      type: array
      items:
        type: string
    targetMode:
      type: string
      enum: [murmur, idea, discussion]
    providerType:
      type: string
      enum: [local_llm, byok, system_llm]
    promptProfile:
      type: string
      nullable: true
    status:
      type: string
      enum: [queued, running, completed, failed, discarded]
    inputSnapshot:
      type: object
      additionalProperties: true
    outputSnapshot:
      type: object
      additionalProperties: true
      nullable: true
    createdAt:
      type: string
      format: date-time
    completedAt:
      type: string
      format: date-time
      nullable: true
```

### Projection

```yaml
Projection:
  type: object
  required:
    [id, sourceIdeaId, targetDiscussionId, participationPolicy, createdAt]
  properties:
    id:
      type: string
    sourceIdeaId:
      type: string
    targetDiscussionId:
      type: string
    projectedExcerpt:
      type: string
    participationPolicy:
      type: string
      enum: [read_only, comment, debate]
    ownershipTransferAcknowledged:
      type: boolean
    createdAt:
      type: string
      format: date-time
```

### DiscussionNode

```yaml
DiscussionNode:
  type: object
  required:
    [id, discussionId, authorIdentityId, nodeType, stance, body, createdAt]
  properties:
    id:
      type: string
    discussionId:
      type: string
    parentNodeId:
      type: string
      nullable: true
    authorIdentityId:
      type: string
    nodeType:
      type: string
      enum: [claim, question, evidence, rebuttal, summary]
    stance:
      type: string
      enum: [support, oppose, clarify, neutral]
    body:
      type: string
    createdAt:
      type: string
      format: date-time
```

### Fork

```yaml
Fork:
  type: object
  required: [id, sourceDiscussionId, rootNodeId, createdByIdentityId, createdAt]
  properties:
    id:
      type: string
    sourceDiscussionId:
      type: string
    rootNodeId:
      type: string
    createdByIdentityId:
      type: string
    reason:
      type: string
      nullable: true
    createdAt:
      type: string
      format: date-time
```

### ConsensusSnapshot

```yaml
ConsensusSnapshot:
  type: object
  required: [id, discussionId, generatedBy, summary, createdAt]
  properties:
    id:
      type: string
    discussionId:
      type: string
    generatedBy:
      type: string
      enum: [system_llm, moderator]
    summary:
      type: string
    agreements:
      type: array
      items:
        type: string
    disagreements:
      type: array
      items:
        type: string
    openQuestions:
      type: array
      items:
        type: string
    sourceNodeIds:
      type: array
      items:
        type: string
    createdAt:
      type: string
      format: date-time
```

## Endpoint Draft

### Identity

#### `POST /v2/auth/oauth`

Purpose:
Issue or retrieve an L0 identity from OAuth login.

Request:

```json
{
  "provider": "google",
  "token": "opaque-or-jwt-token"
}
```

Response:

```json
{
  "identity": {
    "id": "idn_123",
    "did": "oauth:google:abc",
    "displayName": "Alice",
    "trustTier": 0,
    "status": "active",
    "createdAt": "2026-04-16T10:00:00Z"
  },
  "sessionToken": "jwt-or-cookie-session"
}
```

#### `GET /v2/me`

Purpose:
Return the authenticated identity, trust tier, and capabilities.

Response:

```json
{
  "identity": {
    "id": "idn_123",
    "did": "did:vflow:abcd",
    "displayName": "Alice",
    "trustTier": 2,
    "status": "active",
    "createdAt": "2026-04-16T10:00:00Z"
  },
  "capabilities": {
    "canCreateIdea": true,
    "canProjectToDiscussion": true,
    "canForkDiscussion": true,
    "canInitiateModeration": false
  }
}
```

### Content

#### `GET /v2/content-items`

Purpose:
Search content across modes with filters.

Query parameters:

- `mode`
- `subjectId`
- `authorIdentityId`
- `visibility`
- `status`
- `limit`
- `cursor`

#### `POST /v2/content-items`

Purpose:
Create a content item in any mode.

Request:

```json
{
  "mode": "murmur",
  "title": null,
  "body": "短句觀察",
  "visibility": "private",
  "subjectId": null,
  "metadata": {
    "tone": "intuition",
    "sourceType": "typed",
    "privateTags": ["personal"]
  }
}
```

Response:

```json
{
  "contentItem": {
    "id": "cnt_123",
    "authorIdentityId": "idn_123",
    "mode": "murmur",
    "body": "短句觀察",
    "status": "draft",
    "visibility": "private",
    "createdAt": "2026-04-16T10:00:00Z",
    "updatedAt": "2026-04-16T10:00:00Z"
  }
}
```

#### `GET /v2/content-items/{contentItemId}`

Purpose:
Retrieve one content item plus mode-specific fields and lineage.

#### `PATCH /v2/content-items/{contentItemId}`

Purpose:
Edit creator-owned content or update public content within policy limits.

#### `DELETE /v2/content-items/{contentItemId}`

Purpose:
Delete content when permitted by ownership policy.

Expected responses:

- `204` if allowed and performed
- `403` if ownership has been transferred or restricted

### Mode-Specific Feeds

#### `GET /v2/murmurs`

Purpose:
Return murmur-focused reading view.

#### `GET /v2/ideas`

Purpose:
Return idea-focused reading view.

#### `GET /v2/discussions`

Purpose:
Return public discourse surfaces.

Recommended query parameters:

- `subjectId`
- `authorIdentityId`
- `discussionShape`
- `participationPolicy`
- `sort`
- `limit`
- `cursor`

### Transformations

#### `POST /v2/transformation-jobs`

Purpose:
Create an AI-assisted draft transformation.

Request:

```json
{
  "sourceContentItemIds": ["cnt_1", "cnt_2"],
  "targetMode": "idea",
  "providerType": "byok",
  "promptProfile": "researcher"
}
```

Response:

```json
{
  "job": {
    "id": "trf_123",
    "requestedByIdentityId": "idn_123",
    "sourceContentItemIds": ["cnt_1", "cnt_2"],
    "targetMode": "idea",
    "providerType": "byok",
    "status": "queued",
    "createdAt": "2026-04-16T10:00:00Z"
  }
}
```

#### `GET /v2/transformation-jobs/{jobId}`

Purpose:
Poll job status and retrieve the generated draft.

#### `POST /v2/transformation-jobs/{jobId}/publish`

Purpose:
Accept a completed transformation output and create a new content item.

This endpoint should require explicit consent.

### Projection

#### `POST /v2/projections`

Purpose:
Project an idea or excerpt into the public discussion layer.

Request:

```json
{
  "sourceIdeaId": "cnt_idea_1",
  "projectedExcerpt": "The claim I want to open for debate",
  "participationPolicy": "debate",
  "ownershipTransferAcknowledged": true,
  "discussionShape": "stance_map"
}
```

Response:

```json
{
  "projection": {
    "id": "prj_123",
    "sourceIdeaId": "cnt_idea_1",
    "targetDiscussionId": "cnt_disc_1",
    "projectedExcerpt": "The claim I want to open for debate",
    "participationPolicy": "debate",
    "ownershipTransferAcknowledged": true,
    "createdAt": "2026-04-16T10:00:00Z"
  }
}
```

### Discussion Graph

#### `GET /v2/discussions/{discussionId}`

Purpose:
Return public discussion metadata and debate configuration.

#### `GET /v2/discussions/{discussionId}/nodes`

Purpose:
Return the node tree or graph for a discussion.

Response:

```json
{
  "discussionId": "cnt_disc_1",
  "nodes": [
    {
      "id": "node_1",
      "discussionId": "cnt_disc_1",
      "parentNodeId": null,
      "authorIdentityId": "idn_1",
      "nodeType": "claim",
      "stance": "neutral",
      "body": "核心主張",
      "createdAt": "2026-04-16T10:00:00Z"
    }
  ]
}
```

#### `POST /v2/discussions/{discussionId}/nodes`

Purpose:
Create a debate node.

Request:

```json
{
  "parentNodeId": "node_1",
  "nodeType": "rebuttal",
  "stance": "oppose",
  "body": "我不同意，理由如下"
}
```

### Forks

#### `POST /v2/discussions/{discussionId}/forks`

Purpose:
Create a derivative community-owned discussion branch.

### Consensus Summaries

#### `POST /v2/discussions/{discussionId}/consensus-snapshots`

Purpose:
Generate or request a consensus/divergence summary for public discussion.

#### `GET /v2/discussions/{discussionId}/consensus-snapshots`

Purpose:
List previous public summaries.

### Interactions

#### `POST /v2/interactions`

Purpose:
Record a weighted interaction.

Request:

```json
{
  "targetContentItemId": "cnt_disc_1",
  "interactionType": "endorse"
}
```

Possible interaction types:

- `endorse`
- `rebut`
- `bookmark`
- `cite`
- `follow`

### Moderation

#### `POST /v2/moderation-actions`

Purpose:
Initiate a moderation or slashing flow.

Request:

```json
{
  "targetContentItemId": "cnt_disc_1",
  "actionType": "flag",
  "reason": "Coordinated misinformation"
}
```

## Capability and Policy Responses

To keep the frontend simple, APIs that return content should also expose policy
snapshots:

```yaml
ContentPolicySnapshot:
  type: object
  properties:
    canEdit:
      type: boolean
    canDelete:
      type: boolean
    canComment:
      type: boolean
    canFork:
      type: boolean
    canModerate:
      type: boolean
```

This is especially important because Aleth changes permissions based on trust
tier and ownership transfer.

## Migration from Current API

### Current state

- `POST /auth/oauth`
- `GET /posts`
- `POST /posts`

### Recommended bridge

- Map current `Post` to `ContentItem` with `mode=discussion`
- Keep `GET /posts` as a compatibility feed for public discussions
- Add `mode`, `status`, and `visibility` to the underlying model before
  expanding the full API surface

## First Implementation Slice

If the team wants the smallest viable vertical slice for `v2`, implement these
endpoints first:

1. `GET /v2/me`
2. `POST /v2/content-items`
3. `GET /v2/content-items/{id}`
4. `POST /v2/transformation-jobs`
5. `GET /v2/transformation-jobs/{id}`
6. `POST /v2/projections`
7. `GET /v2/discussions/{id}`
8. `GET /v2/discussions/{id}/nodes`
9. `POST /v2/discussions/{id}/nodes`

That slice is enough to support:

- murmur creation
- idea generation
- public projection
- structured discussion replies
