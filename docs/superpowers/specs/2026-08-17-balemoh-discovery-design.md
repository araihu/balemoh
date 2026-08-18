# Balemoh Discovery and Homepage Catalog Design

**Status:** Catalog foundation accepted; concrete Kubernetes and Docker-compatible container adapters are implemented, with the local DevSpace workflow specified in `docs/superpowers/plans/2026-08-17-balemoh-kubernetes-devspace.md`.

## Goal

Add the first domain slice behind Balemoh's homelab homepage: discoverable service candidates, a persistent staging area, explicit pin/unpin decisions that control the homepage view, and read-only Kubernetes and Docker-compatible container sources.

The slice must make Docker and Kubernetes adapters possible without coupling the application core to either platform. It must also preserve uncertainty: a discovered resource may exist without a known hostname, and a port observation must not be presented as an exact route.

## Context

Balemoh already has an API-first Go foundation:

- OpenAPI is the HTTP contract authority.
- `oapi-codegen` generates the `net/http` server boundary.
- application packages expose inward-facing ports.
- SQLite/sqlc provide local persistence.
- embedded `golang-migrate` migrations run during startup.

The initial API exposed only `/healthz`. This design adds the first business
resource; the companion Kubernetes adapter keeps platform code outside the
application core and is enabled only from the composition root.

## Scope

Included:

- source-agnostic catalog domain types;
- discovery and catalog application ports;
- a synchronous discovery run over registered discoverers;
- SQLite/sqlc persistence for candidates, endpoint observations, and pin state;
- generated OpenAPI routes for staging, homepage, and discovery sync;
- HTTP translation and error mapping;
- tests for domain behavior, persistence, and HTTP behavior;
- roadmap documentation for Docker, Kubernetes, reconciliation, UI, and hardening phases.

Excluded:

- container mutation or lifecycle control;
- cluster-wide provisioning or deployment automation outside the local DevSpace workflow;
- Traefik or other discovery extension implementation;
- background scheduling, queues, or event streaming;
- authentication, authorization, rate limiting, and multi-user ownership;
- UI implementation, while keeping the API usable by a future Goshtoso-based UI;
- deployment, release, push, merge, or external publication.

## Design decisions

### Recommended approach: source-agnostic catalog with synchronous ports

The application owns a `catalog` package. Platform adapters implement a narrow `Discoverer` port and return normalized candidates. A SQLite adapter implements the catalog store port. The HTTP adapter depends only on the application use-case interface and generated API types.

This keeps the first implementation small while fixing the boundaries that matter:

```text
Docker / Kubernetes / extension adapters
                |
          Discoverer port
                |
       application/catalog service
          |                 |
   CatalogStore port    sync report
          |
      SQLite/sqlc adapter
          |
       SQLite database
```

Alternatives considered:

1. **Platform-specific domain models.** Rejected. Docker and Kubernetes would leak into HTTP and persistence, making later adapters expensive and making extension data inconsistent.
2. **Event/job architecture first.** Deferred. Useful for continuous discovery and large installations, but unnecessary before one synchronous adapter and a small local catalog exist.
3. **In-memory staging only.** Rejected. Pin decisions must survive restart, and SQLite is already part of the runtime foundation.

### Candidate identity

Every candidate has a deterministic ID derived from:

```text
source kind + source stable ID + resource kind + namespace + resource name
```

The application hashes this canonical identity. A source adapter must provide a stable source ID, such as a Docker host identity or Kubernetes cluster identity. Resource names are not globally unique without the source and namespace context.

An upsert replaces the latest observed metadata and endpoints but never changes the existing pin decision. Discovery is therefore safe to repeat.

### Uncertainty and endpoint evidence

Discovery does not imply publication. Candidates may have no endpoint, or only a port observation. An endpoint contains:

- display name;
- optional URL, empty when no exact hostname is known;
- optional port, zero when no port was observed;
- protocol/evidence text for adapter-provided context.

The API does not invent hostnames. The Kubernetes Ingress/HTTPRoute adapter can
provide an exact host/path observation and evidence such as
`kubernetes.ingress` or `kubernetes.httproute`; a container port-only
observation remains visibly incomplete until an extension resolves it.

