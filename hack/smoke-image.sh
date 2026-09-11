#!/usr/bin/env bash
set -euo pipefail
image="${1:?usage: smoke-image.sh IMAGE}"
api_id=""
ui_id=""
cleanup() {
  for id in "$ui_id" "$api_id"; do
    if [[ -n "$id" ]]; then docker rm -f "$id" >/dev/null; fi
  done
}
trap cleanup EXIT
api_id=$(docker run -d --read-only --tmpfs /data:uid=65532,gid=65532 --tmpfs /tmp \
  -e BALEMOH_DISCOVERY_SYNC_INTERVAL=0 -p 127.0.0.1::8080 -p 127.0.0.1::8081 "$image")
ui_id=$(docker run -d --read-only --tmpfs /data:uid=65532,gid=65532 --tmpfs /tmp \
  --network "container:$api_id" --entrypoint /balemoh-ui \
  -e BALEMOH_UI_API_BASE_URL=http://127.0.0.1:8080 "$image")
api_address=$(docker port "$api_id" 8080/tcp)
ui_address=$(docker port "$api_id" 8081/tcp)
for address in "$api_address" "$ui_address"; do
  curl --retry 15 --retry-delay 1 --retry-connrefused --max-time 5 -fsS "http://$address/healthz"
done
curl -fsS "http://$ui_address/" | grep -q 'No pinned services'
echo 'API and UI image smoke checks passed'
