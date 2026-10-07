# Host pages and native telemetry

Status: implemented locally, 2026-10-07. Production rollout pending.

## Outcome

Hosts is an inventory with links to dedicated pages. Docker hosts show machine CPU and memory plus containers. Kubernetes clusters show aggregate CPU and memory, node links and pods. Nodes have their own pages. Services remain in the existing catalog and link from their source's page.

## First release

- Independent telemetry loop, immediately at startup and every 60 seconds. Each collection has a 30 second deadline.
- Docker agents read an explicitly mounted host proc filesystem. CPU is the busy fraction of aggregate CPU counter deltas, excluding idle and iowait. Guest counters are not counted twice. The first reading and counter resets produce missing CPU. Memory is MemTotal minus MemAvailable. Container inventory comes from the existing Docker-compatible API, including stopped containers.
- Kubernetes reads Nodes and Pods plus the Node Metrics API. CPU percentage uses node CPU capacity; memory uses node capacity. Missing or expired metrics stay missing. Cluster totals exist only when every listed node has the corresponding measurement. Pod discovery respects the configured namespace; node metrics describe the whole machine.
- Telemetry is opt-in through BALEMOH_TELEMETRY_ENABLED, default false. BALEMOH_HOST_PROC_PATH defaults to /host/proc. The collector does not fall back to its container's proc filesystem.
- Registered sources appear before their first observation, even with no services. Existing catalog sources also remain visible when telemetry is disabled.

## Contract and invariants

OpenAPI in api/openapi.yaml is authoritative. GET /api/v1/hosts lists roots; GET /api/v1/hosts/{hostId}?range=1h|24h returns current data, child nodes and samples; POST /api/v1/federation/telemetry ingests one source's batch. The default range is 1h.

A batch contains source, observedAt and hosts. An empty node name identifies the source root. A nonempty node name identifies a Kubernetes node. IDs and parent relationships are derived by the gateway from source and node name, never accepted from an agent. Unknown source kinds remain inventory-only.

Bearer credentials are bound to the exact registered source using existing federation authentication. Telemetry never mutates discovery snapshots, service overrides, pins or visibility. The ingestion body is limited to 2 MiB, 128 hosts per source, 1,000 workloads per host, and bounded strings. Values must be finite and nonnegative; CPU is at most 100 percent and used memory cannot exceed capacity. Root is required, node names are unique, and Docker batches have one host.

Observation time must be within the past 24 hours and at most 30 seconds ahead of the receiver. Duplicate or out-of-order source batches are acknowledged without updating heartbeat, inventory or samples. Each accepted batch updates its hosts atomically. A missing node is retained with its last observation and becomes stale. Host identity survives restarts and display changes.

SQLite stores latest observations separately from one sample per UTC minute per host. A newer reading replaces that minute's sample. History expires after 24 hours; pruning runs on ingestion and every minute even without agents. Nodes unobserved for 24 hours are removed, leaving bounded inventory. Root registrations remain. Reads filter expired samples even before pruning.

Heartbeat uses server receipt time; measurement freshness uses observation time. No receipt means unavailable. Receipt or observation older than three minutes means stale, not proven offline. Missing CPU or memory means partial data. Zero is a valid reading. Missing minutes are explicit gaps, never interpolated or replaced with zero. Historic values remain visible when the latest report is stale.

## Page design

Audience: homelab operator locating a busy or unresponsive machine. First screen answers which host, whether data is recent, and CPU/RAM use. Existing console shell, avatars, page headers, cards, badges, tables and links come from Goshtoso. Goshtoso Charts v0.0.3 renders labelled SVG lines with explicit missing points. Host charts omit captions and hide exact-value disclosures. No custom chart renderer or client-side chart state.

/hosts is a compact linked inventory. /hosts/{id} contains breadcrumb, host identity, freshness and last report, two metric summaries, CPU/RAM history with 1h/24h links, child nodes when applicable, workloads and related catalog services. Charts use percentages on fixed 0–100 axes and UTC time labels. Both ranges retain one-minute resolution. Range links and Refresh use HTMX partial GETs that morph the host detail section, preserve scrolling and dismissed notices, and update browser history. Without JavaScript they remain normal GET links. One same-origin SSE connection updates measurements, status, CPU/RAM history, nodes and workloads without page reloads. The BFF checks the API every five seconds and sends a full HTML snapshot only when host data changes. Unchanged checks send keepalive comments. Agent collection remains once per minute. Catalog links update on navigation or partial Refresh, which preserves the selected range.

At 390px, summaries and charts stack. At 1440px they use two columns. Long names wrap; tables scroll within their container. Empty, unavailable, partial and stale states contain text, not color alone. Keyboard links, visible focus and labelled charts are required. Metadata retains the existing canonical origin and social image; range queries do not change the canonical path.

## Operations and limits

