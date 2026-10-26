#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
mkdir -p bin site/.build/screenshots
GOWORK=off go build -o bin/balemoh-screenshots ./cmd/balemoh
(cd ui && GOWORK=off go build -o ../bin/balemoh-ui-screenshots ./cmd/balemoh-ui)
run="$(mktemp -d "$root/site/.build/screenshots/run.XXXXXX")"
python3 - <<'PY'
import socket
for port in (8180, 8181):
    with socket.socket() as listener:
        listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        listener.bind(("127.0.0.1", port))
        listener.listen()
PY
api_pid=""
ui_pid=""
seed_pid=""
trap 'for pid in "$seed_pid" "$ui_pid" "$api_pid"; do if test -n "$pid"; then kill "$pid" 2>/dev/null || true; fi; done' EXIT
sources='container/atlas,container/pico,kubernetes/lab-cluster'
tokens='container/atlas=synthetic-screenshots-only,container/pico=synthetic-screenshots-only,kubernetes/lab-cluster=synthetic-screenshots-only'
BALEMOH_HTTP_ADDR=127.0.0.1:8180 BALEMOH_DATABASE_PATH="$run/balemoh.db" \
BALEMOH_DISCOVERY_SYNC_INTERVAL=0 BALEMOH_CONTAINER_ENABLED=false \
BALEMOH_KUBERNETES_ENABLED=false BALEMOH_TELEMETRY_ENABLED=false \
BALEMOH_FEDERATION_GATEWAY_URL='' BALEMOH_FEDERATION_ALLOWED_SOURCES="$sources" \
BALEMOH_FEDERATION_SOURCE_TOKENS="$tokens" \
bin/balemoh-screenshots > "$run/api.log" 2>&1 &
api_pid=$!
for attempt in {1..50}; do
  if curl --silent --fail http://127.0.0.1:8180/healthz > /dev/null; then break; fi
  if ! kill -0 "$api_pid" 2>/dev/null; then cat "$run/api.log"; exit 1; fi
  sleep 0.1
done
BALEMOH_UI_HTTP_ADDR=127.0.0.1:8181 BALEMOH_UI_API_BASE_URL=http://127.0.0.1:8180 \
BALEMOH_UI_ICON_CACHE_DIR="$root/site/.build/screenshots/icons" \
bin/balemoh-ui-screenshots > "$run/ui.log" 2>&1 &
ui_pid=$!
python3 hack/seed-site-screenshots.py --keep-fresh &
seed_pid=$!
wait "$seed_pid"
