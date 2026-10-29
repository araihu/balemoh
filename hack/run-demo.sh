#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/balemoh-demo.XXXXXX")
credentials="$scratch/credentials"
mkdir -m 0755 "$credentials"
export BALEMOH_DEMO_CREDENTIAL_DIR="$credentials"
# The parent directory remains private. Container UID 65532 must read the files.
openssl rand -hex 32 > "$credentials/plugin-token"
openssl rand -hex 32 > "$credentials/operator-token"
chmod 0444 "$credentials"/*
project="balemoh-demo-$$"
cleanup() {
  docker compose -p "$project" -f "$root/demo/compose.yaml" down --volumes >/dev/null 2>&1 || true
  rm -rf -- "$scratch"
}
trap cleanup EXIT
printf 'Demo will be available at http://127.0.0.1:18081. Ctrl-C removes its disposable data.\n'
docker compose -p "$project" -f "$root/demo/compose.yaml" up --build --abort-on-container-failure
