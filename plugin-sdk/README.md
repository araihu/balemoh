# Balemoh plugin SDK

Alpha SDK module `github.com/araihu/balemoh/plugin-sdk`. It is independent of
Balemoh's API and BFF internal packages. The first version is `v0.0.1-alpha.1`,
released under MIT through the repository tag `plugin-sdk/v0.0.1-alpha.1`.
Alpha APIs may change; only trusted plugins are supported.

P1 implements closed manifest validation, local schema compilation, UI requests,
host-scoped page URLs and DOM identities, bounded templ rendering, and trusted
local executable transport through HashiCorp go-plugin with gRPC and AutoMTLS.
P2 adds data-only `backend.Handler` methods `Configure`, `Status`, `Query` and
`CloseConnection`, a separate backend child and a scoped `Host` callback.
`runtime.ServeBackend`/`OpenBackend` use the backend executable. `Client.HandleWithServices`
registers host services only for that UI invocation and closes them afterward.
UI-only previews use `NoDataServices` and cannot query sources.

```sh
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go test -race ./...
```

Protocol generation uses protoc 33.0, protoc-gen-go v1.36.11 and
protoc-gen-go-grpc v1.6.1. Install those pinned tools on PATH, then:

```sh
GOWORK=off go generate ./protocol/v1alpha1
```

Packages contain `plugin.json`, declared JSON schemas and a UI executable for the
host GOOS/GOARCH. Hosts call `manifest.Load` before `runtime.Open`; children load
the same package from their working directory before `runtime.Serve`. Describe
must match the host's exact manifest digest, plugin identity and visual profile.

UI handlers render Goshtoso/templ components through `render.Component`. The
buffer rejects HTML beyond 1 MiB. Requests are limited to 64 KiB. Runtime startup
is limited to 10 seconds; calls to 5 seconds or the caller's earlier deadline.
Each child accepts eight concurrent host calls and rejects additional calls
without queuing. `Client.Close` requests shutdown and terminates the owned child.

Only install trusted code. The child receives no inherited host environment or
source secret in its UI capability. Backend receives only explicitly declared
secrets through Configure. go-plugin does not restrict filesystem or network access.
P1 supports GET page content, sidebar navigation and home widgets. Unknown
required capabilities fail installation; unsupported optional contributions are
omitted. P2 accepts declared query schemas, explicit per-connection grants and backend-only
secret declarations. Declared CSS and ES modules are served only from digest-pinned package paths. ES modules export `mount(root)` and return a cleanup function; the shared host observer invokes cleanup when the contribution is removed.

Pages declare a required title and optional description in the manifest. The BFF
uses these values for initial document metadata, never waits for the child to
build its shell, and loads page/widget HTML through isolated HTMX requests.
Home and dashboard SSE make no plugin UI calls. Each slot has its own deadline
and local failure/retry state. The shared HTMX runtime loads isolated slots. Without JavaScript, a native iframe loads the contribution independently; filters, detail links and configuration forms remain usable. Pages/sidebar/widgets must declare their
extension-point capability before the host mounts them.