### Staging and homepage semantics

One persisted candidate represents both views:

- staging lists all currently known candidates;
- homepage lists candidates with a non-null `pinned_at` value;
- `POST .../{id}/pin` sets the pin timestamp;
- `DELETE .../{id}/pin` clears it.

Pinning is explicit and idempotent. Repeating pin or unpin requests leaves the desired state unchanged. A candidate disappearing from a later discovery run is not deleted by this slice; reconciliation and stale-resource policy belong to a later roadmap phase.

## Domain contracts

The application package `internal/application/catalog` owns these concepts:

```go
type SourceRef struct {
    Kind string
    ID   string
}

type ResourceRef struct {
    Kind      string
    Namespace string
    Name      string
}

type Endpoint struct {
    Name       string
    URL        string
    Port       int
    Protocol   string
    Provenance string
}

type Candidate struct {
    ID          string
    Source      SourceRef
    Resource    ResourceRef
    DisplayName string
    Description string
    Metadata    map[string]string
    Endpoints   []Endpoint
    Images      []string
    ObservedAt  time.Time
    PinnedAt    *time.Time
}
```

`NewCandidate` computes the stable ID and defaults the display name to the resource name. Validation rejects missing source/resource identity, invalid endpoint ports, and malformed non-empty URLs. Metadata and endpoint slices are normalized to empty collections rather than nil at API boundaries.

Application ports:

```go
type CatalogStore interface {
    Upsert(context.Context, Candidate) error
    List(context.Context, bool) ([]Candidate, error)
    SetPinned(context.Context, string, bool) (Candidate, error)
}

type Discoverer interface {
    Name() string
    Discover(context.Context) ([]Candidate, error)
}
```

The catalog service exposes list, pin, unpin, homepage, and sync operations. `ErrNotFound` is an application sentinel; adapters translate storage-specific no-row errors into it.

Sync runs registered discoverers in order. With no registered discoverers, it succeeds with zero sources and zero candidates. A discoverer error prevents a successful sync response and does not expose the underlying platform error through HTTP.

## API contract

OpenAPI adds these routes under `/api/v1`:

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/staging/services` | List all candidates, including pin state. |
| `POST` | `/staging/services/{serviceId}/pin` | Pin candidate; return updated candidate. |
| `DELETE` | `/staging/services/{serviceId}/pin` | Unpin candidate; return updated candidate. |
| `GET` | `/homepage/services` | List pinned candidates only. |
| `POST` | `/discovery/sync` | Run registered discoverers and return source/candidate counts. |

List responses use an object containing `services`, so pagination and additional catalog metadata can be added without changing the top-level response shape. Candidate JSON includes source/resource references, metadata, endpoint observations, Pod image observations, observation time, and pin state.

HTTP behavior:

- successful reads and mutations return `200` JSON;
- unknown candidate IDs return `404` with the existing `ErrorResponse` shape;
- storage or discovery failures return `503` with a generic message;
- platform/database error details never reach the client.

## Persistence

Migration `000002_catalog` adds the candidate and endpoint tables. Migration
`000003_catalog_images` adds the structured image observations:

```text
discovered_services
  id, source_kind, source_id, resource_kind, resource_namespace,
  resource_name, display_name, description, metadata_json, images_json,
  observed_at, pinned_at, created_at, updated_at

service_endpoints
  id, service_id, name, url, port, protocol, provenance
