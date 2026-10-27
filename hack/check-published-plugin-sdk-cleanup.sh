#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
cat > "$fixture/go" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == install ]]; then exit 37; fi
if [[ "$1" == clean ]]; then
  printf '%s' "$GOMODCACHE" > "$SMOKE_CLEANUP_RECEIPT"
  exit 41
fi
exit 99
STUB
chmod +x "$fixture/go"
status=0
SMOKE_CLEANUP_RECEIPT="$fixture/cache" PATH="$fixture:$PATH" \
  bash "$root/hack/check-published-plugin-sdk.sh" v0.0.1-alpha.1 || status=$?
test "$status" = 37
test -s "$fixture/cache"
cache=$(cat "$fixture/cache")
[[ "$cache" == /tmp/balemoh-sdk-release.*/modules ]]
test ! -e "${cache%/modules}"
echo 'SDK smoke preserves failure status and removes its owned temporary tree'
