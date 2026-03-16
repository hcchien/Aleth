#!/usr/bin/env bash
# dev.sh — starts all Aleth services locally
# Usage: ./dev.sh [stop]
set -e

REPO="$(cd "$(dirname "$0")" && pwd)"
PIDS_FILE="$REPO/.dev-pids"

# ── helpers ────────────────────────────────────────────────────────────────────

log() { echo "▶  $*"; }

load_env() {
  local file="$1"
  if [ -f "$file" ]; then
    # export KEY=VALUE lines, ignoring comments and blanks
    set -a
    # shellcheck disable=SC1090
    source "$file"
    set +a
  fi
}

wait_port() {
  local host="$1" port="$2" label="$3"
  for _ in $(seq 1 30); do
    if nc -z "$host" "$port" 2>/dev/null; then
      log "$label ready on :$port"
      return 0
    fi
    sleep 0.5
  done
  echo "✗  $label failed to start on :$port" >&2
  return 1
}

# ── stop ───────────────────────────────────────────────────────────────────────

stop_all() {
  if [ -f "$PIDS_FILE" ]; then
    log "Stopping services…"
    while IFS= read -r pid; do
      kill "$pid" 2>/dev/null || true
    done < "$PIDS_FILE"
    rm -f "$PIDS_FILE"
  fi
  log "Done."
}

if [ "${1:-}" = "stop" ]; then
  stop_all
  exit 0
fi

# ── postgres ───────────────────────────────────────────────────────────────────

if ! pg_isready -q 2>/dev/null; then
  log "Starting PostgreSQL 14…"
  pg_ctl -D /usr/local/var/postgresql@14 start -l /tmp/postgresql14.log
  wait_port localhost 5432 "PostgreSQL"
else
  log "PostgreSQL already running."
fi

# ── migrations ─────────────────────────────────────────────────────────────────

GOOSE="$HOME/go/bin/goose"
if ! command -v "$GOOSE" &>/dev/null; then
  log "goose not found — skipping migrations (run: CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.19.2)"
else
  load_env "$REPO/services/auth/.env"
  log "Migrating auth DB…"
  "$GOOSE" -dir "$REPO/migrations/auth" postgres "$AUTH_DATABASE_URL" up

  load_env "$REPO/services/content/.env"
  log "Migrating content DB…"
  "$GOOSE" -dir "$REPO/migrations/content" postgres "$CONTENT_DATABASE_URL" up

  load_env "$REPO/services/notification/.env"
  log "Migrating notification DB…"
  "$GOOSE" -table goose_notification_versions -dir "$REPO/migrations/notification" postgres "$NOTIFICATION_DATABASE_URL" up
fi

# ── build services ─────────────────────────────────────────────────────────────

log "Building services…"
(
  cd "$REPO"
  CGO_ENABLED=0 go build -o /tmp/aleth-auth     ./services/auth/cmd/auth
  CGO_ENABLED=0 go build -o /tmp/aleth-content  ./services/content/cmd/content
  CGO_ENABLED=0 go build -o /tmp/aleth-gateway      ./services/gateway/cmd/gateway
  CGO_ENABLED=0 go build -o /tmp/aleth-feed         ./services/feed/cmd/feed
  CGO_ENABLED=0 go build -o /tmp/aleth-notification ./services/notification/cmd/notification
)

# ── launch services ────────────────────────────────────────────────────────────

> "$PIDS_FILE"   # reset pids file

log "Starting auth service on :8081…"
load_env "$REPO/services/auth/.env"
/tmp/aleth-auth 2>&1 | sed 's/^/[auth]    /' &
echo $! >> "$PIDS_FILE"
wait_port localhost 8081 "auth service"

log "Starting content service on :8082…"
load_env "$REPO/services/content/.env"
/tmp/aleth-content 2>&1 | sed 's/^/[content] /' &
echo $! >> "$PIDS_FILE"
wait_port localhost 8082 "content service"

log "Starting feed service on :8083…"
load_env "$REPO/services/feed/.env"
/tmp/aleth-feed 2>&1 | sed 's/^/[feed]    /' &
echo $! >> "$PIDS_FILE"
wait_port localhost 8083 "feed service"

log "Starting notification service on :8084…"
load_env "$REPO/services/notification/.env"
/tmp/aleth-notification 2>&1 | sed 's/^/[notif]   /' &
echo $! >> "$PIDS_FILE"
wait_port localhost 8084 "notification service"

log "Starting gateway on :4000…"
load_env "$REPO/services/gateway/.env"
/tmp/aleth-gateway 2>&1 | sed 's/^/[gateway] /' &
echo $! >> "$PIDS_FILE"
wait_port localhost 4000 "gateway"

# ── done ───────────────────────────────────────────────────────────────────────

cat <<EOF

✅  All services running:
   Auth:         http://localhost:8081/healthz
   Content:      http://localhost:8082/healthz
   Feed:         http://localhost:8083/healthz
   Notification: http://localhost:8084/healthz
   Gateway:      http://localhost:4000/graphql

Start the web app in another terminal:
   source ~/.nvm/nvm.sh && nvm use 22
   pnpm --filter web dev          # → http://localhost:3000

To stop all services:
   ./dev.sh stop

PIDs written to $PIDS_FILE
EOF

# Keep script alive so Ctrl-C kills everything
trap stop_all INT TERM
wait
