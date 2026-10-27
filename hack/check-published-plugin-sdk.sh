#!/usr/bin/env bash
set -euo pipefail
version=${1:?usage: check-published-plugin-sdk.sh VERSION}
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || exit 2
work=$(mktemp -d /tmp/balemoh-sdk-release.XXXXXX)
cleanup() {
  status=$?
  GOMODCACHE="$work/modules" go clean -modcache || true
  if ! rm -rf "$work"; then
    if [[ "$status" == 0 ]]; then status=1; fi
  fi
  exit "$status"
}
trap cleanup EXIT
mkdir -p "$work/bin" "$work/tmp"
export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GOPRIVATE= GONOPROXY= GONOSUMDB=
export GOMODCACHE="$work/modules" GOBIN="$work/bin" TMPDIR="$work/tmp"
sdk=github.com/araihu/balemoh/plugin-sdk
go install "$sdk/cmd/balemoh-plugin@$version"
test "$(head -n 1 "$GOMODCACHE/$sdk@$version/LICENSE")" = 'MIT License'
for kind in ui backend; do
  flags=()
  if [[ "$kind" == backend ]]; then flags=(--backend); fi
  source="$work/$kind"
  "$GOBIN/balemoh-plugin" init --id "example.$kind" --module "example.com/$kind" --output "$source" "${flags[@]}"
  (
    cd "$source"
    go generate ./...
    go mod tidy
    go test ./...
    go vet ./...
    test "$(go list -m -f '{{.Version}}' "$sdk")" = "$version"
    test "$(go list -m -f '{{if .Replace}}replacement{{end}}' "$sdk")" = ''
    test -n "$(go list -m -f '{{.Sum}}' "$sdk")"
    "$GOBIN/balemoh-plugin" package --source . --output dist
    "$GOBIN/balemoh-plugin" check --package dist
  )
done
echo "Published SDK $version resolved, scaffolded and packaged without replacements"
