# Balemoh

Balemoh is a Go service. Local development uses Go 1.26.6 and the pinned
dependencies and tools declared in `go.mod`.

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
RBAC is limited to `get/list` for Pods, Services, Ingresses, and HTTPRoutes in
`${BALEMOH_NAMESPACE}`; the UI does not receive a Kubernetes token.

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
