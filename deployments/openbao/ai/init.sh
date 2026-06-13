#!/bin/sh
# openbao-init: tiny auto-init / auto-unseal daemon for the ai-compose stack.
# Loops forever: initializes OpenBao on first boot (1 share / 1 threshold),
# unseals it after any restart, ensures the KV v2 "secret/" mount, and ensures a
# fixed service token (opord-ai-root) so the api/worker VAULT_TOKEN keeps working
# across re-creations. The unseal key + root token live in the data volume
# (.init.txt) - acceptable for a LOCAL test stack, not a production pattern.
set -u
export BAO_ADDR="${BAO_ADDR:-http://openbao:8200}"
INIT_FILE=/openbao/file/.init.txt

ensure() {
  # bao status exit codes: 0 = unsealed, 2 = sealed, 1 = unreachable/error.
  bao status >/dev/null 2>&1
  rc=$?
  [ "$rc" -eq 1 ] && return 0 # server not up yet - retry on the next tick

  if ! bao status 2>/dev/null | grep -q 'Initialized.*true'; then
    echo "[openbao-init] initializing (1 share / 1 threshold)"
    bao operator init -key-shares=1 -key-threshold=1 >"$INIT_FILE" 2>/dev/null || return 0
    chmod 600 "$INIT_FILE"
  fi

  if bao status 2>/dev/null | grep -q 'Sealed.*true'; then
    KEY=$(grep 'Unseal Key 1:' "$INIT_FILE" 2>/dev/null | awk '{print $NF}')
    if [ -n "${KEY:-}" ]; then
      bao operator unseal "$KEY" >/dev/null 2>&1 && echo "[openbao-init] unsealed"
    else
      echo "[openbao-init] sealed but no unseal key in $INIT_FILE - manual unseal needed"
      return 0
    fi
  fi

  ROOT=$(grep 'Initial Root Token:' "$INIT_FILE" 2>/dev/null | awk '{print $NF}')
  [ -z "${ROOT:-}" ] && return 0
  export BAO_TOKEN="$ROOT"

  # KV v2 at secret/ (what the OPORD resolver + `bao kv` expect). Idempotent.
  if ! bao secrets list 2>/dev/null | grep -q '^secret/'; then
    bao secrets enable -path=secret -version=2 kv >/dev/null 2>&1 &&
      echo "[openbao-init] kv-v2 mounted at secret/"
  fi

  # Fixed service token so VAULT_TOKEN=opord-ai-root keeps working. Idempotent.
  if ! BAO_TOKEN=opord-ai-root bao token lookup >/dev/null 2>&1; then
    bao token create -id=opord-ai-root -policy=root -orphan -ttl=0 >/dev/null 2>&1 ||
      bao token create -id=opord-ai-root -policy=root -orphan >/dev/null 2>&1
    BAO_TOKEN=opord-ai-root bao token lookup >/dev/null 2>&1 &&
      echo "[openbao-init] service token opord-ai-root ensured"
  fi
}

echo "[openbao-init] watching $BAO_ADDR"
while true; do
  ensure
  sleep 5
done
