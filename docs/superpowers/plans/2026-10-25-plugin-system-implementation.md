# Application plugins implementation plan

Status: P1 through P8 implemented and independently reviewed. Authorized homelab rollout passed with real Alertmanager data and persisted widgets. Checked tasks record verified behavior; source remains a draft PR and no stable SDK tag exists. User separately authorized real Alertmanager access and homelab deployment on 2026-10-25.

**Spec:** [Application plugins](../specs/2026-10-25-plugin-system-design.md).

**Goal:** Configure a trusted Alertmanager plugin, navigate current/detail pages, and persist several independently configured home widgets without plugin-specific core imports or renderers. Deliver a public SDK and working scaffold that another author can use for a different plugin.

**Maturity:** pre-v0.1.0 exploration, as defined by the spec. First achieve a local runnable UI slice, then add real source data, persistent placements, author tooling and the remaining operational contract. Required safety applies whenever a capability becomes reachable; later stages do not excuse exposing unbounded calls or credentials earlier.

## Execution rules

- Use the spec as contract authority. Update it explicitly before changing public behavior; do not quietly implement a different protocol through a convenience endpoint.
- Work in the existing task worktree, preserving the specification and unrelated changes. Recheck current HEAD and migration sequence before implementation. No assumption that migration 10 is still free.
- Each stage ends with runnable behavior, focused verification and a concise evidence receipt. Check boxes only after verification passes. A stage is not a new permission boundary; follow the authority actually granted for implementation.
- Keep existing service pins, host selections, catalog lifecycle, federation and core health intact. New installations are opt-in and disabled by default.
- Keep API/OpenAPI data-only. UI HTML crosses only private BFF/plugin RPC and BFF responses. Never add Alertmanager-specific branches to core routing, application or renderer code.
- Generate protobuf, OpenAPI client/server, sqlc and templ output from their sources; never edit generated files by hand.
- Run independent modules with `GOWORK=off`. Explicit sibling module replacements are allowed during repository development. Never use a hidden go.work dependency to claim an independent consumer builds.
- Apply `managing-go-environment` when configuring host environment: typed caarlos0/env/v11 parsing, envdoc output, semantic validation and injected environment tests. Parse once before DB/process startup. Installation/connection files have a separate typed schema; no ad hoc environment parsing inside plugin handlers.
- Tests exercise contracts, state changes, real child processes and browser behavior. Avoid tests that only duplicate constants or implementation layout. Use fake clocks/source HTTP fixtures where they make failure tests deterministic.
- Runtime budgets, local path checks, output limits, source-secret separation and useful errors enter with their corresponding reachable feature. Do not defer these to a final polish stage.

## Delivery sequence

| Stage | User-visible result | Depends on |
| --- | --- | --- |
| P1 | Independently built fixture plugin adds a sidebar page and home widget inside real BFF shell | Existing spec |
| P2 | First product plugin reads Alertmanager through backend/API/scoped UI callback | P1 |
| P3 | User creates, configures and selects several persistent plugin widgets | P2 |
| P4 | Author generates a separate plugin project and previews it with the real shell | P1 to P3 |
| P5 | Alertmanager current/detail/filter/pagination and widget flows are complete | P2 to P4 |
| P6 | Runtime fulfills supervision, stale recovery, frontend lifecycle and bounded composition | Basic mechanisms from P1 onward; final verification after P5 |
| P7 | CLI validates/packages plugins and CI/image checks cover independent modules | P4 to P6 |
| P8 | Full acceptance evidence and implementation documentation are ready | P1 to P7 |

Stages are deliberately vertical. A small UI fixture precedes the full schema/tooling inventory. Alertmanager is the first product plugin; the fixture is a test consumer, not another supported integration.

## Intended file ownership

Paths below are planned ownership, not files already present.