```

`service_endpoints.service_id` references `discovered_services.id` with cascading delete. Upsert runs in one transaction: write the candidate, replace its endpoint rows, and commit. Pin state is updated separately and survives candidate upserts.

The storage adapter converts timestamps to RFC3339Nano strings and metadata to JSON. Invalid persisted metadata is returned as an internal error rather than silently discarded.

## Kubernetes adapter

`internal/adapters/kubernetes` implements `catalog.Discoverer` with typed
`client-go` reads for Pods, Services, and Ingresses, and a dynamic client for
Gateway API `gateway.networking.k8s.io/v1` HTTPRoutes. Route and Ingress
backend references are resolved to Services first, and Service selectors are
then matched against Pods. When no HTTPRoute or Ingress resolves a Service,
all Services are eligible for the fallback; Services without selectors,
`ExternalName`, or external addresses remain eligible in either mode. Pods are candidates only
when a selected Service matches their labels, so standalone Pods do not enter
staging. Services and Pods contribute port observations; Pods also contribute
deduplicated init, regular, and ephemeral container images. Ingress TLS rules
produce absolute URLs. HTTPRoute host/path observations use scheme-relative URLs
because the route object does not identify the parent listener's HTTP/TLS
scheme. An absent HTTPRoute CRD is an empty optional source; other read errors
are propagated to the sync boundary.

The composition root activates this adapter only with
`BALEMOH_KUBERNETES_ENABLED=true`, a stable source ID, and a namespace. It uses
`rest.InClusterConfig` and therefore consumes the pod ServiceAccount rather than
reading a host kubeconfig or socket.

## Docker-compatible container adapter

`internal/adapters/container` implements `catalog.Discoverer` over the
Docker-compatible `containers/json` API. It uses the configured socket or
endpoint and lists running containers without inspecting, starting, stopping,
or mutating them. Docker Engine and Podman can use the same adapter when their
compatible API socket is configured.

Each container becomes a candidate with its stable host-scoped source ID,
runtime name, image reference, state, Compose metadata when present, and only
the host-published ports returned by the runtime. A binding becomes a generic
`container.port` endpoint with the public port, protocol, and a `tcp://IP:port`
or `udp://IP:port` observation. An empty runtime host IP is normalized to
`0.0.0.0` to make an all-interface binding visible; no HTTP or HTTPS scheme is
guessed.

Containers carrying the Compose project and service labels are grouped into a
second `compose-service` candidate. The aggregate deduplicates images and
published bindings while retaining the project, service, Compose files,
working directory, and running container count. No Traefik informer or other
hostname extension is part of this adapter.

The composition root enables it only with
`BALEMOH_CONTAINER_ENABLED=true`, a stable
`BALEMOH_CONTAINER_SOURCE_ID`, and a Docker-compatible
`BALEMOH_CONTAINER_HOST` endpoint. The adapter performs read-only list calls,
but access to a container socket remains a privileged host capability.

## Composition

`cmd/balemoh` keeps the current startup sequence and adds:

1. sqlc queries from the migrated database;
2. SQLite catalog store;
3. catalog service with zero discoverers by default, or the explicitly enabled
   in-cluster Kubernetes and/or Docker-compatible container discoverers;
4. HTTP handler receiving health and catalog use cases;
5. generated mux.

The empty registry remains the default for a plain local process. Future
adapters are added at the composition root, one read-only source at a time.

## Verification

Required gates for this slice:

- `go generate ./...` leaves generated API and sqlc output stable;
- domain tests cover stable identity, default display name, validation, and idempotent pin behavior;
- application tests cover sync/upsert and discoverer error propagation;
- SQLite tests cover migration version `3`, restart, candidate upsert, image/endpoint replacement, pin preservation, and homepage filtering;
- Kubernetes fake-client tests cover route/Ingress-to-Service-to-Pod resolution, Service fallback, external Services, Pod image extraction, orphan-Pod filtering, namespace filtering, optional HTTPRoute CRD, and permission errors;
- container fake-client tests cover running-container selection, image and Compose service aggregation, published host ports, host IPs, wildcard bindings, Podman Compose labels, and daemon errors;
- DevSpace artifacts cover namespace-scoped RBAC and local KinD/vind setup without creating a cluster during repository verification;
- HTTP tests cover list, pin, unpin, homepage, sync, 404, 503, and generic error bodies;
- `go test ./...`, `go vet ./...`, `go test -race ./...`, and `git diff --check` pass;
- CGO-disabled test/build remains valid.

Acceptance means the API, persistence, Kubernetes and container adapters, and
the local Kubernetes development path are ready for the next
reconciliation/extension work. It does
not authorize merge, release, deployment outside the explicitly configured local
DevSpace workflow, or publication.
