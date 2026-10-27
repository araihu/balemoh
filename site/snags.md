# Static site integration notes

## Shared favicon rejected by Margo

- Question: can Margo use the shared adaptive Balemoh favicon as its site logo?
- Source: `site.yaml` with `ui/internal/view/static/brand/favicon.svg`.
- Evidence: `site.asset_invalid: external or escaped style syntax is forbidden`.
  The shared SVG contains a `@media` rule and CSS custom properties.
- Workaround: use the existing passive light wordmark as the logo, PNG favicon,
  and a light site theme. Override the shells' square-icon dimensions for the
  full wordmark and suppress their duplicate product-name labels. Shared source
  assets remain intact.
- Upstream opportunity: document adaptive SVG restrictions beside Margo's
  `site.logo` contract, or explicitly support safe local color-scheme rules.

## Cross-generator branding in the Manja export

- Question: can the API export use Balemoh's same-origin shared favicon?
- Source: `site/renderer.yaml`, `catalogs[].branding.logo.src`.
- Evidence: `external export resource is not supported`, including an HTTPS
  logo on the site's production origin.
- Workaround: retain Manja's bundled renderer branding. Its title, API source,
  descriptions, canonical URLs, and social image identify Balemoh.
- Upstream opportunity: document static export branding restrictions and add
  explicit local asset acquisition for branded exports.

## Reserved API segment in static publication paths

- Question: can the static reference be mounted at `/api/`?
- Source: Manja v0.1.16 `manja-assets/local-docs.js`, `validBase` and `publicPath`.
- Evidence: export and receipt verification succeed, but the browser marks the
  document `fallback` with `descriptor identity is invalid`. Those validators
  reject any path segment named `api` or `manage`, including a static base path.
- Workaround: publish the reference at `/openapi/`. Verify operation navigation,
  direct-link reload, schema expansion, and search in a browser before deployment.
- Upstream opportunity: reject reserved base paths during `manja export`, with
  a diagnostic that names the path, instead of publishing an unusable export.

## Cloudflare script injection changes exported HTML

- Question: does the custom domain serve the exact exported bytes?
- Evidence: HTTPS returned 200, but SHA-256 differed. The response appended
  `/cdn-cgi/challenge-platform/` JavaScript Detections to the generated HTML.
- Workaround: send `Cache-Control: no-transform` from the static publication,
  then check deployed shell, operation, and sidecar bytes against local files.
- Reference: Cloudflare documents that JavaScript Detections injection is
  disabled for responses with `no-transform`. No zone-wide settings are changed.

## Product screenshots in Margo's landing hero

- Question: can a screenshot use the landing hero without losing UI content?
- Source: Margo `.margo-landing-hero__visual img` defaults to a 4:3 cover crop
  and a compact visual column.
- Workaround: preserve the 16:9 capture, use a full-width visual, and keep
  the surrounding copy short. The source screenshots remain unmodified.
- Upstream opportunity: document a product-screenshot layout alongside the
  illustration-oriented landing example.

## OpenAPI document descriptions in Manja

- Question: does `info.description` render Markdown headings and links?
- Evidence: Manja v0.1.16's static overview displayed `##` and `**` literally.
  `internal/localdocs/render/document_header.templ`, `catalogDocumentHeader`,
  renders `data.Description` in a paragraph with `whitespace-pre-wrap`.
- Workaround: use readable plain paragraphs in the specification overview and
  keep the structured integration tutorial in the Margo reference section.
  Operation descriptions, examples, and tags remain in the OpenAPI source.
- Upstream opportunity: document which description surfaces render CommonMark,
  or render the OpenAPI document description through the supported Markdown path.
