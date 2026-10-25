# Plugin acceptance ledger

Operator task is inspect current alerts and keep several independently filtered home widgets. Existing Balemoh console identity, Goshtoso panels/forms and compact lists remain the visual baseline. No added motion. Host controls own configuration; plugin controls own read-only alert views. HTMX changes the results region; native navigation remains available.

| State | Action | Result | Preserved | Effect |
| --- | --- | --- | --- | --- |
| New widget | Save | 201, then PRG | Form UUID and values | One placement |
| Repeated create | Same desired values | 200 | Placement identity | Still one |
| Existing UUID | Different create values | 409 | Current record | None |
| Current revision | Save options/selection | 200, ETag | Independent placements | One revision |
| Stale revision | Save | 412, reload/reconcile exact desired values only | Confirmed state and submitted values | None |
| Cross origin | POST | 403 | Configuration | None |
| Missing package | Disable/delete | Save or revision-checked delete | Other placements, source state | Configuration only |
| At shared capacity | Add host/widget | 409 | Existing selections | None |
| Collection failure | Read/refresh | Retained stale observation | Observation time/data | No new collector |
| Changed snapshot | Cursor | Conflict and refresh route | Filter form | No mixed page |
| Plugin blocked/crashed | Home/core/SSE | Immediate base response, local slot fallback | Wrappers, focus, stream | Bounded UI call |
| Native browser | Filter/detail/configure | Native GET or PRG | Connection, filters | Same backend operations |
| Contribution removed | Lifecycle | Cleanup listeners/subscriptions | Other mounts | One cleanup per mount |

Runtime checks use controlled source fixtures and real subprocesses. Browser evidence and live homelab checks are recorded separately from cross-compilation and unit checks.

## Local verification, 2026-10-25

- Shared `hack/check-plugin-modules.sh --allow-untracked --alertmanager=...` passed generation reconciliation, tests/vet for root/client/UI/SDK, the external consumer and controlled full-chain integration. Local generation uses the same protoc 33.0 and pinned Go generators as CI.
- SQLite/API suites cover replay, precise JSON numeric comparison, revisions, stale/missing preconditions and concurrent shared capacity. Existing service/host suites pass.
- Real child tests cover malformed/mismatched packages, scoped callbacks, output bounds, cancellation, restart/health exhaustion and recovery. Focused race checks cover runtime/author/UI and host persistence paths.
- Native document HTTP checks verify the asynchronous noscript iframe and top-level navigation. Browser native configuration forms saved independent placement options. Browser JavaScript was not disabled globally; this is not an exhaustive no-JavaScript browser matrix.
- Browser used actual BFF and independently built Alertmanager executables against a controlled 43-alert source. Current filter replaced only results; detail and return links retained search. Two persisted widgets had different filters/row limits and separate identifiers. Previous suspended-child checks preserved core navigation and SSE.
- Independent generated UI-only fixture loaded two lifecycle modules. Refreshing one slot kept two active mounts, proving cleanup/replacement did not accumulate mounts. Generated fullstack fixture showed fresh-empty and stale through the real shell after preview-state changes.
- Browser checked default Goshtoso and explicit minimal/dark appearance, desktop and mobile layouts. Viewport widths were inspected from DOM, with no horizontal overflow. Keyboard focus survived the filter fragment. Mobile screenshots inspected rather than inferred from HTTP responses.
- Non-root read-only image smoke loads an independently generated package through real API/BFF. Plugin artifact image compiles Linux amd64/arm64; native subprocess execution was macOS arm64 and local container architecture. Cross-compilation does not claim an exhaustive Linux architecture matrix.

Final review identities and deployment evidence remain pending. No SDK tag or source
release has been published. The user separately authorized deployment and a LAN-only
HTTPS Alertmanager route. Deployment requires a consistent pre-migration SQLite
backup, aligned package pins and one backend owner.