| Surface | Files/modules |
| --- | --- |
| Public SDK | `plugin-sdk/go.mod`, `manifest/`, `ui/`, `backend/`, `host/`, `render/`, `runtime/`, `protocol/v1alpha1/`, `testing/` |
| Core plugin use cases | `internal/application/plugins/`; extend `internal/application/dashboard/` only for generic references |
| Host adapters | `internal/adapters/plugins/`, `internal/adapters/http/plugins.go`, `internal/adapters/sqlite/plugins.go` |
| Persistence | Next additive migration pair; `internal/storage/queries/plugins.sql`; generated sqlc output |
| Data contract | `api/openapi.yaml`, `internal/api/generated/`, `client/balemoh.gen.go` |
| BFF integration | `ui/internal/plugins/`, `ui/internal/bff/plugins.go`, client/dashboard/events composition, view shell/pages/widget slots |
| Startup | `cmd/balemoh/main.go`, `ui/cmd/balemoh-ui/main.go`, both typed config packages |
| Product plugin | Separate repository `araihu/balemoh-plugin-alertmanager`, initially UI-only in P1; backend added in P2 |
| Test consumer | SDK `runtime/testdata/ui` for generic conformance; separate scaffold-generated consumers in P4/P7 |
| Author tooling | `plugin-sdk/cmd/balemoh-plugin/`, scaffold templates and harness support |
| Verification/build | `.github/workflows/ci.yml`, `Dockerfile`, existing image smoke and a focused `hack/check-plugin-modules.sh` |

Alertmanager source/normalization/views stay in its repository. Core owns only generic protocol, source configuration, placements and dispatch. Consumers import the public SDK, never host internals.

## P1. First executable UI slice

**Deliverable:** real BFF shell displays a page and widget from a separately built trusted executable. No source credentials or live Alertmanager access yet.

- [x] Define SDK manifest/identity types, exact alpha protocol/profile IDs and first closed manifest validator. Include valid/invalid fixture manifests and local schema/path/route/capability checks exercised by this slice. Materialize the spec's remaining fields without advertising unsupported runtime capabilities.
- [x] Add protobuf definitions/generated stubs for Control and UI envelopes plus the scoped callback envelope needed by subsequent stages. Publish no release tag. Set deterministic generation tooling in SDK module.
- [x] Implement go-plugin/gRPC UI Serve/client adapters, bounded templ-to-HTML rendering, Describe comparison, startup/call deadlines and child termination. Keep process transport out of core application packages.
- [x] Create separate `balemoh-plugin-alertmanager` repository as independent consumer of public SDK, as requested during P1. It provides one page, one widget, a valid visual profile and fixed synthetic data. It imports no host internal packages. Generic process fixtures remain in SDK tests; the real Alertmanager collector stays in P2.
- [x] Add generic BFF mount, page-content slot, navigation merge and generic widget renderer. Retain host wrappers; provide scoped URL/DOM helpers with base-path support. No per-plugin switch or import.
- [x] Run BFF with an injected development descriptor/placement provider and loopback fixture API using the existing data-only dashboard envelope, clearly labelled synthetic. This avoids depending on P2's not-yet-generated plugin API methods. Fixture composition may supply a selected placement for preview; ordinary installation still does not auto-pin. No unauthenticated production plugin endpoint is introduced.
- [x] Demonstrate full page, direct link, one widget and invalid-profile fallback. Kill child and verify local unavailable state while Home/Staging/Hosts still work. Verify startup/shutdown leaves no child owned by the test harness.

**Verification:** focused SDK process/manifest tests; BFF route/view tests; local browser page/widget with fixture data. Build the consumer with GOWORK disabled and an explicit local SDK replacement in its isolated test module.

**Exit:** executable plugin UI works through generic host mounts. Record limitations of this first slice; only completed capabilities are advertised.

### P1 local evidence, 2026-10-25

