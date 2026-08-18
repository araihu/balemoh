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

Defaults:

- `BALEMOH_HTTP_ADDR=:8080`
- `BALEMOH_DATABASE_PATH=./data/balemoh.db`
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
read the configured namespace. The Kubernetes discoverer resolves
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

## Local Kubernetes development

DevSpace runs Balemoh inside Kubernetes with a namespace-scoped read-only
ServiceAccount and syncs the repository into a Go development container. The
local cluster is intentionally separate from the application configuration:

```sh
# KinD (Docker-backed Kubernetes)
devspace run-pipeline kind

# vCluster in Docker (vind)
devspace run-pipeline vind

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
the normal `dev` pipeline. The app's RBAC is limited to `get/list` for Pods,
Services, Ingresses, and HTTPRoutes in `${BALEMOH_NAMESPACE}`.

See the [product roadmap](docs/roadmap.md), [discovery design](docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md),
and [Kubernetes/DevSpace plan](docs/superpowers/plans/2026-08-17-balemoh-kubernetes-devspace.md)
for the adapter sequence and scope boundaries.
