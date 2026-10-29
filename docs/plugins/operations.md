# Trusted application plugins

Install matching SDK/API/BFF alpha revisions. Plugins are opt-in; an empty config
runs ordinary Balemoh. The core API persists connection metadata and widget
placements, not source secrets or rendered HTML. API owns backend collection; BFF
owns UI rendering. Both hosts load the same immutable package and exact pins.

## Installation

Build the independent plugin with `balemoh-plugin package`; run `check` on a native
target. Deliver its directory read-only to both hosts. The package contains
`plugin.json`, target executables and declared schemas/assets. Keep deployment
JSON and credentials outside it. The package digest covers names, modes and bytes;
rebuilding or chmod requires new pins. Only trusted local packages are supported.

Create a private service-token file with at least 32 bytes of random Bearer text.
Mount it in API and BFF only. Source credentials are separate files mounted only
in API. Environment reference is generated in `internal/config/environment.md` and
`ui/internal/config/environment.md`; use the actual generated variable names.

API deployment JSON:

```json
{"installations":[{"package":"/plugins/alertmanager","manifestDigest":"MANIFEST_SHA256","packageDigest":"PACKAGE_SHA256","connections":[{"key":"homelab","name":"Homelab Alertmanager","config":{"apiBaseURL":"http://kube-prometheus-stack-alertmanager.monitoring.svc:9093","browserBaseURL":"https://alertmanager.decastro.me","allowHTTP":true,"intervalSeconds":30},"secrets":{},"grants":["alerts.list","alerts.get"]}]}]}
```

BFF deployment JSON:

```json
{"installations":[{"package":"/plugins/alertmanager","manifestDigest":"MANIFEST_SHA256","packageDigest":"PACKAGE_SHA256","connectionKey":"homelab"}]}
```

Set API `BALEMOH_PLUGIN_CONFIG` and `BALEMOH_PLUGIN_SERVICE_TOKEN_FILE`.
Set BFF `BALEMOH_UI_PLUGIN_CONFIG`, `BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE` and
`BALEMOH_UI_API_BASE_URL`. Use an internal API service. Never publish private
plugin routes through a browser ingress. Do not combine production and legacy
preview configuration. Production widgets are selected through persisted controls;
`previewWidgets` is not production pin state.

`apiBaseURL` is source reachability from API, without `/api/v2/alerts` suffix.
`browserBaseURL` is browser reachability; omit it to hide source links. HTTPS
validates certificates. HTTP requires explicit trusted-network `allowHTTP`.
The homelab uses HTTP only inside the cluster and HTTPS on its internal gateway.
No history, silence creation or source mutation is implemented.

## Runtime and controls

API provisions stable connection UUIDs by plugin/key. Equivalent config reuses its
revision; changes advance it. Omission disables and retains previous records. One
backend owner collects each connection independently of browser traffic. Use one
API replica. Multiple writers and distributed collection are unsupported.

Each connection adds its declared sidebar pages and Configure widgets link. Create
several placements with separate options. Enabled placements and retained disabled
placements share the 128-selection limit with host selections. UUID create replay
is idempotent; updates/deletes require current revision. Conflicts retain submitted
values and ask the user to reload. Service/catalog pin state remains independent.

Home, page shells and dashboard SSE never call plugin UI. Isolated content calls
have finite deadlines. Failed source reads retain the complete last observation as
stale. A first failure is unavailable, never an empty healthy source. Failed UI
renders have local retry and may show a matching retained render; current grants,
connection and placement revisions must still match. Native iframe fallback keeps
no-JavaScript content separate from base navigation.

Child startup has a 10-second deadline; UI/backend invocations at most five seconds.
Supervision uses 1/2/4/8/16-second backoff, five restarts per five-minute window and
resets after five healthy minutes. Health runs every 10 seconds with a two-second
budget; three failures restart the child. Source stale state does not restart it.
Logs carry bounded state/duration/restart/request metadata and sanitized errors.

## Disable, update and recovery

Disable a placement through its host form to retain options. Missing packages keep
removable placement records. Omit a connection/installation from deployment config
and restart its host to disable/retain it. Do not delete the database to disable.

For an update, build a new package, check it, back up SQLite consistently and update
both host package/manifest pins together. Restart hosts through normal deployment.
No in-place package editing or hot replacement. Pin mismatch fails locally while
core routes remain usable. Fix the pins/package and restart. API discovery retries
are bounded; restore API then restart BFF if startup retry was exhausted.

Migrations 10/11 are additive; down migration does not delete plugin records. Older
binaries may reject a newer migration version. Application rollback must use a
compatible build or an explicitly reviewed database-restore procedure. Keep an
online SQLite backup plus integrity check before deployment, preserving existing
service pins, host selections and plugin placements. Do not run destructive down
migrations as ordinary rollback.

The trust boundary is one shared dashboard audience and explicitly installed Go,
HTML and JavaScript. go-plugin AutoMTLS separates process RPC; it is not filesystem,
network or browser sandboxing. Signing, marketplace distribution, per-user access,
untrusted plugins and SDK compatibility upgrades remain later work.

## Explicit service icons

UI installations can grant `serviceIDs: ["<catalog service ID>"]` to plugins
that declare `catalog.services.read/v1`. IDs belong to the host's current
Staging catalog, including unpinned live services. Hidden, missing or purged
entries never resolve. Host-owned effective name and icon match Home, including
custom uploads and sprites. Catalog-aware widget HTML is rendered again on
each request so renames and icon changes do not stay behind observation caches.

Alertmanager uses annotation `balemoh_service_id` as the explicit service ID.
Add it to the Prometheus rule and grant that exact ID to the Alertmanager UI
installation. Use the intended source/resource identity in Staging, not a
name lookup. Missing, malformed, denied or unavailable associations leave the
alert text and severity unchanged.
