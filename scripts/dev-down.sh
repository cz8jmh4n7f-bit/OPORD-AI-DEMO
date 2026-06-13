#!/usr/bin/env bash
#
# dev-down.sh - stop the OPORD dev app processes (api, worker, web).
#
#   scripts/dev-down.sh         stop opord-api + opord-worker + web
#   scripts/dev-down.sh --all   also stop docker infra (postgres + openbao)
#
# Leaves Docker infra running by default so a follow-up dev-up.sh is fast.
# Refuses to stop the worker while a `tofu` apply/destroy is in flight (that
# would orphan cloud resources) unless you confirm.
#
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
DEV_DIR="$ROOT/.dev"

log()  { printf '\n\033[1;34m▸ %s\033[0m\n' "$*"; }
ok()   { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[1;33m!\033[0m %s\n' "$*"; }

# Safety: a running tofu means a provision/destroy is mid-flight.
if pgrep -f '[t]ofu' >/dev/null 2>&1; then
  warn "tofu is running - a provision/destroy is in flight."
  warn "Stopping the worker now can orphan cloud resources."
  read -r -p "  Continue anyway? [y/N] " ans
  [ "${ans:-}" = "y" ] || [ "${ans:-}" = "Y" ] || { echo "  aborted."; exit 1; }
fi

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

log "Stopping app processes"
stop_pidfile web        "$DEV_DIR/web.pid"
stop_pidfile opord-api  "$DEV_DIR/api.pid"
stop_pidfile opord-worker "$DEV_DIR/worker.pid"

# Fallback: catch processes not started via dev-up (e.g. `go run`).
pkill -f 'bin/opord-api|exe/api|go run ./cmd/api'        2>/dev/null && ok "opord-api (fallback)"   || true
pkill -f 'bin/opord-worker|exe/worker|go run ./cmd/worker' 2>/dev/null && ok "opord-worker (fallback)" || true
pkill -f 'next dev|next-server'                          2>/dev/null && ok "web (fallback)"        || true

if [ "${1:-}" = "--all" ]; then
  log "Stopping docker infra (postgres + openbao)"
  docker compose -f deployments/docker-compose.yml stop postgres openbao >/dev/null 2>&1 \
    && ok "postgres + openbao stopped" || warn "docker stop failed"
fi

ok "Down."
