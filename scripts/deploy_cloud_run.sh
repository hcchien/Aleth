#!/usr/bin/env bash

set -euo pipefail

# Deploy Aleth frontend and backend to Cloud Run using source-based builds.
#
# Usage:
#   cp production.env.example production.env
#   # edit production.env
#   ENV_FILE=production.env ./scripts/deploy_cloud_run.sh
#
# Notes:
# - This script deploys the API first, then uses the resolved API URL for the web app
#   if NEXT_PUBLIC_API_URL was not provided explicitly.
# - It assumes db_init.sh and db_migrate.sh have already prepared Cloud SQL.
# - The current API code hardcodes WebAuthn RP settings to localhost, so passkeys will
#   not work in production until those values are externalized in the app code.

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

append_csv() {
  local current="$1"
  local next="$2"
  if [[ -z "${next}" ]]; then
    printf '%s' "${current}"
  elif [[ -z "${current}" ]]; then
    printf '%s' "${next}"
  else
    printf '%s,%s' "${current}" "${next}"
  fi
}

require_cmd gcloud

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ENV_FILE:-${REPO_ROOT}/production.env}"

if [[ -f "${ENV_FILE}" ]]; then
  echo "==> Loading env from ${ENV_FILE}"
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
fi

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-asia-east1}"
API_SERVICE_NAME="${API_SERVICE_NAME:-leith-api}"
WEB_SERVICE_NAME="${WEB_SERVICE_NAME:-leith-web}"
API_SOURCE_DIR="${API_SOURCE_DIR:-${REPO_ROOT}/packages/leith-api}"
WEB_SOURCE_DIR="${WEB_SOURCE_DIR:-${REPO_ROOT}/packages/leith-web}"
API_PORT="${API_PORT:-8080}"
WEB_PORT="${WEB_PORT:-3000}"
API_MEMORY="${API_MEMORY:-1Gi}"
WEB_MEMORY="${WEB_MEMORY:-1Gi}"
API_CPU="${API_CPU:-1}"
WEB_CPU="${WEB_CPU:-1}"
API_MIN_INSTANCES="${API_MIN_INSTANCES:-0}"
WEB_MIN_INSTANCES="${WEB_MIN_INSTANCES:-0}"
API_MAX_INSTANCES="${API_MAX_INSTANCES:-10}"
WEB_MAX_INSTANCES="${WEB_MAX_INSTANCES:-10}"
API_ALLOW_UNAUTHENTICATED="${API_ALLOW_UNAUTHENTICATED:-true}"
WEB_ALLOW_UNAUTHENTICATED="${WEB_ALLOW_UNAUTHENTICATED:-true}"
API_SERVICE_ACCOUNT="${API_SERVICE_ACCOUNT:-}"
WEB_SERVICE_ACCOUNT="${WEB_SERVICE_ACCOUNT:-}"
API_DB_DSN_SECRET="${API_DB_DSN_SECRET:-aleth-db-dsn}"
NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL:-}"
API_CUSTOM_ENV_VARS="${API_CUSTOM_ENV_VARS:-}"
WEB_CUSTOM_ENV_VARS="${WEB_CUSTOM_ENV_VARS:-}"
API_CUSTOM_SECRETS="${API_CUSTOM_SECRETS:-}"
WEB_CUSTOM_SECRETS="${WEB_CUSTOM_SECRETS:-}"

require_env PROJECT_ID

echo "==> Setting gcloud project to ${PROJECT_ID}"
gcloud config set project "${PROJECT_ID}" >/dev/null

deploy_api_args=(
  run deploy "${API_SERVICE_NAME}"
  --project="${PROJECT_ID}"
  --region="${REGION}"
  --source="${API_SOURCE_DIR}"
  --port="${API_PORT}"
  --cpu="${API_CPU}"
  --memory="${API_MEMORY}"
  --min-instances="${API_MIN_INSTANCES}"
  --max-instances="${API_MAX_INSTANCES}"
  --set-env-vars="LEITH_STORE_BACKEND=sql,LEITH_DB_DRIVER=pgx"
  --update-secrets="LEITH_DB_DSN=${API_DB_DSN_SECRET}:latest"
)

if [[ "${API_ALLOW_UNAUTHENTICATED}" == "true" ]]; then
  deploy_api_args+=(--allow-unauthenticated)
fi

if [[ -n "${API_SERVICE_ACCOUNT}" ]]; then
  deploy_api_args+=(--service-account="${API_SERVICE_ACCOUNT}")
fi

if [[ -n "${API_CUSTOM_ENV_VARS}" ]]; then
  deploy_api_args+=(--update-env-vars="${API_CUSTOM_ENV_VARS}")
fi

if [[ -n "${API_CUSTOM_SECRETS}" ]]; then
  deploy_api_args+=(--update-secrets="${API_CUSTOM_SECRETS}")
fi

echo "==> Deploying API service ${API_SERVICE_NAME}"
gcloud "${deploy_api_args[@]}"

API_URL="$(gcloud run services describe "${API_SERVICE_NAME}" --region="${REGION}" --format='value(status.url)')"

if [[ -z "${NEXT_PUBLIC_API_URL}" ]]; then
  NEXT_PUBLIC_API_URL="${API_URL}"
  echo "==> NEXT_PUBLIC_API_URL not set, defaulting to deployed API URL: ${NEXT_PUBLIC_API_URL}"
fi

web_env_vars="NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}"
web_env_vars="$(append_csv "${web_env_vars}" "${WEB_CUSTOM_ENV_VARS}")"

deploy_web_args=(
  run deploy "${WEB_SERVICE_NAME}"
  --project="${PROJECT_ID}"
  --region="${REGION}"
  --source="${WEB_SOURCE_DIR}"
  --port="${WEB_PORT}"
  --cpu="${WEB_CPU}"
  --memory="${WEB_MEMORY}"
  --min-instances="${WEB_MIN_INSTANCES}"
  --max-instances="${WEB_MAX_INSTANCES}"
  --set-env-vars="${web_env_vars}"
)

if [[ "${WEB_ALLOW_UNAUTHENTICATED}" == "true" ]]; then
  deploy_web_args+=(--allow-unauthenticated)
fi

if [[ -n "${WEB_SERVICE_ACCOUNT}" ]]; then
  deploy_web_args+=(--service-account="${WEB_SERVICE_ACCOUNT}")
fi

if [[ -n "${WEB_CUSTOM_SECRETS}" ]]; then
  deploy_web_args+=(--update-secrets="${WEB_CUSTOM_SECRETS}")
fi

echo "==> Deploying web service ${WEB_SERVICE_NAME}"
gcloud "${deploy_web_args[@]}"

WEB_URL="$(gcloud run services describe "${WEB_SERVICE_NAME}" --region="${REGION}" --format='value(status.url)')"

cat <<EOF

Deploy complete.

API service:
  ${API_SERVICE_NAME}
  ${API_URL}

Web service:
  ${WEB_SERVICE_NAME}
  ${WEB_URL}

Important:
  Passkey / WebAuthn production settings are not yet externalized in the API code.
  Before enabling passkeys on a custom domain, update the API to read RP ID and RP origins from env.
EOF
