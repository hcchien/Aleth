# Aleth

A trust-centric social platform combining short-form discussion (Threads-style) with long-form writing (Substack-style), with BBS-inspired personal boards and multi-level identity verification.

## Repository Structure

```
aleth/
├── apps/
│   ├── web/              # Next.js main site (aleth.app)
│   └── admin/            # Next.js Admin Tool (admin.aleth.app)
├── services/
│   ├── gateway/          # API Gateway (GraphQL schema stitching)
│   ├── auth/             # Auth Service (login, passkey, VC)
│   ├── content/          # Content Service (posts, articles, boards)
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
| Backend | Go, Chi, gqlgen, pgx v5, sqlc, goose |
| Database | PostgreSQL (Cloud SQL), Redis (Memorystore) |
| Cloud | GCP — GKE, Cloud Build, Secret Manager, GCS |

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
go run ./services/gateway/cmd/gateway
```

## Documentation

- [Product Spec](docs/spec.md)
- [Architecture Design](docs/architecture.md)
