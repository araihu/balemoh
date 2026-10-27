#!/usr/bin/env bash
set -euo pipefail
image="${1:?usage: smoke-image.sh IMAGE}"
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
api_id=""
ui_id=""
cleanup() {
 for id in "$ui_id" "$api_id"; do if [[ -n "$id" ]]; then docker rm -f "$id" >/dev/null; fi; done
 rm -rf "$work"
}
trap cleanup EXIT
architecture=$(docker info --format '{{.Architecture}}')
case "$architecture" in aarch64|arm64) architecture=arm64;; x86_64|amd64) architecture=amd64;; *) echo 'Unsupported Docker architecture' >&2; exit 1;; esac
export GOWORK=off
go -C "$root/plugin-sdk" run ./cmd/balemoh-plugin init --id example.smoke --module example.com/smoke --output "$work/source"
cp "$work/source/go.mod" "$work/development.mod"
go -C "$work/source" mod edit -modfile="$work/development.mod" -replace="github.com/araihu/balemoh/plugin-sdk=$root/plugin-sdk"
go -C "$work/source" run -mod=mod -modfile="$work/development.mod" github.com/a-h/templ/cmd/templ generate
go -C "$work/source" mod tidy -modfile="$work/development.mod"
go -C "$root/plugin-sdk" run ./cmd/balemoh-plugin package --source "$work/source" --output "$work/package" --target "linux/$architecture" --modfile "$work/development.mod" > "$work/pins.json"
mkdir "$work/config"
python3 - "$work" <<'PY'
import json,pathlib,secrets,sys
root=pathlib.Path(sys.argv[1]);pins=json.loads((root/'pins.json').read_text());token=secrets.token_hex(32)
(root/'config'/'token').write_text(token);(root/'curl.conf').write_text('header = "Authorization: Bearer '+token+'"\n')
(root/'config'/'api.json').write_text(json.dumps({'installations':[{'package':'/package',**pins,'connections':[{'key':'preview','name':'Image fixture','config':{},'secrets':{},'grants':[]}]}]}))
(root/'config'/'ui.json').write_text(json.dumps({'installations':[{'package':'/package',**pins,'connectionKey':'preview','previewWidgets':[]}]}))
PY
chmod 755 "$work/config"
chmod 600 "$work/curl.conf"
chmod 444 "$work/config/"*
api_id=$(docker run -d --read-only --tmpfs /data:uid=65532,gid=65532 --tmpfs /tmp \
 -v "$work/package:/package:ro" -v "$work/config:/config:ro" \
 -e BALEMOH_DISCOVERY_SYNC_INTERVAL=0 -e BALEMOH_PLUGIN_CONFIG=/config/api.json -e BALEMOH_PLUGIN_SERVICE_TOKEN_FILE=/config/token \
 -p 127.0.0.1::8080 -p 127.0.0.1::8081 "$image")
ui_id=$(docker run -d --read-only --tmpfs /data:uid=65532,gid=65532 --tmpfs /tmp \
 -v "$work/package:/package:ro" -v "$work/config:/config:ro" --network "container:$api_id" --entrypoint /balemoh-ui \
 -e BALEMOH_UI_API_BASE_URL=http://127.0.0.1:8080 -e BALEMOH_UI_ICON_STARTUP_TIMEOUT=10ms \
 -e BALEMOH_UI_PLUGIN_CONFIG=/config/ui.json -e BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE=/config/token "$image")
api_address=$(docker port "$api_id" 8080/tcp)
ui_address=$(docker port "$api_id" 8081/tcp)
for address in "$api_address" "$ui_address"; do
 curl --retry 20 --retry-delay 1 --retry-all-errors --max-time 5 -fsS "http://$address/healthz"
done
curl --config "$work/curl.conf" -fsS "http://$api_address/api/v1/plugins" > "$work/catalog.json"
connection=$(python3 - "$work/catalog.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))['plugins'][0]['connections'][0]['id'])
PY
)
python3 - "$connection" > "$work/placement.json" <<'PY'
import json,sys
print(json.dumps({'pluginId':'example.smoke','connectionId':sys.argv[1],'contributionId':'summary','options':{},'enabled':True}))
PY
curl --config "$work/curl.conf" -fsS -X PUT -H 'Content-Type: application/json' --data-binary "@$work/placement.json" "http://$api_address/api/v1/plugin-widgets/00000000-0000-4000-8000-000000000002" > /dev/null
curl --retry 15 --retry-delay 1 --retry-all-errors --max-time 5 -fsS "http://$ui_address/plugins/example.smoke/connections/$connection/page-content/current" > "$work/page.html"
grep -q 'Independent UI plugin' "$work/page.html"
curl -fsS "http://$ui_address/plugin-widgets/00000000-0000-4000-8000-000000000002" > "$work/widget.html"
grep -q 'Independent UI plugin' "$work/widget.html"
curl -fsS "http://$ui_address/" > "$work/home.html"
grep -q 'data-plugin-slot' "$work/home.html"
echo 'Non-root read-only API/UI image and independent plugin smoke passed'