Docker deployment needs a read-only /proc:/host/proc bind mount from the same Linux host as the Docker socket. Telemetry requires a local Unix Docker socket; remote Docker endpoints need an agent on that remote host. Kubernetes RBAC needs list on nodes and pods plus list on metrics.k8s.io/nodes. Metrics Server is an optional source: its absence must not prevent heartbeat or inventory.

Upgrade API and UI before enabling agents. Existing agents remain compatible because telemetry has a separate endpoint. Disabling collection stops writes but keeps inventory and history until expiration. Rollback must stop the new binaries before dropping migration 000008; doing so deletes telemetry only.

No process lists, logs, shell access, alert delivery, disk/network metrics, per-container resource accounting, long-term retention or Prometheus dependency in this release. No production rollout is implied by implementation.

## Acceptance

Automated checks cover validation, exact-source authentication, replay handling, transactional writes, minute replacement, retention, null versus zero, proc counter resets, missing Kubernetes metrics and chart gaps. UI checks cover links, range validation, unknown hosts, empty/error/stale states and server-rendered metadata. Browser checks cover narrow/wide layouts, both themes, light/dark, keyboard navigation and accessibility. Production telemetry remains unverified until separately authorized deployment.

## Implementation evidence

The implementation uses migration 000008, a separate telemetry model/store, the existing federation transport, generated OpenAPI handlers/client and Goshtoso Charts v0.0.3. Operator setup is in [host-telemetry.md](../../host-telemetry.md).

Local checks cover API and UI tests, race detector, vet, generated-file stability, federation ingestion into a real SQLite database and browser navigation through source and node pages. The browser matrix uses synthetic observations, including missing minutes. Widths 390 and 1440, Goshtoso and Minimal, light and dark all retain readable content without horizontal page overflow. Keyboard navigation, range selection, chart expansion/Escape and workload scrolling are checked.

One known upstream accessibility issue remains: Charts emits `figure role="img"`, which axe flags as a minor `aria-allowed-role` warning. Exact-value tables remain available. See [Goshtoso integration notes](../../goshtoso-snag-journal.md). Production collection and rollout are not part of this validation.


## Live chart delivery

`GET /hosts/{id}/events?range=1h|24h` returns `text/event-stream`, disables proxy buffering and caching, and sends an immediate snapshot. Goshtoso's bundled HTMX 4 SSE extension applies unnamed HTML events using `innerMorph` out-of-band swaps. Stable application IDs isolate each chart from its public Goshtoso chart-control wrapper. Expanded charts continue receiving data. Caption and exact-value disclosure UI is omitted.

Charts remain SVG because Charts v0.0.3 interactive `LiveData` snapshots accept only finite numeric values. Missing telemetry must remain a gap, not a zero or interpolated point. No custom chart renderer or new dependency is needed.

Each API fetch has the existing request timeout. Stream writes have deadlines, disconnection cancels polling, and UI shutdown cancels active streams. Failed API reads after connection leave the last measurements visible and show retry status; the next successful read sends a complete snapshot. Initial unknown hosts return 404, invalid ranges return 400, and initial upstream failure returns 503 with a Refresh recovery message. HTMX reconnects interrupted streams with backoff and receives a full snapshot, so event replay is unnecessary. Background tabs pause through the bundled extension and reconnect on return.

### Live update acceptance

| Event | Required result |
| --- | --- |
| New sample | Summary and both charts update without navigation; nulls remain gaps |
| New workload or node state | Inventory updates on the same stream |
| Repeated snapshot | Keepalive only, no chart replacement |
| API failure after connection | Last data stays visible, retry message appears, recovery sends a full snapshot |
| Network interruption | Reconnect with backoff, current range preserved, no duplicate stream |
| Expanded chart | Update data without closing the modal |
| Range navigation or leaving page | Old stream closes; new range has one connection |
| Hidden tab or shutdown | Pause or cancel active work |

Verified locally with `GOWORK=off go test -race ./...` and `go vet ./...` in `ui`. The streaming test covers initial snapshots, unchanged keepalives, upstream failure/recovery, reconnect snapshots, cancellation, range forwarding and HTTP errors. Chrome checks inject a new sample into the synthetic preview, interrupt the stream through a local proxy, and verify data retention, one reconnected stream, 24h updates, expanded charts, disclosure state, background pause/resume and navigation cleanup. The eight theme/viewport checks include the main content's scroll width, since the shell can hide document overflow. No JavaScript exceptions or serious accessibility findings; the existing Charts figure-role warning remains.

Host type and telemetry status use a Goshtoso badge. The main scroll container establishes the containing block for absolute descendants so table captions and hidden chart controls cannot create a second document scrollbar.

Partial request failures update the history error message without replacing measurements or changing the selected URL. Rapid history actions replace the pending request. A range change closes the old SSE connection and initializes one for the selected range; messages queued for a superseded range cannot update charts. Full history restores still receive the complete page.
