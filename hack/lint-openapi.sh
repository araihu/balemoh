#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
exec hack/vacuum lint api/openapi.yaml --ruleset api/vacuum.yaml \
  --fail-severity warn --min-score 100 --details --all-results --no-style --no-banner --no-update-check
