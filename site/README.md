# Static site

The site follows Margo's own showcase pipeline. Margo builds the landing page
and guides. Manja exports the repository's OpenAPI specification under `/openapi/`.
Cloudflare Workers Static Assets publishes the combined tree without a request
handler, API server, or Manja server.

## Build and preview

From the repository root, with Go 1.27.3+ and Python 3.9+:

```sh
bash hack/build-site.sh
python3 -m http.server 8090 --directory dist/site
```

The script installs Margo v0.0.25 and Manja v0.1.17 into ignored `bin/`.
It copies the existing envdoc outputs and icon guide into an ignored content
tree, builds fresh artifacts, verifies the Manja receipt, checks metadata and
cross-generator links, then replaces `dist/site`. Failed builds leave the last
successful publication intact. Build receipts and Manja state stay outside the
published directory in `site/.build/`.

Edit Markdown in `site/content/`, routing and identity in `site.yaml`, and
OpenAPI configuration in `site/renderer.yaml`. Change the API contract in
`api/openapi.yaml`, not the exported HTML. Regenerate the environment references
with the project's normal `go generate` commands when changing configuration.

The 1280×640 social card is generated from `site/assets/social-v1.svg` using
`rsvg-convert -o site/assets/social-v1.png site/assets/social-v1.svg`.
Its mark comes from the shared Balemoh favicon already vendored by the UI.

## Deploy

```sh
wrangler login
bash hack/build-site.sh
wrangler deploy --dry-run
wrangler deploy
```

`wrangler.jsonc` uses the same Cloudflare account as Margo, with a separate
`balemoh-site` Worker and `balemoh.araihu.com` custom domain. Deployment replaces
the whole static publication. Roll back by rebuilding a prior revision and
deploying its complete artifact.

The GitHub workflow builds and checks relevant pull requests and pushes to
`main`. Only `release.published` for canonical stable SemVer tags deploys.
Tags must have the form `vMAJOR.MINOR.PATCH`, with no leading zeroes, prerelease
suffix, or build metadata. Examples: `v0.0.7` and `v1.2.3`. Releases marked as
prerelease or draft never deploy, even if the tag has the canonical form.
Other tags, pull requests, and ordinary pushes only validate. Re-run a release's
failed workflow to retry it.

The workflow builds the release commit, sets the site's displayed and JSON-LD
version from its `vMAJOR.MINOR.PATCH` tag, and validates the publication before
deployment. It deploys the saved
artifact from that same run, then compares production files by SHA-256. The
workflow, site sources, and build helpers must be committed in the release tag.

The `production` environment has `CLOUDFLARE_API_TOKEN`, provisioned through the
Cloudflare MCP as account-owned token `balemoh-github-actions-static-workers`.
It grants Workers Editor in this site's account and Zone Read plus Workers
Routes Write only for `araihu.com`. Cloudflare currently does not support
per-Worker roles for Custom Domains, so Worker access uses the account scope.
See [Workers permissions and limitations](https://developers.cloudflare.com/workers/authorization/workers/#limitations).

Local Wrangler OAuth is not used by CI. For rotation, create an account-owned
token with the same permissions and enter its value securely:

```sh
gh secret set CLOUDFLARE_API_TOKEN --repo araihu/balemoh --env production
```

The deploy job uses the `production` environment. Its token is never passed to
pull-request or build jobs. Builds include hidden publication assets in the
uploaded artifact so `.well-known` resources survive the transfer between jobs.

Serve HTML directory indexes and real 404s. Keep Wasm MIME types intact.
Do not enable HTML rewriting, minification, or SPA fallback for Manja assets:
fragments and their sidecars are verified by hash in the browser. Manja's
service worker is scoped to `/openapi/`. The API pages use browser JavaScript; every request is served from the static export.

The publication sends `Cache-Control: no-transform` to prevent Cloudflare
JavaScript Detections from modifying hash-verified fragments. This is scoped
to this Worker's static responses. See Cloudflare's
[cache-control behavior](https://developers.cloudflare.com/cache/concepts/cache-control/#javascript-detections).

API links use the production origin so Margo can validate its own route tree
independently. During local preview, open
`/openapi/catalogs/balemoh/documents/openapi/` directly.

## Validation

The build adds JSON-LD to the initial HTML of the homepage, guides, and reference
pages. `site/structured-data.json` owns the application's factual properties;
the version comes from `site.yaml`. Page names, descriptions, and URLs come from
the rendered HTML. The graph uses `SoftwareApplication`, `WebSite`, `WebPage`,
`CollectionPage`, and `TechArticle`. The reference index links the OpenAPI
documentation as `APIReference`. Manja's exported files and hash receipts remain
unchanged. No schema is added to its search or offline shells.

`hack/site_structured_data.py` runs before publication validation. The checker
rejects missing or duplicate JSON-LD, invalid JSON, incorrect entity types, and
page metadata mismatches. Schema.org validation and Google's rich-result
eligibility are separate checks. The zero-price offer matches the public GitHub
release downloads and the visible homepage copy. There are no invented ratings,
reviews, or license claims. Google can still report optional rating warnings.
Technical articles use their existing Open Graph preview as the article image.
Documentation author is AraiHu, represented as an Organization with its public
website. The checker requires this author on articles and linked API references;
ratings, reviews, and offer currency must remain absent.

```sh
python3 hack/check-site.py dist/site
python3 hack/check-site-live.py
bin/manja export verify --output dist/site/openapi
wrangler dev --port 8091
```

Check home, a guide, the OpenAPI document, operation selection, schema expansion,
search, a direct-operation reload, and a missing route. Deployment must serve
the social card as `image/png` and Wasm as `application/wasm`.

The live check compares every published file by SHA-256, verifies MIME types,
and checks real 404 responses. If local DNS still caches the domain's previous
NXDOMAIN, pass `--resolve PUBLIC_IP` using an address confirmed by public DNS.

The public documentation exposes only repository-owned documentation and its
OpenAPI contract. The existing private Balemoh instance remains a separate
deployment.
