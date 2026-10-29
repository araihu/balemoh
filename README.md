# Balemoh

Public documentation: [balemoh.araihu.com](https://balemoh.araihu.com).
The static site uses Margo for guides and Manja for the OpenAPI reference.
See [site build and Cloudflare deployment](site/README.md).

Balemoh is a Go service. Local development uses Go 1.27.3 and the pinned
dependencies and tools declared in `go.mod`.

## Public read-only demo

Run `bash hack/run-demo.sh` with Docker Compose available and GHCR read access to the pinned internal plugin
carrier, then open
`http://127.0.0.1:18081`. The script builds the host, installs a digest-pinned
Alertmanager plugin and creates temporary, separate plugin/operator credentials.
Ctrl-C removes the containers, credentials and disposable database.

The demo has twelve fictional services, a three-node cluster and a Docker host,
one hour of initial telemetry and a synthetic Alertmanager API. The generator
runs inside the API process through catalog/telemetry application contracts.
Samples advance once per minute independently of visitors. Plugin pages use the
real backend/UI processes against the loopback synthetic source. No Kubernetes
credentials, Docker socket or production data are used.

`BALEMOH_READ_ONLY=true` denies operator API writes while retaining authenticated
federation imports and private retained plugin queries. It is an HTTP write
policy, not a read-only SQLite file. `BALEMOH_DEMO_ENABLED=true` additionally
requires explicit loopback API/source listeners, refuses existing databases without
its synthetic ownership marker, restricts plugins to the synthetic source, disables real collectors and
federation configuration, and initializes synthetic data internally. The initial
plugin widgets are provisioned for the explicit `araihu.alertmanager/demo`
connection only.

The public BFF verifies the synthetic API profile before opening its listener.

`BALEMOH_UI_READ_ONLY=true` denies public BFF writes and editing routes, disables
mutation controls and SSE, and enables a global 16-request concurrency limit with
20 requests/second and a 40-request burst. Requests have an eight-second response
deadline. Work retains its concurrency slot until its handler actually exits.
The cheap health endpoint is separate from admission so overload does not trigger
probe-driven restarts. Browser refreshes run once a minute, pause in hidden tabs
and read retained observations. Navigation and filters are per browser; no
server-side visitor sessions are created. Browser color preferences remain local.
Set `BALEMOH_UI_PUBLIC_URL` to the demo HTTPS origin for canonical/social metadata; an unset demo origin omits these
metadata fields and never points visitors at the private instance.

The BFF must be the only public entry point. Keep API, synthetic source and all
service credentials private. A dedicated `cloudflared` connector can target a
loopback BFF inside the same pod without an ingress gateway. This reduces direct
origin exposure; it does not remove shared-uplink or application overload risk.
Use fixed container/storage budgets and edge traffic controls before publishing.

## Development

Run generation, tests, and static checks:

```sh
go generate ./...
go test ./...
go vet ./...
```

Run the service in one terminal:

```sh
go run ./cmd/balemoh
```

In a second terminal, check the health endpoint:

```sh
curl --fail http://127.0.0.1:8080/healthz
```

The service creates the parent directory for `BALEMOH_DATABASE_PATH` and
automatically runs embedded migrations on startup. The bootstrap migration is
safe to run repeatedly; migration bookkeeping is stored in SQLite's
`schema_migrations` table.

## UI and generated client modules

The server-rendered UI is a separate `ui` Go module and acts as a private BFF.
It uses `araihu/goshtoso` and `goshtoso-app-shells`, and talks to the API only
through the generated SDK in the separate `client` module:

```sh
(cd client && GOWORK=off go generate ./... && GOWORK=off go test ./...)
(cd ui && GOWORK=off go generate ./... && GOWORK=off go test ./...)
```

Run the UI after the API with `BALEMOH_UI_API_BASE_URL` pointing at the API;
it listens on `:8081` by default. `/` shows pinned services and `/staging`
provides the discovery review flow. Pin, unpin, and sync actions use native
HTML forms with POST/Redirect/GET, so the initial BFF does not require HTMX.

Trusted application plugins use `BALEMOH_PLUGIN_CONFIG` in the API and
`BALEMOH_UI_PLUGIN_CONFIG` in the BFF. The independent [plugin SDK](plugin-sdk/README.md)
supplies manifest validation, gRPC transport, templ rendering and author tooling.
`/plugins/widgets` lists widget contributions and retained configuration; home
pinning, options and enabled state persist independently of source availability.
The separate `balemoh-plugin-alertmanager` repository implements a read-only
Alertmanager API v2 backend, current/detail pages, filters and home widgets.
Plugin discovery, child startup and contribution rendering run asynchronously,
independently of core listener readiness. Optional fixture previews still use
`BALEMOH_UI_PLUGIN_PREVIEW_CONFIG`; see [plugin operations](docs/plugins/operations.md)
for installation, credentials and trusted-code boundaries.

Defaults:

- `BALEMOH_HTTP_ADDR=:8080`
- `BALEMOH_DATABASE_PATH=./data/balemoh.db`
- `BALEMOH_DISCOVERY_SYNC_INTERVAL=5m` (set to `0` to disable automatic sync)
- Kubernetes discovery is disabled unless `BALEMOH_KUBERNETES_ENABLED=true`.
- Docker/Podman discovery is disabled unless `BALEMOH_CONTAINER_ENABLED=true`.
- `BALEMOH_CONTAINER_HOST=unix:///var/run/docker.sock`

Runtime configuration is parsed once from environment variables before database
startup and migrations. See the [generated environment documentation](internal/config/environment.md)
for the complete configuration reference.

## Discovery catalog API

The first catalog slice exposes persistent staging and homepage views:

```sh
curl --fail http://127.0.0.1:8080/api/v1/staging/services
curl --fail http://127.0.0.1:8080/api/v1/homepage/services
curl --fail -X POST http://127.0.0.1:8080/api/v1/discovery/sync
curl --fail -X POST http://127.0.0.1:8080/api/v1/staging/services/SERVICE_ID/pin
curl --fail -X DELETE http://127.0.0.1:8080/api/v1/staging/services/SERVICE_ID/pin
```

Discovery sync has no sources by default. When the process runs inside Kubernetes
with `BALEMOH_KUBERNETES_ENABLED=true`, the in-cluster ServiceAccount is used to
read the configured namespace. Leave `BALEMOH_KUBERNETES_NAMESPACE` empty for
cluster-wide discovery. The Kubernetes discoverer resolves
`HTTPRoute`/Ingress backends to Services first, then Services to Pods through
label selectors. If no route/Ingress resolves a Service it falls back to all
Services; `ExternalName` Services, Services without selectors, and externally
addressed Services are staged even without a Pod. A Pod is staged only when a
selected Service actually matches its labels. It does not mutate cluster
resources. Discovered candidates remain in staging until the user explicitly
pins them. A port observation is not treated as an exact hostname; endpoint
provenance records which adapter supplied the evidence.

Staging groups explicit relationships automatically. A single HTTPRoute with a
root path can group its path backends under that facing address. Backends also
exposed under unrelated addresses remain separate. Route evidence retains Gateway
and listener scope, exact versus prefix matches, and redirect destinations.
Redirect-only routes attach only when their destination resolves unambiguously
in that scope. Header-dependent, wildcard, and unsupported filter matches do not
establish automatic relationships.

Services with identical namespace, selectors, and target-port sets can be aliases
when their facing addresses do not identify separate virtual hosts. Sharing Pods,
labels, a namespace, or a hostname alone never proves equivalence. The root-path
Service represents an app group; otherwise the lexically first Service does.
Existing resource IDs and edits stay stored. Pinned non-Service resources remain
explicit rows; they do not pin the Service representative. Pins propagate across
merged Service groups, and unpinning the representative clears its member pins atomically. Display overrides belong to the
representative resource; other members' saved overrides remain stored.

LoadBalancer addresses, external IPs, ExternalName hosts, and NodePort node
addresses are preserved as protocol-qualified endpoint observations. No HTTP
scheme is inferred from a TCP port. NodePort discovery additionally requires
read-only access to Nodes. Older snapshots without the new evidence retain
Service-based grouping until discovery refreshes them.


When `BALEMOH_CONTAINER_ENABLED=true`, the Docker-compatible adapter reads
running containers from `BALEMOH_CONTAINER_HOST` and stages both individual
containers and Compose services identified by Compose labels. Images and
published host ports are retained as evidence. A published binding is exposed
as a `tcp://IP:port` or `udp://IP:port` observation; `0.0.0.0` means the
runtime reported all host interfaces. The adapter does not infer a hostname or
call an extension such as Traefik. The container socket is a privileged
capability even though this adapter only performs list requests, so configure
its access deliberately.

## Local Docker/Podman discovery

Run Balemoh on the host with a stable source ID and a Docker-compatible socket
or endpoint:

```sh
BALEMOH_CONTAINER_ENABLED=true \
BALEMOH_CONTAINER_SOURCE_ID=docker-desktop \
BALEMOH_CONTAINER_HOST=unix:///var/run/docker.sock \
go run ./cmd/balemoh

curl --fail -X POST http://127.0.0.1:8080/api/v1/discovery/sync
curl --fail http://127.0.0.1:8080/api/v1/staging/services
```

For rootless Podman, set `BALEMOH_CONTAINER_HOST` to the user Podman socket.

## Federação entre instâncias

Uma instância pode ser agente, gateway ou ambos. O agente executa discovery
local e, após `POST /api/v1/discovery/sync`, envia um snapshot autenticado para
o gateway. O gateway não precisa de acesso ao socket Docker nem ao RBAC do
cluster remoto; mantém uma visão única no próprio staging.

Configure o gateway com registro explícito das fontes e um Bearer token
independente para cada fonte:

```sh
BALEMOH_FEDERATION_ALLOWED_SOURCES=container/docker-local,kubernetes/cluster-1 \
BALEMOH_FEDERATION_SOURCE_TOKENS=container/docker-local=container-secret,kubernetes/cluster-1=kubernetes-secret \
go run ./cmd/balemoh
```

Configure cada agente com a URL e o token emitido pelo gateway:

```sh
BALEMOH_FEDERATION_GATEWAY_URL=https://balemoh-gateway.example.test \
BALEMOH_FEDERATION_TOKEN=container-secret \
BALEMOH_CONTAINER_ENABLED=true \
BALEMOH_CONTAINER_SOURCE_ID=docker-local \
go run ./cmd/balemoh
```

O snapshot preserva fonte, recurso, imagens, endpoints e provenance. Pins não
são enviados: cada gateway controla seus próprios pins. A importação reconcilia
a fonte: remove candidatos remotos ausentes que ainda não foram pinados e
preserva pins locais. Cada fonte remota tem credencial própria; fontes locais e
remotas com a mesma identidade são rejeitadas na configuração. A sincronização
automática roda uma vez no startup e depois em
`BALEMOH_DISCOVERY_SYNC_INTERVAL`; o endpoint de sync manual continua
disponível para refresh explícito. Produção deve usar HTTPS; HTTP só é aceito
com opt-in explícito para gateway loopback local.

## Local Kubernetes development

DevSpace runs Balemoh inside Kubernetes with a namespace-scoped read-only
ServiceAccount and syncs the repository into Go development containers. The
API and UI are separate DevSpace deployments; the UI calls the API through the
in-cluster `app:8080` Service. The local cluster is intentionally separate
from the application configuration:

```sh
# KinD (Docker-backed Kubernetes)
devspace run-pipeline kind

# vCluster in Docker (vind)
devspace run-pipeline vind

# Homelab Kubernetes: internal Gateway + Pi-hole DNS
devspace run-pipeline homelab

# Same homelab resources without starting file sync/port forwarding
devspace run-pipeline homelab-deploy

# Open after the homelab route and certificate are ready
open https://balemoh.decastro.me

# API and staging while devspace is running
curl --fail http://127.0.0.1:8080/api/v1/staging/services

# Optional teardown
./hack/dev-k8s.sh kind-down
./hack/dev-k8s.sh vind-down
```

Install `devspace`, `kubectl`, Docker, and either `kind` or `vcluster` first.
Override the local cluster names with `BALEMOH_KIND_CLUSTER_NAME` or
`BALEMOH_VIND_CLUSTER_NAME`. The `HTTPRoute` CRD is optional; install the
Gateway API standard CRDs in the selected cluster when route fixtures are
needed. The adapter treats an absent CRD as an empty route source.

The `kind` and `vind` pipelines create or connect the local cluster, then run
the normal `dev` pipeline. The `homelab` pipeline additionally applies
`devspace/httproute.yaml`, which binds the UI to the existing
`default/internal-gateway` HTTPS listener at `balemoh.decastro.me`. The app's
RBAC is read-only cluster-wide for Pods, Services, Nodes, Ingresses, and HTTPRoutes;
the UI does not receive a Kubernetes token. `BALEMOH_KUBERNETES_NAMESPACE` is
empty by default, so discovery lists those resources across all namespaces.
Pod candidates are still emitted only when a Service, Ingress, or HTTPRoute
cross-references them.

The homelab pipeline expects the Gateway API CRD, Envoy Gateway's `internal`
GatewayClass, the `default/internal-gateway`, and the internal TLS certificate
to already be managed by the homelab GitOps repository. ExternalDNS is already
configured there with the Pi-hole provider and the `*.decastro.me` filter, so
the route hostname is the DNS input; Balemoh does not call Pi-hole directly.
The pipeline does not apply resources to the cluster until it is explicitly
run.

See the [product roadmap](docs/roadmap.md), [discovery design](docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md),
and [Kubernetes/DevSpace plan](docs/superpowers/plans/2026-08-17-balemoh-kubernetes-devspace.md)
for the adapter sequence and scope boundaries.

See [Icon library](docs/icons.md) for catalog vendoring, uploads, and storage requirements.

See the [application plugin specification](docs/superpowers/specs/2026-10-25-plugin-system-design.md)
for the SDK, manifest, runtime, scaffold plan and first Alertmanager plugin.
P2 implements the backend/data path with private service authentication and
scoped BFF callbacks. Enable it explicitly through `BALEMOH_PLUGIN_CONFIG` and
`BALEMOH_PLUGIN_SERVICE_TOKEN_FILE` in API, and the corresponding
`BALEMOH_UI_PLUGIN_CONFIG`/`BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE` in BFF.
Deployment JSON and package pins are documented in the separate
`balemoh-plugin-alertmanager` consumer README. Defaults leave plugins disabled;
source failures affect only isolated plugin content. Individual package/pin or
connection errors are published in the private `/api/v1/plugins` catalog
`problems` array while core routes and valid installations continue. A malformed
top-level deployment document or invalid global service credential fails startup.

## Production image

The source-owned `Publish image` workflow runs manually from `main`. It checks
both processes in a non-root, read-only container before publishing amd64 and
arm64 images to `ghcr.io/araihu/balemoh`. Deploy the resulting immutable digest.

The image starts `/balemoh` by default. Set the container command to
`/balemoh-ui` for the UI. Mount persistent storage at `/data`; the image uses
`/data/balemoh.db` for SQLite and `/data/icons` for the UI icon cache. The UI
needs `BALEMOH_UI_API_BASE_URL` pointing to the API. Kubernetes discovery uses
read-only RBAC on the API service account; the UI needs no Kubernetes token.

Container checks: `bash hack/smoke-image.sh IMAGE` and `bash hack/smoke-demo.sh IMAGE`.

## Web Push notifications

Web Push lets browsers receive new inbox messages even when no Balemoh tab is
open. It is disabled until the API has both settings below. No browser-side
polling is required for delivery.

```sh
go run ./cmd/balemoh-webpush-keygen /secure/path/balemoh-vapid.key
```

The generator creates a new private-key file with mode 0600 and refuses to
overwrite an existing key. Store it outside the repository, preserve it across
restarts, back it up with the notification database, and mount it as a secret.
Configure the API through its generated environment contract:

```sh
BALEMOH_WEB_PUSH_VAPID_KEY_FILE=/run/secrets/balemoh-vapid.key
BALEMOH_WEB_PUSH_SUBJECT=mailto:operator@example.com
BALEMOH_NOTIFICATION_OPERATOR_TOKEN_FILE=/run/secrets/notification-operator
```

The existing UI notification operator credential must match the API. VAPID keys
stay exclusively in the API. Browsers receive the public key through the BFF.
Enable browser notifications from the bell tray or inbox. On iPhone/iPad, add
Balemoh to the Home Screen before opting in. Balemoh serves a web app manifest.

Subscriptions and pending sends use SQLite migration 17. Publication queues one
job per subscribed browser in the same transaction, including plugin messages.
Identical publisher retries do not queue another job. Subscribing never replays
old inbox messages. The API checks jobs every two seconds in batches of up to 32,
with a 10-second request timeout and a one-minute claim lease. Leases allow
restart recovery and exclude simultaneous sends from other API replicas sharing
the database. Transient failures retry up to eight attempts with exponential
backoff; jobs expire after 24 hours. Accepted pushes have a 24-hour push-service
TTL. Delivery is at least once; notification tags coalesce retries by message ID.
Read or cleared messages are skipped before claiming. An in-flight send may
already have reached the push service when a message is read or removed.

Each browser owns its subscription through a random management token whose hash
is stored on the server. No subscription list or private endpoint is exposed.
There is a 128-subscription limit and a 90-day lease renewed on visits. HTTP
404/410 responses remove expired subscriptions. VAPID key changes invalidate
existing subscriptions and require browsers to opt in again. Disabling server
push removes queued sends but retains subscriptions for the same key. Rolling
back migration 17 removes push state; the existing notification inbox remains.

The API needs outbound HTTPS and DNS access to browser push services. The
transport rejects private/reserved destinations, validates resolved addresses at
connection time, disables proxies and refuses redirects. No inbound connection
from a push provider to Balemoh is needed. Push endpoints and encryption keys
are private data and must not be logged. Notification titles and preview text
are encrypted for the browser; the browser vendor's push service handles
transport. Message text may appear on the OS lock screen.

Keep the same VAPID configuration across replicas. OS/browser settings, device
connectivity and push-service expiry affect delivery. Full notification text
remains in the inbox; pushed previews are bounded to fit the protocol limit.
