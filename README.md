# Aleth

A trust-centric social platform combining short-form discussion (Threads-style) with long-form writing (Substack-style), with BBS-inspired personal boards, multi-level identity verification, ActivityPub federation, and Facebook-style fan pages.

## Features

### Core Content
- **Posts** — short-form threaded discussions with replies and reshares
- **Articles** — long-form writing published to personal boards
- **Boards** — per-user publication spaces with configurable access and comment policy
- **Feed** — personalised home feed based on follows and trust level

### Trust & Identity
- **Trust levels** — reputation score (0–4) gates access to boards and pages
- **Verifiable Credentials (VC)** — optional VC-based access control on boards and pages; supports multiple VC types per gate
- **Passkey / email auth** — passwordless login via WebAuthn passkeys

### ActivityPub Federation
- Per-user opt-in federation toggle — user accounts become AP `Person` actors at `/@username`
- **HTTP Signatures** — outbound activities signed with per-user RSA-2048 keys stored in `actor_keys`
- **WebFinger** — `acct:username@domain` discovery
- **Inbox** — handles `Follow` / `Undo` from remote servers; stores remote followers
- **Outbox** — serves `OrderedCollection` of public posts as AP `Note` activities
- **Delivery queue** — fan-out to remote followers' inboxes on new post creation

### Fan Pages
- **Multi-admin pages** at `/p/{slug}` — standalone team-managed spaces separate from personal boards
- **Roles** — `admin` (full control) and `editor` (publish only); last-admin guard prevents lockout
- **Content** — publish posts and articles under the page's identity
- **Policy** — configurable `default_access`, `comment_policy`, `min_trust_level`, `min_comment_trust`, and VC gates (mirrors board policy)
- **Followers** — local users follow pages; local follower count tracked separately from user follows
- **ActivityPub pages** — optional AP toggle makes a page a `Group` actor at `/p/{slug}`; WebFinger resource `acct:p.{slug}@domain`; remote followers receive new page posts via signed `Create` activities

## Repository Structure

```
aleth/
├── apps/
│   ├── web/              # Next.js main site (aleth.app)
│   └── admin/            # Next.js Admin Tool (admin.aleth.app)
├── services/
│   ├── gateway/          # API Gateway (GraphQL schema stitching)
│   ├── auth/             # Auth Service (login, passkey, VC)
│   ├── content/          # Content Service (posts, articles, boards, fan pages)
│   ├── federation/       # ActivityPub Federation Service
│   └── feed/             # Feed/Reach Service
├── proto/                # Protobuf definitions (gRPC inter-service)
├── migrations/           # DB migrations (goose, per service)
├── infra/                # Docker, Kubernetes, Cloud Build configs
├── docs/                 # Architecture and spec documents
├── go.work               # Go workspace
└── pnpm-workspace.yaml   # pnpm monorepo config
```

## Tech Stack

| Layer | Tech |
|-------|------|
| Frontend | Next.js 15 (App Router), Tailwind CSS, shadcn/ui, Apollo Client |
| Backend | Go, Chi, graph-gophers/graphql-go, pgx v5, goose |
| Database | PostgreSQL (Cloud SQL), Redis (Memorystore) |
| Cloud | GCP — GKE, Cloud Build, Secret Manager, GCS |
| Federation | ActivityPub (JSON-LD), HTTP Signatures (RFC 9421), WebFinger (RFC 7033) |

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 22+
- pnpm 10+
- Docker

### Development

```bash
# Frontend
pnpm dev:web     # Start main site at localhost:3000
pnpm dev:admin   # Start admin tool at localhost:3001

# Backend (each service)
go run ./services/auth/cmd/auth
go run ./services/content/cmd/content
go run ./services/federation/cmd/federation
go run ./services/gateway/cmd/gateway
```

## Documentation

- [Product Spec](docs/spec.md)
- [Architecture Design](docs/architecture.md)
