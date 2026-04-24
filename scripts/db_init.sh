#!/usr/bin/env bash

set -euo pipefail

# Bootstrap a Cloud SQL for PostgreSQL environment for Aleth.
#
# What this script does:
# 1. Enables the required GCP APIs.
# 2. Creates a Cloud SQL PostgreSQL instance if it does not exist.
# 3. Creates the application database and user if they do not exist.
# 4. Stores the DSN in Secret Manager for later Cloud Run use.
#
# What this script does NOT do:
# - It does not run schema migrations.
# - It does not deploy Cloud Run services.
#
# Recommended usage:
#   PROJECT_ID=my-gcp-project \
#   REGION=asia-east1 \
#   INSTANCE_NAME=aleth-pg-prod \
#   DB_NAME=aleth \
#   DB_USER=aleth_app \
#   ./scripts/db_init.sh

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "error: missing required env var: $name" >&2
    exit 1
  fi
}

random_password() {
  openssl rand -base64 24 | tr -d '\n'
}

require_cmd gcloud
require_cmd openssl

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-asia-east1}"
INSTANCE_NAME="${INSTANCE_NAME:-aleth-pg}"
DB_VERSION="${DB_VERSION:-POSTGRES_16}"
TIER="${TIER:-db-custom-1-3840}"
AVAILABILITY_TYPE="${AVAILABILITY_TYPE:-ZONAL}"
STORAGE_SIZE_GB="${STORAGE_SIZE_GB:-20}"
DB_NAME="${DB_NAME:-aleth}"
DB_USER="${DB_USER:-aleth_app}"
DB_PASSWORD="${DB_PASSWORD:-$(random_password)}"
SECRET_NAME="${SECRET_NAME:-aleth-db-dsn}"
LEITH_DB_DRIVER="${LEITH_DB_DRIVER:-pgx}"
LABELS="${LABELS:-app=aleth,component=database}"

require_env PROJECT_ID

echo "==> Setting gcloud project to ${PROJECT_ID}"
gcloud config set project "${PROJECT_ID}" >/dev/null

echo "==> Enabling required APIs"
gcloud services enable \
  sqladmin.googleapis.com \
  secretmanager.googleapis.com \
  run.googleapis.com \
  artifactregistry.googleapis.com

if ! gcloud sql instances describe "${INSTANCE_NAME}" >/dev/null 2>&1; then
  echo "==> Creating Cloud SQL instance ${INSTANCE_NAME}"
  gcloud sql instances create "${INSTANCE_NAME}" \
    --project="${PROJECT_ID}" \
    --database-version="${DB_VERSION}" \
    --region="${REGION}" \
    --tier="${TIER}" \
    --availability-type="${AVAILABILITY_TYPE}" \
    --storage-size="${STORAGE_SIZE_GB}" \
    --storage-auto-increase \
    --backup-start-time=03:00 \
    --deletion-protection \
    --labels="${LABELS}"
else
  echo "==> Cloud SQL instance ${INSTANCE_NAME} already exists, skipping create"
fi

if ! gcloud sql databases describe "${DB_NAME}" --instance="${INSTANCE_NAME}" >/dev/null 2>&1; then
  echo "==> Creating database ${DB_NAME}"
  gcloud sql databases create "${DB_NAME}" --instance="${INSTANCE_NAME}"
else
  echo "==> Database ${DB_NAME} already exists, skipping create"
fi

if ! gcloud sql users describe "${DB_USER}" --instance="${INSTANCE_NAME}" >/dev/null 2>&1; then
  echo "==> Creating database user ${DB_USER}"
  gcloud sql users create "${DB_USER}" \
    --instance="${INSTANCE_NAME}" \
    --password="${DB_PASSWORD}"
else
  echo "==> Database user ${DB_USER} already exists, resetting password"
  gcloud sql users set-password "${DB_USER}" \
    --instance="${INSTANCE_NAME}" \
    --password="${DB_PASSWORD}"
fi

PUBLIC_IP="$(gcloud sql instances describe "${INSTANCE_NAME}" --format='value(ipAddresses[0].ipAddress)')"

if [[ -z "${PUBLIC_IP}" ]]; then
  echo "error: failed to discover Cloud SQL public IP for ${INSTANCE_NAME}" >&2
  exit 1
fi

LEITH_DB_DSN="postgres://${DB_USER}:${DB_PASSWORD}@${PUBLIC_IP}:5432/${DB_NAME}?sslmode=require"

echo "==> Writing DSN to Secret Manager: ${SECRET_NAME}"
if gcloud secrets describe "${SECRET_NAME}" >/dev/null 2>&1; then
  printf '%s' "${LEITH_DB_DSN}" | gcloud secrets versions add "${SECRET_NAME}" --data-file=-
else
  printf '%s' "${LEITH_DB_DSN}" | gcloud secrets create "${SECRET_NAME}" --data-file=-
fi

cat <<EOF

Bootstrap complete.

Cloud SQL instance:
  ${INSTANCE_NAME}

Database:
  ${DB_NAME}

Database user:
  ${DB_USER}

Secret Manager:
  ${SECRET_NAME}

Suggested Cloud Run env:
  LEITH_STORE_BACKEND=sql
  LEITH_DB_DRIVER=${LEITH_DB_DRIVER}
  LEITH_DB_DSN=<from Secret Manager ${SECRET_NAME}>

Next step:
  Run ./scripts/db_migrate.sh against this database before deploying traffic.
EOF
