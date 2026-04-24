# Aleth GCP Deployment README

This document describes a practical first production deployment path for Aleth on GCP using:

- Cloud Run for `packages/leith-web`
- Cloud Run for `packages/leith-api`
- Cloud SQL for PostgreSQL
- Secret Manager for runtime secrets
- Optional HTTPS Load Balancer in front of both services

## Recommended Architecture

For the current repo, the simplest production shape is:

1. `leith-api` on Cloud Run
2. `leith-web` on Cloud Run
3. Cloud SQL PostgreSQL for persistent public data
4. Secret Manager for `LEITH_DB_DSN` and other secrets
5. Optional Global HTTPS Load Balancer for custom domain routing

Recommended public URL shape:

- `https://aleth.example.com/` -> web
- `https://aleth.example.com/api/*` -> api

This is cleaner than splitting `app.example.com` and `api.example.com`, especially once passkeys and auth cookies matter.

## Current Production Caveat

The API currently hardcodes WebAuthn relying party settings to localhost in:

- [packages/leith-api/cmd/leithd/main.go](/Users/hcchien/code/Aleth/packages/leith-api/cmd/leithd/main.go)

That means:

- local passkey login works
- production passkey login will not work correctly on a real domain yet

Before enabling passkeys in production, externalize:

- `RPID`
- `RPOrigins`
- optionally `RPDisplayName`

into environment variables or Secret Manager backed configuration.

## Files Added

- [scripts/db_init.sh](/Users/hcchien/code/Aleth/scripts/db_init.sh)
- [scripts/db_migrate.sh](/Users/hcchien/code/Aleth/scripts/db_migrate.sh)
- [scripts/deploy_cloud_run.sh](/Users/hcchien/code/Aleth/scripts/deploy_cloud_run.sh)
- [production.env.example](/Users/hcchien/code/Aleth/production.env.example)

## Step 1: Bootstrap Cloud SQL

Example:

```bash
PROJECT_ID=my-gcp-project \
REGION=asia-east1 \
INSTANCE_NAME=aleth-pg-prod \
DB_NAME=aleth \
DB_USER=aleth_app \
./scripts/db_init.sh
```

This script:

- enables required APIs
- creates the Cloud SQL PostgreSQL instance
- creates the database and DB user
- stores the DSN in Secret Manager

By default it writes the DSN to:

- `aleth-db-dsn`

## Step 2: Run Schema Migrations

Example:

```bash
PROJECT_ID=my-gcp-project \
SECRET_NAME=aleth-db-dsn \
./scripts/db_migrate.sh
```

This repo already embeds migrations in Go:

- [packages/leith-api/internal/store/sql_migrations.go](/Users/hcchien/code/Aleth/packages/leith-api/internal/store/sql_migrations.go)

So the migration script intentionally reuses API startup instead of introducing a second migration framework.

## Step 3: Prepare Runtime Configuration

Copy the example file:

```bash
cp production.env.example production.env
```

Then edit:

- `PROJECT_ID`
- `REGION`
- service names
- service accounts
- `NEXT_PUBLIC_API_URL` if you want to pin it
- any optional extra secrets/env vars
- `API_CUSTOM_ENV_VARS=LEITH_BOOTSTRAP_VERIFIER_DIDS=did:vflow:...` to seed the first L4 verifier

## Step 4: Deploy Cloud Run Services

Example:

```bash
ENV_FILE=production.env ./scripts/deploy_cloud_run.sh
```

The deploy script:

1. deploys `leith-api`
2. discovers the deployed API URL
3. deploys `leith-web`
4. injects `NEXT_PUBLIC_API_URL` for the frontend if you did not specify one

## Step 5: Add a Custom Domain

For a real production setup, place both services behind a global HTTPS load balancer.

Suggested routing:

- `/` -> `leith-web`
- `/api/*` -> `leith-api`

This part is not automated by the current script. It is better handled with Terraform or a dedicated infra runbook.

## Suggested Service Accounts

Create separate service accounts for least privilege:

- `leith-api-prod@...`
- `leith-web-prod@...`

Suggested API permissions:

- `roles/secretmanager.secretAccessor`
- `roles/cloudsql.client`
- any future storage / tasks roles you need

The frontend service often needs far fewer permissions.

## Suggested Next Hardening Steps

After the first production deployment, I would recommend:

1. Move passkey RP config to env vars
2. Add Cloud Tasks for AI transformation jobs
3. Add Cloud Armor in front of the load balancer
4. Add Cloud Monitoring uptime checks and alerting
5. Add staging and production as separate GCP projects
6. Move deployment from shell scripts to Terraform + CI/CD

## Minimal Deploy Checklist

- Cloud SQL instance exists
- Secret Manager contains `LEITH_DB_DSN`
- migrations have run successfully
- API deploy succeeds
- frontend deploy succeeds
- CORS / domain routing is verified
- passkey production config is fixed before public rollout

## Recommended Next Repo Changes

To make GCP deployment cleaner, the next useful code changes would be:

1. read passkey RP settings from env
2. add explicit health endpoints for Cloud Run and load balancer checks
3. add Dockerfiles or a Cloud Build config for reproducible builds
4. add a dedicated migration entrypoint instead of relying on full API startup
