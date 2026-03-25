# Deployment Guide

This guide covers deploying every Aleth service — from local development to production on GCP.

---

## Architecture Overview

```
Internet
    │
    ▼
┌─────────────┐     ┌──────────────────────────────────────────┐
│  Next.js    │────▶│  Gateway  :8080 / :4000 (dev)           │
│  web  :3000 │     │  Single GraphQL entry point (chi/cors)   │
└─────────────┘     └──────┬──────────┬──────────┬────────────┘
                           │          │          │
              ┌────────────┘  ┌───────┘  ┌──────┘
              ▼               ▼          ▼
        ┌──────────┐   ┌──────────┐  ┌──────────┐   ┌──────────────┐
        │  Auth    │   │ Content  │  │   Feed   │   │ Notification │
        │  :8081   │   │  :8082   │  │  :8083   │   │   :8086      │
        └────┬─────┘   └────┬─────┘  └────┬─────┘   └──────────────┘
             │              │              │
        ┌────▼──────┐  ┌────▼──────┐  ┌───▼───────┐
        │  auth DB  │  │content DB │  │ auth DB + │
        │(Postgres) │  │(Postgres) │  │content DB │
        └───────────┘  └─────┬─────┘  └───────────┘
                             │
                    ┌────────▼────────┐
                    │  GCP Pub/Sub    │  (optional)
                    │ content-events  │
                    └────┬────────────┘
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
        ┌──────────┐         ┌──────────┐
        │Notification│       │ Counter  │
        │(subscriber)│       │(worker)  │
        └──────────┘         └──────────┘

  ┌─────────────┐
  │ Federation  │  :8084  (ActivityPub — optional for MVP)
  │  :8084      │
  └─────────────┘
```

**Databases:** Each service owns its own Postgres database (schema isolation).
**Auth token:** A single `ACCESS_TOKEN_SECRET` is shared across Auth, Content, Feed, Notification, and Gateway for local JWT validation.
**No service mesh:** Services call each other via plain HTTP.

---

## Local Development

### Prerequisites

```bash
# Go 1.24+
go version

# Node 22 + pnpm
node --version   # 22.x
pnpm --version

# PostgreSQL 14
pg_ctl --version

# goose (DB migration tool)
CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.19.2
```

### One-command start

```bash
# Start all backend services (auth, content, feed, notification, gateway)
./dev.sh

# In a second terminal — start the web app
pnpm --filter web dev        # → http://localhost:3000

# Stop all services
./dev.sh stop
```

`dev.sh` does the following automatically:

1. Starts PostgreSQL 14 (if not already running)
2. Runs goose migrations for auth, content, and notification databases
3. Builds all services with `CGO_ENABLED=0`
4. Launches them as background processes, writing PIDs to `.dev-pids`
5. Waits for each port to open before starting the next service

### Service .env files

Each service reads its config from a `.env` file in its directory. Create these before running `dev.sh`:

```
services/auth/.env
services/content/.env
services/feed/.env
services/gateway/.env
services/notification/.env
```

Minimum contents for local dev:

**`services/auth/.env`**
```dotenv
AUTH_DATABASE_URL=postgres://localhost:5432/aleth_auth?sslmode=disable
AUTH_ACCESS_TOKEN_SECRET=dev-access-secret-change-in-prod
AUTH_REFRESH_TOKEN_SECRET=dev-refresh-secret-change-in-prod
AUTH_PASSKEY_RP_ID=localhost
AUTH_FRONTEND_URL=http://localhost:3000
```

**`services/content/.env`**
```dotenv
CONTENT_DATABASE_URL=postgres://localhost:5432/aleth_content?sslmode=disable
CONTENT_ACCESS_TOKEN_SECRET=dev-access-secret-change-in-prod
```

**`services/feed/.env`**
```dotenv
FEED_AUTH_DATABASE_URL=postgres://localhost:5432/aleth_auth?sslmode=disable
FEED_CONTENT_DATABASE_URL=postgres://localhost:5432/aleth_content?sslmode=disable
FEED_ACCESS_TOKEN_SECRET=dev-access-secret-change-in-prod
```

**`services/notification/.env`**
```dotenv
NOTIFICATION_DATABASE_URL=postgres://localhost:5432/aleth_notification?sslmode=disable
NOTIFICATION_ACCESS_TOKEN_SECRET=dev-access-secret-change-in-prod
```

