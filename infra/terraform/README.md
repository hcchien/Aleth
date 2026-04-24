# Aleth Terraform

This Terraform config provisions the first split-service GCP deployment for Aleth:

- Cloud Run service for `leith-api`
- Cloud Run service for `leith-web`
- Cloud SQL for PostgreSQL
- Secret Manager `LEITH_DB_DSN`
- service accounts and minimal IAM
- Artifact Registry repository
- optional global HTTPS load balancer routing `/api/*` to the API and everything else to the web app

## Why FE and BE are split

Cloud Run can run multiple containers in one service, but only one container is the public ingress container and the rest are sidecars. Splitting FE and BE keeps independent deploys, rollback, scaling, permissions, and resource sizing while still allowing a same-domain setup through the HTTPS load balancer.

## Prerequisites

1. Install Terraform `>= 1.6`.
2. Authenticate Google Cloud locally:

```bash
gcloud auth application-default login
gcloud auth login
```

3. Build and push container images for:

```text
packages/leith-api
packages/leith-web
```

Terraform expects image URIs through `api_image` and `web_image`.

Important for the web image:

- `NEXT_PUBLIC_API_URL` is normally compiled into the Next.js client bundle at build time.
- If you deploy with the load balancer enabled, build the web image with `NEXT_PUBLIC_API_URL=/api`.
- If you deploy without the load balancer, build the web image with the concrete API Cloud Run URL or use a runtime config pattern in the app.
- Terraform still sets `NEXT_PUBLIC_API_URL` on the Cloud Run service, but that alone may not change already-built browser JavaScript.

## First Run

```bash
cd infra/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`, then run:

```bash
terraform init
terraform plan
terraform apply
```

## Load Balancer

By default, `enable_load_balancer = false`.

When you are ready to use a custom domain:

```hcl
enable_load_balancer = true
domain_name          = "aleth.example.com"
```

After apply, point your DNS `A` record to the `load_balancer_ip` output. Google-managed SSL certificates can take time to become active.

When using the load balancer, the frontend should call the API through:

```text
/api
```

The URL map routes `/api` and `/api/*` to the API service.

## Database Connection

The API connects to Cloud SQL through the Cloud Run Cloud SQL mount:

```text
/cloudsql/<PROJECT>:<REGION>:<INSTANCE>
```

Terraform stores this DSN in Secret Manager and injects it into the API as `LEITH_DB_DSN`.

The API env is configured as:

```text
LEITH_STORE_BACKEND=sql
LEITH_DB_DRIVER=pgx
LEITH_DB_DSN=<secret>
```

## Important Production Caveat

The API currently hardcodes WebAuthn RP settings to localhost. Terraform already passes:

```text
WEBAUTHN_RP_ID
WEBAUTHN_RP_ORIGINS
```

but the API must still be updated to read those env vars before production passkeys work on your custom domain.

## Recommended Next Steps

1. Add Dockerfiles or Cloud Build configs for reproducible image builds.
2. Update the API to read WebAuthn RP config from env.
3. Add `/healthz` endpoints for API and web.
4. Add Cloud Tasks for AI transformation and discussion summary jobs.
5. Move Terraform state to a remote GCS backend before multiple people operate it.