- SDK exposes the P1 manifest, UI, rendering, process and protobuf contracts. Real-child tests verify matching Describe, source-query denial, no inherited host secret, deadlines, bounded HTML and process cleanup. Package tests reject external schema references, symlink escapes and incompatible profiles.
- The independent consumer is the local Git repository `balemoh-plugin-alertmanager`. `make test SDK_DIR=<Balemoh checkout>/plugin-sdk` generates views, builds a current-platform package, tests placement identity and runs vet. The public SDK dependency is unpublished `v0.0.0`; only ignored development modfiles contain a local replacement. No remote repository, tag or code publication was created.
- BFF preview configuration is disabled by default. Explicit synthetic connection/placement UUIDs mount a sidebar page and two home widgets through generic paths. No API/OpenAPI, database migration, real source access or service pin mutation was added.
- Passed SDK and UI full module tests/vet, SDK race tests, focused BFF/config race tests, consumer build/test/vet, Docker image build and existing API/UI image smoke. The first smoke attempt received an empty reply during UI startup; the second passed without a code change to the smoke script. Protobuf, templ and envdoc output were generated from source.
- Browser at 1280 px verified widget links, direct page navigation, sidebar entry, Home/Staging/Hosts and local unavailable state after killing the real child. The existing single SSE stream updated both widgets without reloading Home. Startup/shutdown and owned-child termination were checked locally. Mobile/theme/keyboard/scroll acceptance remains in P4/P5.
- P1 limits are explicit: UI-only GET content, one preview connection per plugin, no Host query grants, persistent placements, custom assets, backend collector, automatic restart or scaffold CLI. P2 starts in the separate Alertmanager repository; P3/P4/P6 complete the remaining host behavior.

### P1 independent review, 2026-10-25

Two independent reviewers inspected architecture and code quality without producer history. Three current blockers were reproduced with temporary Go overlays; reviewed source files stayed unchanged during reproduction:

- [x] Enforce extension-point declarations. The original defect mounted pages/sidebar/widgets without declarations. Manifest validation now rejects that state; runtime resolves only declared, negotiated capabilities. Negative tests cover each missing extension point and an unavailable optional page.
- [x] Omit incompatible installations before mounting contributions. The original defect retained mounts after required-capability rejection. An explicit compatible state now gates page/sidebar/widget lookup, and tests verify rejected installations never start children or expose contributions. A compatible child that later fails retains local placeholders.
- [x] Remove plugin RPC from base rendering entirely, as required after review. Home, page shells and SSE render host-owned loading slots without invoking children. Each slot uses an independent bounded content request. The old synchronous reproduction took 10.010 seconds with a 50 ms BFF budget and two slow placements; the revised negative tests assert zero plugin calls from base responses/SSE and isolated timeout/retry behavior. P1 page metadata is now declared in the manifest.

Non-blocking follow-ups retain their existing activation stages: explicit presence of secret flags before P2 secrets, scaffold/consumer executable conformance in P4/P7, and pinned protobuf generation with CI drift checks in P7. No P0 was found. Existing green suites did not detect the three negative cases above. Corrections passed focused/full module checks and independent architecture/code re-review. Both reviewers found no remaining current functional blocker. Automated browser DOM-preservation regression coverage remains a P3/P5/P7 task; it is not a current defect. P1 plugin content requires shared HTMX; no-JavaScript plugin behavior remains a P5 task.

### Non-blocking rendering verification

The revised SDK/UI suites, vet, affected race checks and external consumer build/test/vet passed. The new Docker image built and passed the existing API/UI smoke. Generated templ/envdoc output reconciles without changes.

Browser verification suspended the actual UI child: the page shell rendered its loading slot immediately, Staging stayed usable, and timeout swapped only the slot to a visible retry state. Resuming the child and retrying restored plugin content without shell navigation. Adding/removing a synthetic service widget through native Staging controls triggered structural dashboard SSE updates while both loaded plugin slots and keyboard focus were preserved. Only one shared HTMX 4 runtime was loaded.

Review source bytes remained unchanged during re-review. One reviewer accidentally created an untracked `ui/balemoh-ui` build artifact and removed it; source hashes and the final complete snapshot matched the frozen baseline. Independent reviewers completed their final analyses against the restored source state. No tag, remote publication, push or deployment was performed.

## P2. Backend SDK, data API and first Alertmanager connection

**Deliverable:** Alertmanager becomes the first product plugin. Its current observations reach UI through the API and scoped host callback.

