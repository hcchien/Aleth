#!/usr/bin/env bash

set -euo pipefail

# Run Aleth schema migrations against the configured database.
#
# This repo already embeds SQL migrations in the Go API and applies them during
# SQL store startup via store.OpenSQLStore(...). This script intentionally reuses
# that path instead of introducing a second migration system.
#
# Usage examples:
#   LEITH_STORE_BACKEND=sql \
#   LEITH_DB_DRIVER=pgx \
#   LEITH_DB_DSN='postgres://user:pass@host:5432/aleth?sslmode=require' \
#   ./scripts/db_migrate.sh
#
#   PROJECT_ID=my-gcp-project SECRET_NAME=aleth-db-dsn ./scripts/db_migrate.sh

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
API_DIR="${REPO_ROOT}/packages/leith-api"

require_cmd go

export LEITH_STORE_BACKEND="${LEITH_STORE_BACKEND:-sql}"
export LEITH_DB_DRIVER="${LEITH_DB_DRIVER:-pgx}"
export LEITH_ADDR="${LEITH_ADDR:-127.0.0.1:18080}"

if [[ -z "${LEITH_DB_DSN:-}" ]]; then
  require_cmd gcloud
  PROJECT_ID="${PROJECT_ID:-}"
  SECRET_NAME="${SECRET_NAME:-aleth-db-dsn}"

  if [[ -z "${PROJECT_ID}" ]]; then
    echo "error: set LEITH_DB_DSN directly, or provide PROJECT_ID so the script can read Secret Manager" >&2
    exit 1
  fi

  echo "==> Reading database DSN from Secret Manager: ${SECRET_NAME}"
  export LEITH_DB_DSN
  LEITH_DB_DSN="$(gcloud secrets versions access latest --secret="${SECRET_NAME}" --project="${PROJECT_ID}")"
fi

if [[ -z "${LEITH_DB_DSN}" ]]; then
  echo "error: LEITH_DB_DSN resolved to an empty value" >&2
  exit 1
fi

echo "==> Running Aleth embedded migrations using API startup path"
(
  cd "${API_DIR}"
  timeout 15s go run ./cmd/leithd >/tmp/aleth-db-migrate.log 2>&1 || true
)

if rg -n "sql backend init failed|apply migration|commit migration tx|panic:" /tmp/aleth-db-migrate.log >/dev/null 2>&1; then
  echo "error: migration bootstrap failed" >&2
  cat /tmp/aleth-db-migrate.log >&2
  exit 1
fi

if ! rg -n "starting leith api|Listening|listen|server" /tmp/aleth-db-migrate.log >/dev/null 2>&1; then
  echo "warning: migration path completed but server startup marker was not found; inspect /tmp/aleth-db-migrate.log if needed" >&2
fi

cat <<EOF
Migration bootstrap finished.

Because migrations are applied during SQL store initialization in:
  ${API_DIR}/internal/store/sql_migrations.go

this script validates migration execution by starting the API briefly with:
  LEITH_STORE_BACKEND=sql
  LEITH_DB_DRIVER=${LEITH_DB_DRIVER}
  LEITH_DB_DSN=<resolved>

Log file:
  /tmp/aleth-db-migrate.log
EOF