**`services/gateway/.env`**
```dotenv
GATEWAY_AUTH_SERVICE_URL=http://localhost:8081
GATEWAY_CONTENT_SERVICE_URL=http://localhost:8082
GATEWAY_FEED_SERVICE_URL=http://localhost:8083
GATEWAY_NOTIFICATION_URL=http://localhost:8086
GATEWAY_ALLOWED_ORIGIN=http://localhost:3000
```

**`apps/web/.env.local`**
```dotenv
GATEWAY_URL=http://localhost:4000
NEXT_PUBLIC_API_URL=/graphql
NOTIFICATION_URL=http://localhost:8086
```

### Create local databases

```bash
createdb aleth_auth
createdb aleth_content
createdb aleth_notification
createdb aleth_federation   # only if running federation locally
```

---

## Database Migrations

Migrations use [goose](https://github.com/pressly/goose). Each database has its own migration directory.

| Database | Migration dir | Goose table (default unless noted) |
|---|---|---|
| Auth | `migrations/auth/` | `goose_db_version` |
| Content | `migrations/content/` | `goose_db_version` |
| Notification | `migrations/notification/` | `goose_notification_versions` |
| Federation | `migrations/federation/` | `goose_db_version` |

### Run migrations manually

```bash
# Auth
goose -dir migrations/auth postgres "$AUTH_DATABASE_URL" up

# Content
goose -dir migrations/content postgres "$CONTENT_DATABASE_URL" up

# Notification (custom table name!)
goose -table goose_notification_versions \
      -dir migrations/notification \
      postgres "$NOTIFICATION_DATABASE_URL" up

# Federation
goose -dir migrations/federation postgres "$FEDERATION_DATABASE_URL" up
```

### Check migration status

```bash
goose -dir migrations/auth postgres "$AUTH_DATABASE_URL" status
```

### Create a new migration

```bash
goose -dir migrations/auth create add_something sql
# Edit the generated file, then run: goose ... up
```

> **Production rule:** Migrations always run before deploying new service images. The CI/CD pipeline enforces this order.

---

## Building Docker Images

All Dockerfiles are in `infra/docker/`. They use a two-stage build: compile on `golang:1.23-alpine`, run on `gcr.io/distroless/static-debian12` (no shell, minimal attack surface).

### Build locally

```bash
# From the repo root (Dockerfiles copy the whole services/ directory)
docker build -f infra/docker/auth.Dockerfile       -t aleth/auth:local .
docker build -f infra/docker/content.Dockerfile    -t aleth/content:local .
docker build -f infra/docker/gateway.Dockerfile    -t aleth/gateway:local .
docker build -f infra/docker/federation.Dockerfile -t aleth/federation:local .
```

All eight Dockerfiles are in `infra/docker/`:

| Dockerfile | Service | Port |
|---|---|---|
| `auth.Dockerfile` | Auth | 8081 |
| `content.Dockerfile` | Content | 8082 |
| `feed.Dockerfile` | Feed | 8083 |
| `gateway.Dockerfile` | Gateway | 8080 |
| `notification.Dockerfile` | Notification | 8086 |
| `counter.Dockerfile` | Counter | 8080 (healthz only) |
| `federation.Dockerfile` | Federation | 8084 |
| `web.Dockerfile` | Next.js web app | 3000 |

---

## CI/CD Pipeline (GCP Cloud Build)

The pipeline is defined in `infra/cloudbuild.yaml` and runs on every push to `main`.
It builds every service and deploys to **Cloud Run**.

### Pipeline stages (in order)

```
[parallel] Tests + lint
  go-test-auth       go-test-feed          next-lint-web
  go-test-content    go-test-notification  next-lint-admin
  go-test-federation go-test-counter
  go-test-gateway
       │
       ▼
[parallel] DB Migrations (each waits for its own test)
  db-migrate-auth        (waitFor: go-test-auth)
  db-migrate-content     (waitFor: go-test-content)
  db-migrate-federation  (waitFor: go-test-federation)
  db-migrate-notification (waitFor: go-test-notification) ← custom goose table
       │
       ▼
[parallel] Build Docker images (each waits for its migration or test)
  build-auth      build-content    build-federation  build-gateway
  build-feed      build-notification build-counter   build-web
       │
       ▼
[parallel] Push to Artifact Registry
  push-auth  push-content  push-federation  push-gateway
  push-feed  push-notification  push-counter  push-web
       │
       ▼
[parallel] Deploy to Cloud Run
  deploy-auth (internal)       deploy-gateway (public)
  deploy-content (internal)    deploy-federation (public)
  deploy-feed (internal)       deploy-web (public)
  deploy-notification (internal, min=1)
  deploy-counter (internal, min=1)
```

### Substitution variables

| Variable | Default | Override when triggering |
|---|---|---|
| `_REGION` | `asia-east1` | Your GCP region |
| `_ENV` | `production` | `staging` for a staging run |

### Trigger setup

```bash
# Connect your repo in Cloud Build console, then:
gcloud builds triggers create github \
  --repo-name=aleth \
  --repo-owner=YOUR_ORG \
  --branch-pattern='^main$' \
  --build-config=infra/cloudbuild.yaml \
  --substitutions=_REGION=asia-east1
```

### Grant Cloud Build permission to deploy Cloud Run

```bash
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')
CB_SA="$PROJECT_NUMBER@cloudbuild.gserviceaccount.com"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CB_SA" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CB_SA" \
  --role="roles/iam.serviceAccountUser"
```

### Secret Manager secrets (required for migrations)

```bash
# DB connection strings for goose migrations
echo -n "postgres://user:pass@host/aleth_auth" | \
  gcloud secrets create auth-db-url --data-file=-

echo -n "postgres://user:pass@host/aleth_content" | \
  gcloud secrets create content-db-url --data-file=-

echo -n "postgres://user:pass@host/aleth_federation" | \
  gcloud secrets create federation-db-url --data-file=-

echo -n "postgres://user:pass@host/aleth_notification" | \
  gcloud secrets create notification-db-url --data-file=-
```

Grant Cloud Build access to all four secrets:
```bash
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')
CB_SA="serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com"

for SECRET in auth-db-url content-db-url federation-db-url notification-db-url; do
  gcloud secrets add-iam-policy-binding $SECRET \
    --member="$CB_SA" \
    --role="roles/secretmanager.secretAccessor"
done
```

### First-time Cloud Run service configuration

Cloud Build only updates the image on each deploy — it preserves existing env vars.
**Before the first CI run**, create each service with its full env var set:

```bash
# Example: first deploy of auth with env vars
gcloud run deploy auth \
  --image=asia-east1-docker.pkg.dev/$PROJECT_ID/aleth/auth:initial \
  --region=asia-east1 \
  --platform=managed \
  --port=8081 \
  --no-allow-unauthenticated \
  --set-env-vars="AUTH_DATABASE_URL=postgres://...,AUTH_ACCESS_TOKEN_SECRET=...,..."

# After this, CI pipeline only passes --image; all other config is preserved.
```

See the **Service-by-Service Deployment** section below for the full env var list for each service.

### Goose Docker image for Cloud Build

The migration steps use `gcr.io/$PROJECT_ID/goose:latest`. Build and push it once:

```bash
cat > /tmp/Dockerfile.goose <<'EOF'
FROM golang:1.23-alpine
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.19.2
ENTRYPOINT ["goose"]
EOF

docker build -f /tmp/Dockerfile.goose -t asia-east1-docker.pkg.dev/$PROJECT_ID/aleth/goose:latest .
docker push asia-east1-docker.pkg.dev/$PROJECT_ID/aleth/goose:latest
```

---

## Service-by-Service Deployment

### Auth Service

**Port:** 8081
**Database:** Auth Postgres (`aleth_auth`)
**Dockerfile:** `infra/docker/auth.Dockerfile`
**Health check:** `GET /healthz` → `200 ok`

**What it does:** User registration, login (password, Google, Facebook, passkey), JWT issuance, password reset, email verification, OAuth credential management, reputation stamps, verifiable credentials.

**Required env vars for production:**
```dotenv
AUTH_DATABASE_URL=postgres://...
AUTH_ACCESS_TOKEN_SECRET=<64-char hex>
AUTH_REFRESH_TOKEN_SECRET=<64-char hex, different from above>
AUTH_PASSKEY_RP_ID=aleth.social
AUTH_FRONTEND_URL=https://aleth.social
AUTH_OAUTH_CALLBACK_BASE=https://aleth.social
AUTH_SMTP_HOST=smtp.mailgun.org
AUTH_SMTP_PORT=587
AUTH_SMTP_USER=postmaster@mg.aleth.social
AUTH_SMTP_PASSWORD=<key>
AUTH_SMTP_FROM=noreply@aleth.social
```

**Optional (enable social login):**
```dotenv
AUTH_GOOGLE_CLIENT_ID=...
AUTH_FACEBOOK_APP_ID=...
AUTH_FACEBOOK_CLIENT_SECRET=...
```

**Kubernetes deployment (example):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: auth
          image: asia-east1-docker.pkg.dev/PROJECT/aleth/auth:SHA
          ports:
            - containerPort: 8081
          livenessProbe:
            httpGet: { path: /healthz, port: 8081 }
          readinessProbe:
            httpGet: { path: /healthz, port: 8081 }
          envFrom:
            - secretRef: { name: auth-secrets }
```

---

### Content Service

**Port:** 8082
**Database:** Content Postgres (`aleth_content`)
**Dockerfile:** `infra/docker/content.Dockerfile`
**Health check:** `GET /healthz` → `200 ok`

**What it does:** Posts, articles, boards, fan pages, series, reactions, article comments. Optionally publishes events to GCP Pub/Sub.

**Required env vars:**
```dotenv
CONTENT_DATABASE_URL=postgres://...
CONTENT_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>
```

**Optional (enable Pub/Sub fan-out and federation):**
```dotenv
CONTENT_PUBSUB_ENABLED=true
CONTENT_PUBSUB_PROJECT_ID=my-gcp-project
CONTENT_PUBSUB_TOPIC=content-events
CONTENT_FEDERATION_URL=http://federation:8084
```

---

### Feed Service

**Port:** 8083
**Databases:** Auth DB (read-only, follow graph) + Content DB (read-only, posts)
**Dockerfile:** ⚠️ Not yet created — use template above
**Health check:** `GET /healthz` → `200 ok`

**What it does:** Aggregates the home feed and explore feed from the two databases. Read-only — never writes.

**Required env vars:**
```dotenv
FEED_AUTH_DATABASE_URL=postgres://...
FEED_CONTENT_DATABASE_URL=postgres://...
FEED_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>
```

> **Performance note:** Feed runs read-only queries across two databases. Give it a connection pool to read replicas in production if load is high.

---

### Gateway Service

**Port:** 8080 (production) / 4000 (dev)
**Databases:** None — calls downstream services
**Dockerfile:** `infra/docker/gateway.Dockerfile`
**Health check:** `GET /healthz` → `200 ok`
**GraphQL:** `POST /graphql`

**What it does:** Aggregates GraphQL from Auth, Content, Feed, Notification, and Federation. This is the only service exposed to the internet.

**Required env vars:**
```dotenv
GATEWAY_AUTH_SERVICE_URL=http://auth:8081
GATEWAY_CONTENT_SERVICE_URL=http://content:8082
GATEWAY_FEED_SERVICE_URL=http://feed:8083
GATEWAY_NOTIFICATION_URL=http://notification:8086
GATEWAY_ALLOWED_ORIGIN=https://aleth.social
```

**Optional:**
```dotenv
GATEWAY_FEDERATION_URL=http://federation:8084
GATEWAY_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>
```

**Internet exposure:** Gateway is the only service that should be reachable from outside the cluster. All other services should be cluster-internal only.

---

### Notification Service

**Port:** 8086
**Database:** Notification Postgres (`aleth_notification`)
**Dockerfile:** ⚠️ Not yet created — use template above
**Health check:** `GET /healthz` → `200 ok`
**Migration table:** `goose_notification_versions` (non-default!)

**What it does:** Stores and delivers in-app notifications. Subscribes to GCP Pub/Sub `content-events` to receive new post/reply events. Also supports page-follower fan-out when given access to the content DB.

**Required env vars:**
```dotenv
NOTIFICATION_DATABASE_URL=postgres://...
NOTIFICATION_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>
```

**Optional (enable Pub/Sub):**
```dotenv
NOTIFICATION_PUBSUB_PROJECT_ID=my-gcp-project
NOTIFICATION_PUBSUB_SUBSCRIPTION=notification-content-events
NOTIFICATION_CONTENT_DATABASE_URL=postgres://...   # for page fan-out
```

**Migration note:** Must use the custom table name when running migrations:
```bash
goose -table goose_notification_versions \
      -dir migrations/notification \
      postgres "$NOTIFICATION_DATABASE_URL" up
```

---

### Federation Service (ActivityPub)

**Port:** 8084
**Database:** Federation Postgres (`aleth_federation`)
**Dockerfile:** `infra/docker/federation.Dockerfile`
**Health check:** `GET /healthz` → `200 ok`

**What it does:** Implements ActivityPub (Fediverse). Handles WebFinger lookups, actor profiles, inbox/outbox, follow/accept flows. Required for federation with Mastodon, Threads, etc.

**This service is optional for an MVP.** Disable it by simply not deploying it and leaving `GATEWAY_FEDERATION_URL` unset.

**Required env vars:**
```dotenv
FEDERATION_DATABASE_URL=postgres://...
FEDERATION_DOMAIN=aleth.social
FEDERATION_PLATFORM_KEY_SECRET=<32-byte hex from: openssl rand -hex 32>
```

**Optional:**
```dotenv
FEDERATION_AUTH_URL=http://auth:8081
FEDERATION_CONTENT_URL=http://content:8082
FEDERATION_PORT=8084
```

**Never set in production:**
```dotenv
# FEDERATION_SKIP_SIG_VERIFY=true   ← dev only
```

**DNS requirement:** The domain specified in `FEDERATION_DOMAIN` must serve WebFinger at `https://aleth.social/.well-known/webfinger`. Route this to the federation service.

---

### Counter Service

**Port:** none (worker only, no HTTP server)
**Database:** Counter Postgres (optional)
**Dockerfile:** ⚠️ Not yet created — use template above

**What it does:** Background Pub/Sub worker that processes `content-events` for analytics and counters. Has no GraphQL endpoint. Does not need to be exposed.

**Required env vars:**
```dotenv
COUNTER_PUBSUB_PROJECT_ID=my-gcp-project
COUNTER_PUBSUB_SUBSCRIPTION=counter-content-events
```

**Optional:**
```dotenv
COUNTER_DATABASE_URL=postgres://...
```

---

### Next.js Web App (`apps/web`)

**Port:** 3000
**Build:** `pnpm --filter web build`
**Start:** `pnpm --filter web start`

**What it does:** Public-facing Next.js app. Proxies GraphQL to the gateway and notifications to the notification service via `next.config.ts` rewrites.

**Build and run with Docker:**

```dockerfile
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN corepack enable && pnpm install --frozen-lockfile

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/apps/web/node_modules ./apps/web/node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm --filter web build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
```

**Required env vars at runtime:**
```dotenv
GATEWAY_URL=http://gateway:8080
NOTIFICATION_URL=http://notification:8086
NEXT_PUBLIC_API_URL=/graphql
```

**Optional (social login buttons):**
```dotenv
NEXT_PUBLIC_GOOGLE_CLIENT_ID=...
NEXT_PUBLIC_FACEBOOK_APP_ID=...
```

**Image uploads (GCS):**
```dotenv
GCS_BUCKET=aleth-uploads
GCS_KEY_FILE=/secrets/gcs-key.json   # or use Workload Identity (ADC)
```

> **Standalone output:** Add `output: "standalone"` to `next.config.ts` to get a minimal `server.js` bundle suitable for Docker. Without this the Docker image must include all `node_modules`.

---

## GCP Infrastructure Setup

### One-time GCP setup

```bash
export PROJECT_ID=my-aleth-project
export REGION=asia-east1

# Enable APIs
gcloud services enable \
  container.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  secretmanager.googleapis.com \
  pubsub.googleapis.com

# Create Artifact Registry repo
gcloud artifacts repositories create aleth \
  --repository-format=docker \
  --location=$REGION

# Create GKE cluster (adjust machine type and node count)
gcloud container clusters create aleth-production \
  --region=$REGION \
  --machine-type=e2-standard-2 \
  --num-nodes=2 \
  --workload-pool=$PROJECT_ID.svc.id.goog

# Create Pub/Sub topic and subscriptions (if using events)
gcloud pubsub topics create content-events
gcloud pubsub subscriptions create notification-content-events --topic=content-events
gcloud pubsub subscriptions create counter-content-events --topic=content-events

# Create GCS bucket for image uploads
gcloud storage buckets create gs://aleth-uploads \
  --location=$REGION \
  --public-access-prevention=enforced
```

### Store all secrets in Secret Manager

```bash
# Generate secrets
ACCESS_SECRET=$(openssl rand -hex 32)
REFRESH_SECRET=$(openssl rand -hex 32)
FEDERATION_KEY=$(openssl rand -hex 32)

echo -n "$ACCESS_SECRET" | gcloud secrets create auth-access-token-secret --data-file=-
echo -n "$REFRESH_SECRET" | gcloud secrets create auth-refresh-token-secret --data-file=-
echo -n "$FEDERATION_KEY" | gcloud secrets create federation-platform-key --data-file=-

# DB connection strings
echo -n "postgres://..." | gcloud secrets create auth-db-url --data-file=-
echo -n "postgres://..." | gcloud secrets create content-db-url --data-file=-
echo -n "postgres://..." | gcloud secrets create notification-db-url --data-file=-
echo -n "postgres://..." | gcloud secrets create federation-db-url --data-file=-

# SMTP (Mailgun)
echo -n "smtp.mailgun.org" | gcloud secrets create smtp-host --data-file=-
echo -n "postmaster@mg.aleth.social" | gcloud secrets create smtp-user --data-file=-
echo -n "<mailgun-smtp-password>" | gcloud secrets create smtp-password --data-file=-
```

---

## Health Checks Summary

| Service | Health endpoint | Port |
|---|---|---|
| Auth | `GET /healthz` | 8081 |
| Content | `GET /healthz` | 8082 |
| Feed | `GET /healthz` | 8083 |
| Gateway | `GET /healthz` | 8080 |
| Notification | `GET /healthz` | 8086 |
| Federation | `GET /healthz` | 8084 |
| Counter | _(worker only, no HTTP)_ | — |
| Web | `GET /` | 3000 |

All health endpoints return HTTP 200 with body `ok`.

---

## Deployment Checklist

### Before first deploy

- [ ] All databases created and migration user has permissions
- [ ] All migrations run successfully (`goose ... status` shows all applied)
- [ ] `AUTH_ACCESS_TOKEN_SECRET` value confirmed identical across Auth, Content, Feed, Notification, Gateway
- [ ] `AUTH_PASSKEY_RP_ID` set to your real domain (not `localhost`)
- [ ] `GATEWAY_ALLOWED_ORIGIN` set to your frontend URL (not `*`)
- [ ] `AUTH_FRONTEND_URL` and `AUTH_OAUTH_CALLBACK_BASE` set to production URLs
- [ ] `FEDERATION_PLATFORM_KEY_SECRET` generated with `openssl rand -hex 32`
- [ ] SMTP credentials tested (send a test email)
- [ ] GCS bucket created and service account has `roles/storage.objectCreator`
- [ ] Dockerfiles created for feed, notification, counter services
- [ ] Cloud Build trigger connected to main branch

### Per deploy

- [ ] Tests pass in CI
- [ ] Migrations applied before images deployed
- [ ] Health checks passing after deploy
- [ ] Verify `/healthz` on each service
- [ ] Smoke test: register a new user, check verification email arrives

---

## Monitoring (Cloud Monitoring)

### Enable required APIs

```bash
gcloud services enable monitoring.googleapis.com \
                       cloudmonitoring.googleapis.com
```

### Uptime checks

Create an uptime check for each public-facing service. Cloud Run exposes HTTPS, so use `https` protocol.

```bash
# Gateway GraphQL endpoint
gcloud monitoring uptime-checks create \
  --display-name="Gateway /healthz" \
  --resource-type=uptime-url \
  --hostname=gateway-HASH-uc.a.run.app \
  --path=/healthz \
  --protocol=HTTPS \
  --period=60 \
  --timeout=10

# Next.js web app (checks the root page)
gcloud monitoring uptime-checks create \
  --display-name="Web /" \
  --resource-type=uptime-url \
  --hostname=aleth.social \
  --path=/ \
  --protocol=HTTPS \
  --period=60 \
  --timeout=10
```

> In the Cloud Console you can also create uptime checks at **Monitoring → Uptime checks → Create uptime check**. Use a monitored resource type of "URL" and check every 1 minute from multiple regions.

### Alert policies

#### 1 — Uptime failure (service down)

Fires when an uptime check fails from 2 or more regions simultaneously.

```bash
# Get the uptime check IDs first
gcloud monitoring uptime-checks list

# Create alert policy (replace CHECK_ID with the ID from the list command)
gcloud alpha monitoring policies create \
  --display-name="Aleth: service down" \
  --condition-display-name="Uptime check failure" \
  --condition-filter='metric.type="monitoring.googleapis.com/uptime_check/check_passed" AND resource.type="uptime_url"' \
  --condition-threshold-value=1 \
  --condition-threshold-comparison=COMPARISON_LT \
  --condition-duration=120s \
  --notification-channels="projects/$PROJECT_ID/notificationChannels/CHANNEL_ID"
```

> It is easier to create alert policies in the Cloud Console (**Monitoring → Alerting → Create policy**) and link them to existing uptime checks. Recommended setup:
>
> - **Condition:** Uptime health check → check fails in ≥ 2 regions
> - **Duration:** 2 minutes
> - **Notification channel:** Email or PagerDuty

#### 2 — High error rate (5xx) on Cloud Run

```bash
# Alert when Gateway has >1% 5xx rate over a 5-minute window
gcloud alpha monitoring policies create \
  --display-name="Aleth: high gateway error rate" \
  --condition-display-name="Gateway 5xx > 1%" \
  --condition-filter='resource.type="cloud_run_revision" AND resource.labels.service_name="gateway" AND metric.type="run.googleapis.com/request_count" AND metric.labels.response_code_class="5xx"' \
  --condition-threshold-value=0.01 \
  --condition-threshold-comparison=COMPARISON_GT \
  --condition-duration=300s \
  --notification-channels="projects/$PROJECT_ID/notificationChannels/CHANNEL_ID"
```

#### 3 — High CPU or memory on Cloud Run

Add these from the Console:

- **Metric:** `run.googleapis.com/container/cpu/utilization` > 0.8 for 5 minutes
- **Metric:** `run.googleapis.com/container/memory/utilizations` > 0.85 for 5 minutes

#### 4 — Database connection exhaustion

If using Cloud SQL, alert when active connections exceed 80% of `max_connections`:

```
metric.type="cloudsql.googleapis.com/database/postgresql/num_backends"
> 0.8 * max_connections for 5 minutes
```

### Notification channels

Create an email notification channel (replace with your address):

```bash
gcloud alpha monitoring channels create \
  --display-name="Aleth on-call email" \
  --type=email \
  --channel-labels=email_address=oncall@aleth.social
```

Then reference the returned channel ID in the alert policies above.

### Log-based metrics for auth failures

Track brute-force attempts and account lockouts with a log-based metric:

```bash
# Create a metric that counts login lockout events emitted by the auth service
gcloud logging metrics create auth_login_locked \
  --description="Login attempts blocked by rate limiter" \
  --log-filter='resource.type="cloud_run_revision" AND resource.labels.service_name="auth" AND textPayload:"too many failed attempts"'

# Then create an alert: if this metric > 50 in 10 minutes, fire
```

### Dashboard (recommended)

Create a custom dashboard in **Monitoring → Dashboards → Create dashboard** with:

| Widget | Metric |
|---|---|
| Uptime scorecard | `monitoring.googleapis.com/uptime_check/check_passed` |
| Gateway request rate | `run.googleapis.com/request_count` (service=gateway) |
| Gateway latency (p99) | `run.googleapis.com/request_latencies` |
| Auth 5xx count | `run.googleapis.com/request_count` (service=auth, 5xx) |
| Auth login lockouts | `logging.googleapis.com/user/auth_login_locked` |
| Cloud SQL connections | `cloudsql.googleapis.com/database/postgresql/num_backends` |

### Monitoring checklist

- [ ] Uptime check created for Gateway `/healthz`
- [ ] Uptime check created for web app root `/`
- [ ] Alert policy created for service downtime (2-region failure)
- [ ] Alert policy created for gateway 5xx rate > 1%
- [ ] Email (or PagerDuty) notification channel configured
- [ ] Log-based metric created for auth lockout events
- [ ] Custom dashboard created and bookmarked