- [x] Complete backend SDK Configure/Status/Query/CloseConnection adapters and typed state/error envelopes. Propagate parent deadline/cancellation. Add scoped broker callback for UI queries; never pass the BFF API credential or source secret to UI child.
- [x] Implement typed installation/connection deployment configuration, package/manifest digest checks and declared secret references. Default disabled; missing credential/grant/package fails explicitly. Test absent/empty/malformed/semantic-invalid configuration without printing values. Regenerate envdoc when environment fields are added.
- [x] Add connection application/store ports and the next additive migration with revision semantics. Define initial provisioning reconciliation: persist/reuse the host-issued connection UUID, same validated configuration is no-op, changed configuration advances revision, omission disables/retains records instead of deleting them. Explicit deletion remains blocked while placements exist.
- [x] Extend OpenAPI with authenticated plugin descriptors and query route; generate server/client before BFF calls them. Add installation-private BFF service credential verification, per-plugin/connection grants and input/output schema validation on server. Keep legacy API behavior unchanged.
- [x] Compose one backend child per plugin capability in API host, configure enabled connections, and maintain bounded status metadata cache. Configure starts one immediate asynchronous collection, independent of page requests.
- [x] Extend the separate `balemoh-plugin-alertmanager` repository with backend entrypoint and connection/list/get/widget schemas. Implement read-only API v2 collector, server API URL versus browser URL, optional backend-only Bearer token, verified TLS and explicit trusted-network HTTP option.
- [x] Implement complete-response replacement with 5 MiB/5,000-alert limits; retained snapshots, source timestamps/state, stable severity/name/fingerprint order, snapshot-bound cursors and full filtered counts. No source mutation or history capability.
- [x] Wire a minimal current page and summary through UI HostServices.Query, generated client, core API and backend Query. Render reads cached source observations. Three tabs must not start three collectors.

**Verification:** real local child RPC plus httptest source fixtures; configured token absent from descriptors/UI/error/log fixtures; 401/403 direct query tests; fresh-empty/stale/invalid/truncated/oversized source responses; Configure idempotency and old state on failure; query reads do not fetch source.

**Exit:** real Alertmanager protocol works against a controlled local source fixture and an explicitly configured developer source when available. A live source is optional evidence, never an implicit homelab probe or deployment prerequisite.

### P2 implementation evidence

The API owns data-only connection ports, migration 000010, pinned deployment
loading, per-connection query grants and one backend child per plugin. Configure
starts asynchronous collection. BFF owns a separate UI child per plugin and
invocation-scoped host callbacks. API/BFF service credentials never enter UI RPC.

The external `balemoh-plugin-alertmanager` module implements Alertmanager API v2
collection, list/get DTOs and schemas, retained complete observations and minimal
current/summary views. It remains a local unborn repository with no remote or
published version. Its README documents exact package pins and separate host
configuration. `make integration` accepts explicit matching API/BFF binaries.

Controlled real-host integration exercises two connections, source Bearer auth,
private API auth, denied direct queries, cached reads across repeated page/widget
requests, immediate shell/Home/SSE and stale retention after source failure.
Unit/process tests cover malformed/truncated/oversized/duplicate source data,
fresh empty, Configure replay/rejection, cursor revision binding, TLS/redirects,
scoped callbacks, configuration omission/restart and restricted deletion.

P2 deliberately retains explicit `previewWidgets` references. Persistent placement
controls remain P3. Filter/detail/pagination controls and the second Alertmanager
widget remain P5; backend list/get operations already implement their data
contracts. Automatic supervision/restart remains P6. API discovery failure at
BFF startup records a local plugin problem; restart BFF after recovery.

P2 review corrections cover asynchronous local UI startup/pin failure, named
connection navigation, atomic cancellation-aware Configure, revisioned stale/error
transitions and shared credential syntax. Descriptors distinguish accepted from
desired configuration and retain source status plus separate Configure errors.
Migration 000010 supports data-preserving down/up replay. Regression tests include
a hung UI startup, semantic rejection in the real Alertmanager child and a bad
optional BFF pin.

Verification includes root/SDK/UI/consumer Go tests and vet, focused race runs,
generated-source checks, a local Docker image build/smoke test and native browser
checks. The real-host integration uses explicit matching API/BFF binaries and a
controlled local source, without homelab access. It also provisions invalid API
package and manifest pins alongside the valid installation, checking health,
legacy routes, valid data and sanitized catalog problems.

Two independent Git-based reviews cover architecture and code quality. Accepted
source identities are reported in the execution handoff. Generic Configure
payload/secret aggregate bounds remain a v0.1.0-candidate hardening item; the
current Alertmanager configuration is below both limits. No push, release,
install into the homelab or deployment is implied.

## P3. Persistent placements and dashboard integration

**Deliverable:** create two differently filtered widgets from one connection, select/unselect them, restart hosts and retain choices.

