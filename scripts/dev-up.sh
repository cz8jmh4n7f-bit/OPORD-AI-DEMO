#!/usr/bin/env bash
#
# dev-up.sh - bring up the full OPORD dev stack with one command.
#
#   postgres + openbao (docker)  ->  unseal  ->  migrations  ->  build
#   ->  opord-api  ->  opord-worker  ->  web (next dev)
#
# Idempotent for infra/web. App binaries are rebuilt and restarted so newly
# added API routes/jobs do not keep serving from a stale process. Runtime
# logs/PIDs go to .dev/ (self-ignored).
#
# Env: reads .env if present, else falls back to the repo dev defaults.
# VAULT_TOKEN is derived from deployments/openbao/.init.json when not set, so
# provider creds resolve from OpenBao instead of falling back to env.
#
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
DEV_DIR="$ROOT/.dev"
mkdir -p "$DEV_DIR"
printf '*\n' > "$DEV_DIR/.gitignore"   # keep runtime files out of git

COMPOSE=(docker compose -f deployments/docker-compose.yml)

log()  { printf '\n\033[1;34m▸ %s\033[0m\n' "$*"; }
ok()   { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[1;33m!\033[0m %s\n' "$*"; }
die()  { printf '  \033[1;31m✗ %s\033[0m\n' "$*"; exit 1; }
port_up() { nc -z -w1 localhost "$1" >/dev/null 2>&1; }
stop_pidfile() {  # name, pidfile
  local name="$1" f="$2" pid
  [ -f "$f" ] || return 1
  pid="$(cat "$f" 2>/dev/null)"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null; sleep 1; kill -9 "$pid" 2>/dev/null
    ok "$name stopped (pid $pid)"
  fi
  rm -f "$f"
  return 0
}

# ── env ────────────────────────────────────────────────────────────────────
[ -f .env ] && { set -a; . ./.env; set +a; }
export DATABASE_URL="${DATABASE_URL:-postgres://opord:opord@localhost:5432/opord?sslmode=disable}"
export VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
if [ -z "${VAULT_TOKEN:-}" ] && [ -f deployments/openbao/.init.json ]; then
  VAULT_TOKEN="$(python3 -c "import json;print(json.load(open('deployments/openbao/.init.json'))['root_token'])" 2>/dev/null)" && export VAULT_TOKEN
fi

# ── 1. docker infra ────────────────────────────────────────────────────────
log "Docker infra (postgres + openbao)"
"${COMPOSE[@]}" up -d postgres openbao >/dev/null 2>&1 || die "docker compose up failed (is Docker running?)"
for _ in $(seq 1 30); do
  "${COMPOSE[@]}" exec -T postgres pg_isready -U opord >/dev/null 2>&1 && break
  sleep 1
done
"${COMPOSE[@]}" exec -T postgres pg_isready -U opord >/dev/null 2>&1 && ok "postgres ready" || die "postgres not ready"

# ── 2. openbao unseal ──────────────────────────────────────────────────────
log "OpenBao"
for _ in $(seq 1 20); do port_up 8200 && break; sleep 1; done
SEALED="$(curl -s --max-time 5 http://localhost:8200/v1/sys/seal-status | python3 -c 'import sys,json;print(json.load(sys.stdin)["sealed"])' 2>/dev/null || echo unknown)"
if [ "$SEALED" = "True" ]; then
  if [ -f deployments/openbao/.init.json ]; then
    KEY="$(python3 -c "import json;print(json.load(open('deployments/openbao/.init.json'))['unseal_keys_b64'][0])")"
    curl -s --max-time 5 -X PUT http://localhost:8200/v1/sys/unseal -d "{\"key\":\"$KEY\"}" >/dev/null
    NOW="$(curl -s http://localhost:8200/v1/sys/seal-status | python3 -c 'import sys,json;print(json.load(sys.stdin)["sealed"])' 2>/dev/null)"
    [ "$NOW" = "False" ] && ok "unsealed" || warn "unseal did not take - creds fall back to env"
  else
    warn "sealed and no .init.json - provider creds fall back to env"
  fi
elif [ "$SEALED" = "False" ]; then
  ok "already unsealed"
else
  warn "could not read seal status (continuing; creds may fall back to env)"
fi

# ── 3. migrations ──────────────────────────────────────────────────────────
log "Migrations (goose)"
if command -v goose >/dev/null 2>&1; then
  goose -dir migrations postgres "$DATABASE_URL" up >/dev/null 2>&1 && ok "schema up to date" || warn "goose up failed (check DB / migrations)"
else
  warn "goose not on PATH - skipping (install: go install github.com/pressly/goose/v3/cmd/goose@latest)"
fi

# ── 4. build ───────────────────────────────────────────────────────────────
log "Build binaries"
go build -o bin/opord-api ./cmd/api       || die "go build ./cmd/api failed"
go build -o bin/opord-worker ./cmd/worker || die "go build ./cmd/worker failed"
ok "opord-api + opord-worker built"

start_bg() {  # name, pidfile, logfile, cmd...
  local name="$1" pidf="$2" logf="$3"; shift 3
  nohup "$@" > "$logf" 2>&1 &
  echo $! > "$pidf"
}

# ── 5. opord-api ───────────────────────────────────────────────────────────
log "opord-api :8080"
stop_pidfile opord-api "$DEV_DIR/api.pid" >/dev/null 2>&1 || true
pkill -f 'bin/opord-api|exe/api|go run ./cmd/api' 2>/dev/null || true
start_bg opord-api "$DEV_DIR/api.pid" "$DEV_DIR/api.log" ./bin/opord-api
for _ in $(seq 1 30); do port_up 8080 && break; sleep 1; done
port_up 8080 && ok "listening (pid $(cat "$DEV_DIR/api.pid"))" || warn "did not come up - tail .dev/api.log"

# ── 6. opord-worker ────────────────────────────────────────────────────────
log "opord-worker"
stop_pidfile opord-worker "$DEV_DIR/worker.pid" >/dev/null 2>&1 || true
pkill -f 'bin/opord-worker|exe/worker|go run ./cmd/worker' 2>/dev/null || true
start_bg opord-worker "$DEV_DIR/worker.pid" "$DEV_DIR/worker.log" ./bin/opord-worker
sleep 2
kill -0 "$(cat "$DEV_DIR/worker.pid")" 2>/dev/null && ok "running (pid $(cat "$DEV_DIR/worker.pid"))" || warn "exited - tail .dev/worker.log"

# ── 7. web ─────────────────────────────────────────────────────────────────
log "web :3000"
if port_up 3000; then
  warn "already running (skip)"
else
  ( cd web && nohup npm run dev > "$DEV_DIR/web.log" 2>&1 & echo $! > "$DEV_DIR/web.pid" )
  for _ in $(seq 1 45); do port_up 3000 && break; sleep 1; done
  port_up 3000 && ok "listening" || warn "did not come up - tail .dev/web.log"
fi

# ── status ─────────────────────────────────────────────────────────────────
HEALTH="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 http://localhost:8080/healthz 2>/dev/null || echo 000)"
log "Stack status"
printf '  %-17s %s\n' "postgres :5432"  "$(port_up 5432 && echo up || echo DOWN)"
printf '  %-17s %s\n' "openbao :8200"   "$(port_up 8200 && echo up || echo DOWN)"
printf '  %-17s %s\n' "opord-api :8080" "$([ "$HEALTH" = 200 ] && echo "healthy (200)" || echo "DOWN ($HEALTH)")"
printf '  %-17s %s\n' "opord-worker"    "$(pgrep -f 'opord-worker' >/dev/null && echo running || echo DOWN)"
printf '  %-17s %s\n' "web :3000"       "$(port_up 3000 && echo up || echo DOWN)"
echo
ok "Up. Logs: .dev/*.log  ·  Stop: scripts/dev-down.sh"