See the [system specification](https://github.com/araihu/balemoh/blob/plugin-sdk/v0.0.1-alpha.1/docs/superpowers/specs/2026-10-25-plugin-system-design.md)
and [implementation plan](https://github.com/araihu/balemoh/blob/plugin-sdk/v0.0.1-alpha.1/docs/superpowers/plans/2026-10-25-plugin-system-implementation.md).
The first external consumer is `araihu/balemoh-plugin-alertmanager`, with a read-only Alertmanager API v2 backend and current/detail pages, filtering, pagination, summary and active-list widgets. Generic process conformance fixtures live under
`runtime/testdata`, and are never installed as a product plugin.

The `balemoh.goshtoso/v1alpha2` author baseline remains Goshtoso v0.4.4;
the host serves the compatible v0.4.5 runtime, including nested-popover focus
fixes. Consumers compile their components against the scaffold baseline and use
the host's shared assets. This patch split does not waive manifest/capability
validation or the requirement to use a compatible host and SDK version.
For this alpha, build the integration host from the same `plugin-sdk/` release tag.

## Backend and data contracts

`Configure` receives a host-issued connection ID, positive decimal configuration
revision, schema-validated public config and declared secrets. Its lifetime is
independent of the Configure RPC context. Start asynchronous collection and
return status without waiting for source health. Repeated same-revision config
is a no-op; reject conflicting/invalid config without replacing previous state.

`Status` reads metadata only. `Query` reads retained observations and returns a
bounded JSON object, state, observation revision, observation/last-success times
and optional sanitized source error. Both host and SDK validate query schemas.
Use `backend.Fail` for typed errors. Hosts never forward arbitrary error text.
`CloseConnection` cancels collection and releases connection resources.

Backends may optionally declare `notifications.publish/v1`. The declaration
requires a backend executable for every target, including packages with no
queries. Hosts grant publication per connection with
`BackendClient.ConfigureWithNotifications`; the resulting
`backend.Config.Notifications` is a scoped `notifications.Publisher`. The
callback accepts only `notifications.Message` and returns a host-issued
`notifications.Receipt`; source identity never crosses from the child. The
SDK applies a 16 KiB envelope bound, four concurrent calls per connection and
a five-second publish deadline. UI handlers receive no publisher.

`Message.ServiceID` optionally references the exact 64-character lowercase hex
catalog ID. It is presentation metadata, separate from producer identity and
`Destination`. The host fills an empty reference once; a different nonempty
reference conflicts. Replay still compares the original immutable event content.
The inbox resolves a visible service through the producer connection's catalog
grant, without calling the plugin. Missing, hidden or denied services retain the
producer icon.

Backends may type-assert `notifications.ServiceBinder` on their publisher and call
`BindService(ctx, eventID, serviceID)` to enrich a retained event from their own
connection. Binding never creates a notification, changes content or marks it
unread. It returns `not_found` for an absent or expired event and `conflict` for a
replacement reference. The callback shares the publication deadline and call
budget. Hosts without this additive alpha.3 callback may reject it; producers
must preserve their existing baseline and delivery behavior.

Backend queries allow four concurrent calls per child, without queuing. Input
is limited to 64 KiB; data result to 1 MiB. Caller deadlines propagate through
API and callback. UI requests retain their separate eight-call budget. Each UI
callback is scoped by the BFF to its connection and query grants; the UI cannot
supply another connection or obtain the private API credential.

`manifest.Load` validates declarations/schemas and computes manifest digest.
`Package.ContentDigest` hashes all package file names, permissions, lengths and
contents, rejecting symlinks and packages over 128 files/256 MiB. Both hosts pin
the complete package and manifest through explicit deployment configuration.
Keep credentials and deployment JSON outside the package. This pins trusted
local files; it is not signed package distribution or an OS sandbox.

`credential.ReadBearerFile` and `credential.MatchBearer` define one RFC 6750
lexical contract for both hosts and service authentication. Source token files
use the same parser; only the private service credential requires 32 bytes.

API descriptors expose nullable applied/observation revisions before Configure
succeeds, host readiness and separate sanitized Configure failure metadata.
Source attempt/error metadata retains its backend meaning. BFF resolves current
query grants inside each isolated invocation.

API runtime owns one backend child per plugin and caches Status independently
of browser requests. BFF discovers optional data and starts at most four UI children concurrently
after returning its registry, so its core listener does not wait for them.
BFF shares one UI child across its configured connections.
Home, shell and dashboard SSE use metadata/references only. Source observations
and plugin HTML are loaded through separate isolated requests. Persistent placements use the private data API with UUID replay, ETag revisions and a shared 128-selection budget. Supervision restarts failed children with bounded backoff. Package replacement requires a host restart; live replacement is unsupported.

The private descriptor route returns `{plugins, problems}`. Per-installation or
connection validation problems use public ordinals/IDs and fixed error messages;
paths, URLs, rejected values and secret references are omitted. Invalid optional
entries disable/retain their connection records and do not stop valid entries or
core API routes. Malformed top-level documents/global service credentials remain
explicit startup errors.

## Author commands

Install the released CLI, then generate an independent project:

```sh
GOWORK=off go install github.com/araihu/balemoh/plugin-sdk/cmd/balemoh-plugin@v0.0.1-alpha.1
balemoh-plugin init --id example.metrics --module example.com/metrics --output ../metrics --backend
cd ../metrics
GOWORK=off go generate ./...
GOWORK=off go mod tidy
GOWORK=off go test ./...
balemoh-plugin dev --source . --host /path/to/balemoh-ui
```

Omit `--backend` for UI-only contributions. `init` accepts a new or empty directory.
The generated module owns its templ source, tests and assets and requires SDK
`v0.0.1-alpha.1` through normal Go module resolution, with no source checkout or
replacement. Commit its `go.mod` and `go.sum`. Optional `dev --sdk` is for SDK
co-development only and creates a temporary development modfile. An exported
module never includes a developer absolute replacement or go.work.
The real host must report the matching protocol/profile via `--plugin-host-info`.

Dev runs the actual BFF shell with an isolated loopback fixture API, two widget
placements and temporary service credentials. The printed state URLs select
loading, empty, fresh, stale or error responses. Fullstack handlers can inspect
those states through Query; UI-only handlers have no data query. Author file
changes rebuild and restart only the owned preview. `--watch=false` disables it.
Provide operation results with `--fixtures file.json`, keyed by state then query ID.
Preview data does not contact the source backend or a production API.

```sh
balemoh-plugin package --source . --output dist --target linux/amd64 --target linux/arm64
balemoh-plugin check --package dist
```

Generate templ and tidy the author module before packaging. `package` compiles the
requested targets and copies only manifest-declared schemas/assets plus their local
schema references. Output must be new. It prints separate manifest and package
SHA-256 digests; neither operation publishes or installs anything. `check` validates
manifest, capabilities, schemas, assets and complete package digest, then executes
Describe and health on the current native target. Handler output validation runs
on every runtime call. Cross-compilation is distinct from executing that target.

## UI contracts

Implement `ui.Handler` or register SDK router pages/fragments. Requests carry the
plugin/connection/placement identity, region, query/form/body, display context,
request ID and exact package digest. `ui.PageURL`, `FragmentURL`, `WidgetURL`,
`AssetURL` and DOM helpers retain host routing and contribution scope.
`ui.HostServices.Query` is limited to the current connection's granted operations;
`Placement` returns current options, never source secrets. `plugin-sdk/testing`
provides fake services for independent author tests.

Return validated `ui.Response` page/fragment HTML, no-content, or a typed redirect
to a registered page in the same connection. Register only GET/POST methods.
Responses may reference declared asset IDs; the host owns the document, navigation,
shared styles/runtime and outer widget wrapper. Never emit another shell, HTMX,
Alpine or global CSS replacement. Fragment HTML replaces its contribution region.
Use templ's normal escaping for source text. Source/API credentials stay out of UI.

See [operator guide](https://github.com/araihu/balemoh/blob/plugin-sdk/v0.0.1-alpha.1/docs/plugins/operations.md) and [acceptance ledger](https://github.com/araihu/balemoh/blob/plugin-sdk/v0.0.1-alpha.1/docs/plugins/acceptance.md).

## Producer logo assets

The optional top-level `icon` field names an asset ID, not a URL or symbol.
Declare that asset with `kind: image` and `mediaType: image/png`. `manifest.Load`
checks PNG headers and dimensions up to 2048x2048; scripts and SVG are rejected.
The host renders the logo through a local, content-addressed asset URL without
calling the plugin. The notification's authenticated source determines the logo,
independently of its destination. Existing manifests without `icon` retain the
navigation/source-symbol fallback.

Declare license and attribution files with `kind: document`, `mediaType:
text/plain` so packaging copies and hashes them alongside the image. Image and
document assets follow the existing 5 MiB per-file bound and package-root rules.
This additive alpha manifest contract requires a matching host; install upgraded
host binaries and plugin package pins together. No notification payload or RPC
change is required.

## Release checks

A nested Go module uses tags prefixed with `plugin-sdk/`; a root `v` tag does not
version this module. Tag the reviewed, merged commit without moving an existing
tag. The current Go dependency version is `v0.0.1-alpha.3`, not the prefixed Git tag.

After publishing, run `bash hack/check-published-plugin-sdk.sh v0.0.1-alpha.3`
from the repository root. It installs the CLI and builds independent UI-only and
backend projects using a fresh public Go module cache and checksum database,
without a local replacement. Host integration tests must compile the consumer
against that published module even when the integration host comes from the
matching repository tag.

## Service presentation

Declare optional capability `catalog.services.read/v1`, then type-assert
`ui.ServiceCatalog` on the invocation's `ui.HostServices`. Call
`ResolveServices(ctx, ids)` with 1 to 64 distinct lowercase 64-character hex
service IDs. Results contain only ID, effective display name and local image
or sprite references. IDs that no longer resolve to visible live services
are omitted. Source URLs, endpoints and credentials are never returned.

The host must explicitly grant IDs per installation/connection. In Balemoh UI
configuration, `serviceIDs` is that allow-list. An ungranted ID or an undeclared
capability rejects the batch. Catalog failure must not hide the plugin's main
content. No name matching or catalog enumeration is provided. The optional
interface keeps existing HostServices authors source compatible; old hosts
return an unavailable callback and consumers should retain their text fallback.
