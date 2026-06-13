#!/usr/bin/env bash
# Fetch external OpenBao secrets-engine plugins into ./plugins so the openbao
# container can register them (dynamic short-lived cloud creds, ADR-0010).
# Binaries are NOT committed to git (see .gitignore) - run this once after
# cloning. MPL-2.0 plugins from the first-party openbao/openbao-plugins repo
# (no BUSL - keeps the OSS posture).
#
# Usage: ./fetch-plugins.sh   (linux/amd64 by default; override with ARCH=arm64)
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p plugins

ARCH="${ARCH:-amd64}"
# engine=tag pairs. Bump tags to upgrade a plugin.
PLUGINS=(
  "gcp=secrets-gcp-v0.23.0"
  "aws=secrets-aws-v0.3.0-beta20260326"
  "azure=secrets-azure-v0.23.0"
)

fetch() {
  local eng="$1" tag="$2"
  local base="https://github.com/openbao/openbao-plugins/releases/download/${tag}"
  local asset="openbao-plugin-secrets-${eng}_linux_${ARCH}_v1.tar.gz"
  echo " to ${eng} (${tag})"
  curl -fsSL -o "plugins/${asset}" "${base}/${asset}"
  curl -fsSL -o "plugins/ck-${eng}.txt" "${base}/checksums-secrets-${eng}.txt"
  local want got
  want=$(grep "linux_${ARCH}_v1.tar.gz\$" "plugins/ck-${eng}.txt" | awk '{print $1}')
  got=$(shasum -a 256 "plugins/${asset}" | awk '{print $1}')
  [ "$want" = "$got" ] || { echo "❌ ${eng} checksum mismatch"; exit 1; }
  tar -xzf "plugins/${asset}" -C plugins
  mv -f "plugins/openbao-plugin-secrets-${eng}_linux_${ARCH}_v1" "plugins/openbao-plugin-secrets-${eng}"
  chmod +x "plugins/openbao-plugin-secrets-${eng}"
  rm -f "plugins/${asset}" "plugins/ck-${eng}.txt"
  shasum -a 256 "plugins/openbao-plugin-secrets-${eng}" | awk '{print $1}' > "plugins/.plugin-sha256-${eng}"
  echo "✅ plugins/openbao-plugin-secrets-${eng}"
}

for pair in "${PLUGINS[@]}"; do
  fetch "${pair%%=*}" "${pair#*=}"
done
echo "✅ all plugins fetched. Register each: bao plugin register -sha256=\$(cat plugins/.plugin-sha256-<eng>) -command=openbao-plugin-secrets-<eng> secret <eng>"
