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

## Independent review and CI

Consumer package version is `0.0.1-alpha`; generated independent fixture version is
`0.0.1`. Both use protocol `balemoh.plugins/v1alpha1` and UI profile
`balemoh.goshtoso/v1alpha1`, with Go 1.27 module contracts, Goshtoso v0.3.3 and
templ v0.3.1070. SDK module remains `github.com/araihu/balemoh/plugin-sdk` v0.0.0
with the documented explicit bootstrap replacement, not a published stable tag.

Two independent reviewers accepted architecture and code quality at the final source
checkpoint. Main review identity was
`817c36cc8ab3a2647dd10fd9d6c9444c2aec1c4d72f2faec318b697c809ff199`;
external consumer identity was
`25d99b4339111b45e87ccc58c7749faa5cccc15f44e64f92ceff5b3af69deed2`.
Main source commit `6c71e41` and merge `033af189` have identical trees. Consumer
commit `2b1e9e5d2b8f0d05bfd8aa7e550a5b17039dcd65` adds only a trailing-space
cleanup to its accepted bytes, acknowledged by review. Both reviewers also accepted
the three portable smoke assertions in `78c4029`, identity
`c33dacbbd83fd5be519ddd207edc22895f5127336a535dbf982fed425a21cdba`.

- [Host CI at 78c4029](https://github.com/araihu/balemoh/actions/runs/40191143681)
  passed shared module/generation checks, builds, all module race suites and Linux
  non-root read-only image smoke. CodeQL checks on the source PR passed.
- [Independent consumer CI](https://github.com/araihu/balemoh-plugin-alertmanager/actions/runs/40190684037)
  passed pinned-host SDK/consumer integration, generated drift checks and consumer
  race suites. `BALEMOH_SDK_REF` is explicitly pinned to `033af1892b4c46cd901bf44468fe44a655b0da41`.
- Source integration is tracked in [PR 42](https://github.com/araihu/balemoh/pull/42).
  No SDK tag or source release has been published. Source merge and stable SDK
  distribution remain separate actions.

## Authorized homelab verification, 2026-10-25

Both reviewers accepted deployment identity
`837e52b3fda8921bec3a590cff5090e195c7a18dfdd364015cba8012442bdfee`,
based on `c6bd09532c0a45c5f5b6f58beedf91e3a385d11f`. Review corrections made
the Balemoh validator independent of init-container order and documented that
trusted plugin children can read host-mounted credentials. RPC scoping is not a
filesystem sandbox.

[Homelab PR 573](https://github.com/guilycst/home-lab/pull/573) deployed commit
`3a5b6c3d336b8bf2231d3145e3305eb04865e1e0`. Balemoh, Envoy and Prometheus
Argo applications reached Synced/Healthy at that Git revision. API/UI reported the
same reviewed host image and plugin artifact digests, ready with zero restarts.
The cluster executed the real plugin on Linux amd64; local container checks also
executed the packaged Linux arm64 child.

- A private consistent SQLite backup passed integrity and restored migration 9 to
  11 before rollout. The live post-rollout backup passed integrity at version 11;
  hashes of all three existing host selections and all 41 service edits matched.
- Real Alertmanager connection reached ready/fresh. At 12:10 UTC, its complete
  snapshot contained six alerts: zero critical, one warning, two info and three
  unknown. Counts are point-in-time evidence, not expected permanent values.
- Native BFF forms created and enabled two persistent placements: complete summary
  and active critical/warning list limited to five rows. Private API confirmed their
  independent options and stable placement IDs. Existing Home/SSE remained connected.
- Live browser verified filtering, details, filter-preserving return URLs and the
  HTTPS Alertmanager link. Both widgets loaded within their async Home slots at a
  1280px viewport without horizontal overflow. Mobile evidence above remains local
  fixture evidence, not a separate live mobile run.
- `https://alertmanager.decastro.me/` returned 200 with certificate validation.
  Certificate generation 10 became Ready. Route Accepted/ResolvedRefs attaches only
  to internal gateway HTTPS; Pi-hole and system resolver returned `10.0.0.130`.
  Public resolver queries for A and AAAA returned NXDOMAIN.
- Private API without its service credential returned 401; the same path through
  browser ingress returned 404. No alerts or silences were changed.
- Operator-owned diagnostic pods and local fixture processes were removed after
  use. Private database backups remain outside Git. Ordinary image-only rollback
  across migration 11 remains unsupported; follow the documented restore procedure.

## Native Alertmanager filters, 2026-10-25

Visual profile `balemoh.goshtoso/v1alpha2` replaces the earlier profile recorded
above. It pins Goshtoso v0.4.3 in host assets and generated consumers. The RPC
protocol remains `balemoh.plugins/v1alpha1`; upgrade host and plugin packages
together. Old visual profiles are rejected. The Goshtoso tag points to
`4c46fa5be66c2c302fb3028e8e57aafbab23c217` and resolves with the public Go checksum
`h1:rOGM2LFm2d8w7w5WjcELLqf4JgeMHXUdH/WFDaZBfLU=`.

Alertmanager uses the native Goshtoso multi-select combobox. Search matches
case-insensitive terms across retained public alert fields. Changes replace only
results; the form, search draft, focus and open filters survive. Native iframe
requests receive exclusive checkbox controls through `__native=1`, avoiding
hidden fallback values in JavaScript fragments.

After resolving the real tag, the normal multi-module gate passed generation,
module tidy, tests and vet for root/client/UI/SDK, generated standalone UI/backend
consumers and the external Alertmanager consumer. Controlled API/BFF integration
also passed. The SDK remains unpublished; consumer CI still requires its explicit
reviewed host commit.

## Compact alert results, 2026-10-25

Goshtoso v0.4.4 adds non-interactive trailing option content and cancels pending
keyboard opens when Escape is pressed. The UI host and generated SDK consumers
pin this patch so the host serves the fixed runtime used by the Alertmanager
filter controls. CSS remains byte-identical; visual profile v1alpha2 and the RPC
contract remain compatible. Deploy the host and plugin package pins together.


## Notification overlay scroll containment

With a long tray containing both read and unread messages, scroll the main page
before opening the bell. Opening, refreshing and closing the popover must preserve
the document width/height and the main container's bounds and scroll position.
Only the tray may gain its own scrollbar; it must not create a document scrollbar
outside the console shell. Scroll the tray and verify that the main page stays put.
Repeat at desktop/mobile widths and in light/dark modes. Check that the header
menu remains visible, Escape returns focus to the bell, and the unread filter
keeps its state. Do not mark or clear production messages for this test.
