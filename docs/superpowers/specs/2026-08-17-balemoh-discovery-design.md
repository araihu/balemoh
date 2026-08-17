# Balemoh Discovery and Homepage Catalog Design

**Status:** Proposed implementation design; architecture approved in chat, pending written-spec review.

## Goal

Add the first domain slice behind Balemoh's homelab homepage: discoverable service candidates, a persistent staging area, and explicit pin/unpin decisions that control the homepage view.

The slice must make Docker and Kubernetes adapters possible without coupling the application core to either platform. It must also preserve uncertainty: a discovered resource may exist without a known hostname, and a port observation must not be presented as an exact route.

## Context

Balemoh already has an API-first Go foundation:

- OpenAPI is the HTTP contract authority.
- `oapi-codegen` generates the `net/http` server boundary.
- application packages expose inward-facing ports.
- SQLite/sqlc provide local persistence.
- embedded `golang-migrate` migrations run during startup.

The current API only exposes `/healthz`. This design adds the first business resource without changing the health composition or introducing a platform client.

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

- Docker socket access, Docker API client, or container mutation;
- Kubernetes client, RBAC setup, or cluster mutation;
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

The first API does not invent hostnames. A future Traefik or HTTPRoute adapter can provide an exact URL and evidence such as `traefik` or `kubernetes.httproute`; a Docker port-only observation remains visibly incomplete until an extension resolves it.

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

List responses use an object containing `services`, so pagination and additional catalog metadata can be added without changing the top-level response shape. Candidate JSON includes source/resource references, metadata, endpoint observations, observation time, and pin state.

HTTP behavior:

- successful reads and mutations return `200` JSON;
- unknown candidate IDs return `404` with the existing `ErrorResponse` shape;
- storage or discovery failures return `503` with a generic message;
- platform/database error details never reach the client.

## Persistence

Migration `000002_catalog` adds:

```text
discovered_services
  id, source_kind, source_id, resource_kind, resource_namespace,
  resource_name, display_name, description, metadata_json,
  observed_at, pinned_at, created_at, updated_at

service_endpoints
  id, service_id, name, url, port, protocol, provenance
```

`service_endpoints.service_id` references `discovered_services.id` with cascading delete. Upsert runs in one transaction: write the candidate, replace its endpoint rows, and commit. Pin state is updated separately and survives candidate upserts.

The storage adapter converts timestamps to RFC3339Nano strings and metadata to JSON. Invalid persisted metadata is returned as an internal error rather than silently discarded.

## Composition

`cmd/balemoh` keeps the current startup sequence and adds:

1. sqlc queries from the migrated database;
2. SQLite catalog store;
3. catalog service with an empty discoverer registry;
4. HTTP handler receiving health and catalog use cases;
5. generated mux.

The empty registry is intentional. It makes the API and sync lifecycle available now without pretending that a platform connection exists. Future adapters are added at the composition root, one read-only source at a time.

## Verification

Required gates for this slice:

- `go generate ./...` leaves generated API and sqlc output stable;
- domain tests cover stable identity, default display name, validation, and idempotent pin behavior;
- application tests cover sync/upsert and discoverer error propagation;
- SQLite tests cover migration version `2`, restart, candidate upsert, endpoint replacement, pin preservation, and homepage filtering;
- HTTP tests cover list, pin, unpin, homepage, sync, 404, 503, and generic error bodies;
- `go test ./...`, `go vet ./...`, `go test -race ./...`, and `git diff --check` pass;
- CGO-disabled test/build remains valid.

Acceptance means the API and persistent domain primitives are ready for a real adapter. It does not mean Docker or Kubernetes discovery is implemented.