- [x] Implement independent placement model/options validation and store. Add next additive migration, queries and generated sqlc output. Preserve existing service pins/host selections and avoid destructive down migrations during rollback.
- [x] Enforce shared 128 persisted non-service selection budget in the same SQLite transaction across existing host-selection and new placement writes. Verify replay before limit checks and handle concurrent host/placement additions without oversubscription.
- [x] Add OpenAPI list/PUT/DELETE placement routes with exact create replay, semantic JSON comparison, If-Match, new ETag, revision conflicts and delete semantics from spec. Regenerate client/server.
- [x] Add generic dashboard provider using cached references/revisions, not remote source calls in provider loop. Emit plugin-widget/plugin-ui references and preserve old-UI unknown-renderer fallback.
- [x] Build host-owned definition/placement list, option/create controls and selection/options/delete forms. Support the spec's schema subset, explicit enabled state, form UUID retry identity, CSRF/same-origin checks and revision conversion to If-Match. Native forms use PRG; errors preserve input/focus.
- [x] Resolve placements in generic BFF renderer; UI callbacks read only current placement/connection. Include placement identity in every DOM/tooltip/HTMX target and cache key.
- [x] Integrate changed plugin references/revisions into the existing single dashboard SSE stream; refresh content only through isolated slot requests. Preserve host wrappers and masonry geometry. Structural reconciliation is limited to membership/order changes.
- [x] Verify disable/missing package/source failure retains removable configuration and does not erase source data or unrelated selections.

**Verification:** SQLite create/replay/restart/concurrency/revision tests; HTTP 201/200/204/404/409/412/428 contracts; browser two placements, repeated checkbox actions, no-JavaScript forms and nonzero scroll/focus checks.

**Exit:** configuration is durable and independently addressable. No second copy of catalog pin state exists.

## P4. Scaffold and real-shell development workflow

**Deliverable:** another author can generate an independent project from the working recipe and preview it inside the real BFF.

- [x] Implement `balemoh-plugin init --id ... --backend`, templates and author README from P1/P2 module structure. New/empty output only; no overwrite. Emit manifest, schemas, pinned SDK/profile dependencies, author-owned templ generation, UI/backend entrypoints and fixture tests. UI-only mode omits backend/query declarations.
- [x] Keep SDK/profile version selection explicit. Before an SDK tag exists, out-of-tree CI consumer tests may add a temporary local module replace to the generated test copy. Do not ship developer absolute paths or silently depend on go.work. Actual public tag publication is a separate release action, not required to develop this stage.
- [x] Implement `dev --host` compatibility check and loopback fixture API. Launch actual BFF executable with isolated temporary configuration/data/ports. Do not import BFF internals, duplicate shell markup, reuse production registry or access real sources by default.
- [x] Build a temporary manifest/target for current GOOS/GOARCH. Rebuild/restart the isolated harness on author changes and clean up owned processes/ports. This does not add production hot replacement.
- [x] Provide preview routes for loading/empty/fresh/stale/error, direct page links and two copies of same widget. Add generic declared-asset dispatch and initial SDK mount/unmount integration needed by a small JS lifecycle fixture, without another HTMX/Alpine runtime. P5 exercises the complete visual/profile behavior in Alertmanager.
- [x] Generate and build an out-of-tree UI-only plugin and full-stack plugin. Run both with real host and prove neither imports API/UI internals.
- [x] Compare generated full-stack layout with Alertmanager module. Move repeated host mechanics into SDK, without wrapping/copying the entire Goshtoso library or moving Alertmanager business logic into templates.

**Verification:** CLI temp-directory tests for invalid ID/nonempty output; generated module build/test under GOWORK=off; harness loopback/fixture/cleanup checks; browser preview in real shell.

**Exit:** scaffold produces a working independent consumer, not empty placeholders. Record exact local bootstrap steps while SDK is unpublished.

## P5. Complete Alertmanager pages and widget experience

**Deliverable:** current list, detail, filters, pagination, source navigation and both home widgets behave as one plugin application.

