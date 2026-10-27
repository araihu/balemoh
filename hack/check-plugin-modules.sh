#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
allow_untracked=false
consumer=""
for argument in "$@"; do
 case "$argument" in
  --allow-untracked) allow_untracked=true ;;
  --alertmanager=*) consumer=${argument#*=} ;;
  *) echo 'usage: check-plugin-modules.sh [--allow-untracked] [--alertmanager=/absolute/repo]' >&2; exit 2 ;;
 esac
done
export GOWORK=off
receipt=$(mktemp -d)
trap 'rm -rf "$receipt"' EXIT
snapshot() {
 python3 - "${1:-$root}" <<'PY'
import hashlib,json,pathlib,subprocess,sys
root=pathlib.Path(sys.argv[1]);names=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard'],cwd=root,text=True).splitlines()
result={}
for name in sorted(set(names)):
 p=root/name
 if name in ('go.mod','go.sum') or name.endswith(('/go.mod','/go.sum','_templ.go','.pb.go','.gen.go','environment.md')) or '/storage/sqlc/' in name:
  result[name]=hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None
print(json.dumps(result,sort_keys=True))
PY
}
snapshot > "$receipt/before.json"
for module in . client ui plugin-sdk; do
 go -C "$module" generate ./...
 go -C "$module" mod tidy
 go -C "$module" test ./...
 go -C "$module" vet ./...
done
snapshot > "$receipt/after.json"
cmp "$receipt/before.json" "$receipt/after.json" || { echo 'Generated output changed or appeared. Review and commit generated files.' >&2; exit 1; }
if ! "$allow_untracked"; then
 python3 - "$receipt/after.json" <<'PY'
import json,subprocess,sys
for name in json.load(open(sys.argv[1])):
 subprocess.run(['git','ls-files','--error-unmatch',name],check=True,stdout=subprocess.DEVNULL)
PY
fi
if [[ -n "$consumer" ]]; then
 snapshot "$consumer" > "$receipt/consumer-before.json"
 make -C "$consumer" test SDK_DIR="$root/plugin-sdk"
 snapshot "$consumer" > "$receipt/consumer-after.json"
 cmp "$receipt/consumer-before.json" "$receipt/consumer-after.json" || { echo "Consumer generated output changed or appeared" >&2; exit 1; }
 git -C "$consumer" diff --check
 CGO_ENABLED=0 go build -o "$receipt/balemoh" ./cmd/balemoh
 CGO_ENABLED=0 go -C ui build -o "$receipt/balemoh-ui" ./cmd/balemoh-ui
 BALEMOH_TEST_API_BINARY="$receipt/balemoh" BALEMOH_TEST_UI_BINARY="$receipt/balemoh-ui" go -C "$consumer" test -modfile=.local/development.mod -count=1 ./integration
fi
git diff --check
echo 'Plugin modules and generated outputs verified'
