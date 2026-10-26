#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
export GOWORK=off

# Keep publication tools independent from the application's module graph.
margo_version=v0.0.25
manja_version=v0.1.16
mkdir -p bin site/.build dist
if ! bash hack/lint-openapi.sh > site/.build/vacuum-report.txt 2>&1; then
  cat site/.build/vacuum-report.txt >&2
  exit 1
fi
GOBIN="$root/bin" go install "github.com/araihu/margo/cmd/margo@$margo_version"
GOBIN="$root/bin" go install "github.com/araihu/manja/cmd/manja@$manja_version"

python3 - <<'PY'
from pathlib import Path
import shutil

content = Path("site/.build/content")
if content.exists():
    shutil.rmtree(content)
shutil.copytree("site/content", content)
for source, destination, title, description in [
    ("internal/config/environment.md", "api-environment/index.md", "API environment",
     "Generated configuration for the Balemoh API, discovery sources, telemetry, and federation."),
    ("ui/internal/config/environment.md", "ui-environment/index.md", "UI environment",
     "Generated configuration for the Balemoh UI and its API connection."),
    ("docs/icons.md", "icons/index.md", "Icon library",
     "Bundled icon collections, uploads, provenance, and persistent storage."),
]:
    frontmatter = f"---\ntitle: {title}\ndescription: {description}\nlanguage: en\n---\n\n"
    target = content / "reference" / destination
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(frontmatter + Path(source).read_text())
PY

stage="$(mktemp -d "$root/site/.build/publication.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
bin/margo site site.yaml --output-dir "$stage/public" --diagnostics text \
  > site/.build/margo-report.json
bin/manja export --renderer-config site/renderer.yaml \
  --data-dir site/.build/manja-data --output "$stage/manja" --base-path /openapi/ \
  > site/.build/manja-report.json
bin/manja export verify --output "$stage/manja" > site/.build/manja-verification.json
mv "$stage/manja" "$stage/public/openapi"
cp site/headers "$stage/public/_headers"
python3 hack/site_structured_data.py "$stage/public"
python3 hack/check-site.py "$stage/public"

# Replace only generated output after both exporters and verification succeed.
if test -e dist/site; then
  mv dist/site "$stage/previous"
fi
if ! mv "$stage/public" dist/site; then
  if test -e "$stage/previous"; then mv "$stage/previous" dist/site; fi
  exit 1
fi
printf 'Static site ready: %s/dist/site\n' "$root"