- [x] Finish current page with full filtered counts, state/severity filters, bounded search, snapshot-bound pagination, observation age and local refresh. Changed snapshot/cursor presents recoverable refresh, not silently mixed pages.
- [x] Implement detail route keyed by connection/fingerprint; direct navigation and native forms work. Escape source labels/annotations; validate source/runbook HTTP(S) URLs without embedded credentials. Do not infer host identity from a label.
- [x] Complete summary and active-list views, independent options, bounded rows and links preserving selected filters/connection. No source credentials or in-cluster-only API URL in browser navigation.
- [x] Implement local fragment handlers through SDK and typed responses; coordinate table/count updates within one contribution. Preserve filters, focus, scroll and open controls. Do not replace whole grid/shell for a value update.
- [x] Finish declared asset serving and profile dependency loading in BFF. Package-relative/digest URLs, deduplication, scoped CSS and supported Goshtoso dependencies; validate code generated by pinned templ/profile.
- [x] Implement/test frontend mount/unmount hooks using supported HTMX lifecycle behavior. Repeated refresh/navigation/removal must not accumulate timers/listeners/subscriptions or load another runtime.
- [x] Keep history/source mutations/reserved extension points absent from manifest/navigation. Include configuration and stale/empty explanations sufficient to avoid confusing outage with zero alarms.

**Verification:** source/handler fixtures plus browser current/detail/filter/cursor/two-widget paths at 390 and 1440 px, supported themes/color modes, keyboard and native fallback. Check repeated operations after scrolling; HTTP success alone is insufficient.

**Exit:** first product plugin exercises pages, navigation, data callbacks, independent widgets and source links without Alertmanager-specific host code.

## P6. Runtime operational contract and recovery

**Deliverable:** complete spec's process supervision and failure behavior. Basic limits/secrets/cleanup already introduced with P1/P2 remain mandatory throughout.

- [x] Complete enable/disable/validation/start/ready/backoff/failure/stopping state machine in shared runtime adapters. Handle disable during startup/backoff and host shutdown without scheduling another restart.
- [x] Enforce spec restart delays/budget/reset, health-check cadence/deadline and three-failure policy. Source stale/unavailable does not fail child readiness; ordinary query timeout does not trigger restart storm.
- [x] Enforce independent UI/backend concurrency and early payload bounds, including callbacks. No self-deadlock or unbounded admission queue. Concurrent dashboard rendering shares originating deadline rather than serial per-widget timeouts.
- [x] Complete retained HTML/reference/status caches with package/configuration/placement/observation/display-context keys. Error/recovery never rewrites observation time or removes unrelated widgets. Verify source and process failure are visually distinct.
- [x] Add structured state/duration/restart/source-freshness diagnostics and request correlation with bounded metric labels. Validate redacted fixture logs without printing real credentials/raw upstream bodies.
- [x] Exercise package mismatch, missing files, invalid output, stuck child, source outage/recovery, disabled connection, missing installation and restart exhaustion. Preserve host configuration and core health.
- [x] Confirm graceful cancellation and forced child termination with an owned subprocess fixture. Keep initial evidence scoped to supported local/CI platform; defer hostile containment claims.

**Verification:** deterministic clock supervisor tests; actual subprocess crash/hang/exit fixtures; concurrent callbacks and dashboard isolation; source outage while child stays healthy; one- versus three-tab collection counts.

**Exit:** all operational behavior promised by alpha spec has evidence, while source history, hostile sandboxing and distributed writers remain unsupported.

## P7. Check, package and multi-module CI

**Deliverable:** authors can validate/package plugins; host and consumers build through ordinary CI and non-root container constraints.

- [x] Finish `balemoh-plugin check` using same validator/Describe/profile paths as runtime and preview. Positive and negative fixtures cover required capabilities, local schemas, route scope, assets, sizes and response envelopes. Do not accept manifests host would reject.
- [x] Implement package target builds, exact declared files, package-relative paths and immutable output. Report SHA-256 package digest and retain distinct manifest digest used by Describe. Exclude local credentials/config, source caches, dev binaries and temp files. Packaging never installs/publishes/deploys.
- [x] Add rerunnable `hack/check-plugin-modules.sh` for explicit root/client/UI/SDK/Alertmanager/example module generation, tests and vet. Run with GOWORK disabled. Generated-file drift checks must also detect missing/untracked generated outputs, not only git diff on tracked files.
- [x] Extend existing CI to call shared check, compile local subprocess fixtures and run the relevant race suites once the package set exists. Keep functional failures reproducible without live Docker/Kubernetes/Alertmanager services.
- [x] Update Docker dependency-copy order for nested SDK module before module download. Keep API/UI builds CGO-disabled and non-root. Product plugin is independently built/installed, not secretly statically imported into core binary.
- [x] Extend existing local image smoke with an explicitly mounted fixture package and writable private IPC/temp directory, read-only package/root filesystem and non-root user. Verify page/widget and API query paths; no registry publish or cluster rollout.
- [x] Build declared Linux amd64/arm64 plugin package targets. Execute on available local/CI architecture and distinguish cross-compilation evidence from native execution. Do not claim an exhaustive platform test matrix.

