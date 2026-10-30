#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
image=${1:?usage: smoke-demo.sh IMAGE}
scratch=$(mktemp -d "${TMPDIR:-/tmp}/balemoh-demo-smoke.XXXXXX")
export BALEMOH_DEMO_CREDENTIAL_DIR="$scratch/credentials"
export BALEMOH_DEMO_PORT=0
mkdir -m 0755 "$BALEMOH_DEMO_CREDENTIAL_DIR"
openssl rand -hex 32 > "$BALEMOH_DEMO_CREDENTIAL_DIR/plugin-token"
openssl rand -hex 32 > "$BALEMOH_DEMO_CREDENTIAL_DIR/operator-token"
chmod 0444 "$BALEMOH_DEMO_CREDENTIAL_DIR"/*
project="balemoh-demo-smoke-$$"
compose=(docker compose -p "$project" -f "$root/demo/compose.yaml")
cleanup() {
  "${compose[@]}" down --volumes >/dev/null 2>&1 || true
  rm -rf -- "$scratch"
}
trap cleanup EXIT
docker tag "$image" balemoh:public-demo
"${compose[@]}" up -d --no-build >/dev/null
origin="http://$("${compose[@]}" port api 8081)"
ready() {
  local deadline=$((SECONDS + 90))
  until curl --silent --fail --max-time 3 "$origin/healthz" >/dev/null; do
    if (( SECONDS >= deadline )); then echo 'Demo did not become ready' >&2; return 1; fi
    sleep 1
  done
}
ready
for visitor in first second; do
  curl --silent --fail --max-time 10 -H "Cookie: visitor=$visitor" "$origin/" > "$scratch/home.html"
  python3 - "$scratch/home.html" "$origin" <<'PY'
import re, sys, urllib.request
body=open(sys.argv[1]).read()
assert 'Shared read-only demo' in body and 'grafana' in body
assert 'hx-sse:connect=' not in body
images=set(re.findall(r'src="(/ui/icon-library/selfhst/images/[a-f0-9]+\.png)"', body))
assert len(images) == 12, f'Expected twelve service logos, got {len(images)}'
for path in images:
    with urllib.request.urlopen(sys.argv[2] + path, timeout=5) as response:
        assert response.status == 200 and response.read(8) == b'\x89PNG\r\n\x1a\n', path
PY
  for action in /staging/sync /notifications/read-all /notifications/clear /presentation/save /icons/upload; do
    status=$(curl --silent --max-time 10 -o /dev/null -w '%{http_code}' -X POST -H "Cookie: visitor=$visitor" "$origin$action")
    [[ "$status" == 403 ]]
  done
done
# Poll the real plugin UI contribution, not a preview or fake BFF response.
deadline=$((SECONDS + 90))
until curl --silent --fail --max-time 5 "$origin/plugin-widgets/00000000-0000-4000-8000-000000000002" > "$scratch/widget.html" &&
  python3 - "$scratch/widget.html" 2>/dev/null <<'PY'
import sys
assert 'DemoServiceRestarts' in open(sys.argv[1]).read()
PY
do
  if (( SECONDS >= deadline )); then echo 'Synthetic plugin data unavailable' >&2; exit 1; fi
  sleep 1
done
"${compose[@]}" restart api >/dev/null
api_id=$("${compose[@]}" ps -q api)
deadline=$((SECONDS + 90))
until [[ "$(docker inspect --format '{{.State.Health.Status}}' "$api_id")" == healthy ]]; do
  if (( SECONDS >= deadline )); then echo 'Demo API did not recover' >&2; exit 1; fi
  sleep 1
done
"${compose[@]}" up -d --no-build --force-recreate ui >/dev/null
origin="http://$("${compose[@]}" port api 8081)"
ready
curl --silent --fail --max-time 10 "$origin/staging" > "$scratch/staging.html"
python3 - "$scratch/staging.html" <<'PY'
import sys
body=open(sys.argv[1]).read()
assert 'grafana' in body and 'minio' in body and '<fieldset disabled' in body
PY
echo 'Shared read-only demo: PASS'