**Verification:** clean regeneration across modules; check/package round trip loaded by real API/BFF; non-root image smoke; wrong target/digest rejection; no local secrets or absolute developer replacements in exported package/template.

**Exit:** first author toolchain produces a loadable package. SDK tag/image/plugin publication remains separately scoped.

## P8. Acceptance, documentation and handoff

**Deliverable:** implementation is assessable against all spec invariants and a future deployment has concrete prerequisites.

- [x] Run the coverage matrix below using the smallest suites justified by changed code. Repeat/broaden only for new changes, failures or unresolved concerns.
- [x] Demonstrate fixture plugin and Alertmanager installed through same runtime, with new page/widget added without host import/route/renderer edits. Record package, module and profile versions.
- [x] Document installation/configuration/grants, generated author workflow, browser/source URL distinction, cached-refresh semantics, errors and local SDK bootstrap. Generate environment docs from actual fields, never duplicate defaults by hand.
- [x] Document disable/update/mismatch/rollback procedures. Use additive migration evidence and preserve existing pins/placements; never run destructive down migrations as ordinary rollback.
- [x] Record execution evidence: commands/results, fixture scenarios, browser states, available native platforms and remaining live-environment limits. Do not mark spec's proposed APIs as implemented until their acceptance exists.
- [x] Prepare deployment prerequisites: API/BFF package revision alignment, single backend owner, secret delivery, plugin directory permissions and shared audience boundary. The original deployment exclusion was superseded by the user's explicit homelab authorization; execution is recorded separately in the acceptance ledger.
- [x] Update roadmap/spec status to accurately describe completed and deferred behavior. Keep deferred register below; do not turn future hardening into silent blockers.

**Exit:** spec's first-delivery journey passes and remaining release/deployment actions are explicit. Closing this plan does not perform them.

## Spec invariant coverage

Invariant numbers refer to [spec section 15](../specs/2026-10-25-plugin-system-design.md#15-invariants-and-acceptance).

| Invariant | Delivery stages | Verification |
| --- | --- | --- |
| 1 | P1, P4, P8 | Independent fixture/new page/widget without core-specific imports/routes |
| 2 | P1, P2, P7 | SDK consumer import/build checks and generated API boundary |
| 3 | P1, P2, P3 | Data records have no rendered-HTML fields; private UI render tests; source strings remain escaped text |
| 4 | P3, P5 | Two placements with separate filters/IDs/targets |
| 5 | P3 | SQLite/API replay, revision and concurrent-limit tests |
| 6 | P1, P6, P7 | Invalid manifest/profile/capability fixtures and local fallback |
| 7 | P1, P2, P3 | Direct route/query denial and prefix/grant tests |
| 8 | P2, P6, P7 | Source secret excluded from descriptors/UI/browser/errors/export fixtures |
| 9 | P2, P6 | Configure replay and one-versus-three-tab collection count |
| 10 | P2, P5, P6 | Failed/oversized/truncated source retains timestamps/data |
| 11 | P3, P5, P6 | Browser partial updates at nonzero scroll; focus/geometry/SSE preserved |
| 12 | P1, P2, P6 | Bounded subprocess/callback/output/timeout recovery |
| 13 | P4, P5 | Frontend lifecycle fixture and repeated navigation/removal |
| 14 | P4, P7 | Preview/ordinary host use same validation/render/mount paths |
| 15 | P2, P4, P5, P8 | Product plugin and unrelated generated fixture through same runtime |
| 16 | P2, P5, P8 | Manifest/navigation and source-call assertions omit deferred capabilities |

## Verification commands and evidence rules

Current repository modules are root, client and UI. SDK and plugin commands below become runnable only after their stages create the corresponding modules. Do not run a generation command that writes outside scoped work or treat a nonexistent module as a passing/skipped check.

For a changed module, run from its directory with GOWORK disabled:

```sh
GOWORK=off go generate ./...
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

Use focused tests while implementing, then these module-wide gates for the stage. Race checks require a suitable CGO-enabled toolchain and cover stateful/runtime packages; CGO-disabled build checks remain separate. Client-only regeneration need not trigger unrelated source experiments.

| Module | Build target/evidence |
| --- | --- |
| Root | `CGO_ENABLED=0 GOWORK=off go build ./cmd/balemoh` |
| UI | `CGO_ENABLED=0 GOWORK=off go build ./cmd/balemoh-ui` |
| SDK | CLI build plus protocol/real-child conformance tests |
| Alertmanager/example | Separate `cmd/ui` and optional `cmd/backend` builds, fixture API integration |

After P7, use the shared check script in local development and CI instead of parallel hand-maintained module lists. Keep browser/image checks attached to the surface they prove. A cross-compiled target, successful HTTP response or registered plugin is not evidence of browser behavior/native execution.

Every stage receipt records implemented checkboxes, affected contracts/files, focused commands/results, visible journey, failures/limits and deferred items. `git diff --check` and generated-output reconciliation are required before a stage is considered ready.

## Deferred work and activation

| Deferred capability | Activation |
| --- | --- |
| Stable published SDK migration, signing and reproducible releases | v0.1.0 candidate/public distribution |
| Exhaustive supported-platform containment/cancellation matrix | v0.1.0 candidate |
| Untrusted executable/browser isolation | Before accepting untrusted plugin code |
| Per-user authorization and source-side commands | Before separate audiences or source mutations |
| Multiple API backend owners and distributed collection | Before multi-replica support |
| Alert history and durable plugin-state migrations/restore | Separate source/persistence contract before history claims |
| Marketplace, public installation API and hot replacement | Separate later scope |

Maturity receipt: pre-v0.1.0 exploration; journey is one real source plus current/detail pages and independent persisted widgets; trust is explicitly installed Go/templ code and shared dashboard audience. Current blockers are broken journey, leaked credentials, unsafe routing, lost configuration, unbounded ordinary calls and false guarantees. Future candidate/distribution gates are tracked, not silently promoted into this milestone.

## P3 through P8 execution receipt, 2026-10-25

Migration 000011 persists independent placements. Private placement API, generated
client and native BFF forms implement replay, ETag revisions and the shared budget.
Home and SSE carry references only. Each plugin slot resolves current options and
grants separately; caches include package, connection, placement, observation and
display revisions. Missing/disabled installations retain removable configuration.

The independent author CLI implements init/dev/check/package. Generated UI-only and
fullstack consumers build/test outside the host module using an explicit temporary
SDK replacement. Dev uses the real shell and isolated fixtures. Native check executes
Describe/health. Packaging builds both Linux architectures and exports declared files.
No SDK tag or source release has been published. Consumer CI uses an explicit matching host commit while the SDK is unpublished.

Alertmanager owns filtered current/detail views, snapshot pagination, safe source and
runbook links, severity summary and bounded active-list placements. Browser URL may
be omitted, in which case source links are absent; deployment can configure a separate
browser URL. Source history and mutations remain out of scope.

Runtime supervision implements bounded restart/health policy, invocation deadlines,
structured request IDs and local fallback. Deterministic clock and owned-subprocess
tests cover restart exhaustion, health failures, reset and cancellation. Host asset
serving accepts declared digest paths; one shared browser observer manages modules.

Shared module check, affected race suites, controlled real API/BFF/plugin integration,
non-root/read-only image smoke and Linux amd64/arm64 plugin compilation passed during
implementation. Final frozen identities, independent review and live evidence follow
in the acceptance ledger. Existing branding from origin/main 00cda0d was integrated
into this task worktree to preserve the deployed console identity.

The original P8 deployment exclusion describes the initial implementation authority.
The user's later instruction explicitly authorizes deployment and real source tests;
this adds the internal HTTPS Alertmanager route and GitOps rollout to the execution.
