# Balemoh Discovery Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the first API-first catalog slice for discovered homelab services, persistent staging, explicit pin/unpin decisions, and a source-agnostic discovery port.

**Architecture:** `internal/application/catalog` owns candidate identity, validation, catalog use cases, and `Discoverer`/`CatalogStore` ports. The SQLite adapter owns sqlc persistence and transactional endpoint replacement. The HTTP adapter translates generated OpenAPI types; `cmd/balemoh` composes the catalog with an empty discoverer registry so Docker/Kubernetes adapters can be added without changing the core.

**Tech Stack:** Go 1.26, `net/http`, OpenAPI 3.0.3, oapi-codegen `std-http-server`, SQLite/sqlc, embedded golang-migrate migrations, standard library JSON/time/URL validation.

**Spec:** `docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md`

## Global Constraints

- Keep OpenAPI as HTTP contract authority and regenerate committed output with `go generate ./...`.
- Keep Docker, Kubernetes, Traefik, RBAC, socket, and platform-client dependencies out of `internal/application/catalog`.
- Keep candidate discovery read-only; this slice must not mutate external homelab resources.
- Candidate identity is derived from source kind, source stable ID, resource kind, namespace, and resource name.
- Candidate upsert replaces observed metadata/endpoints but preserves existing `pinned_at`.
- Empty endpoint URL means no exact hostname is known; never infer a hostname from a port.
- Persist endpoint replacement and candidate upsert in one SQLite transaction.
- Map unknown candidates to HTTP 404 and internal/platform/database failures to generic HTTP 503 responses.
- Do not add authentication, background jobs, UI, deployment, release, push, merge, or cleanup work.
- Use TDD for handwritten production code: write a focused failing test, verify the expected failure, implement the smallest passing behavior, then refactor while green.
- Generated API and sqlc files are outputs; modify their source contract/query, regenerate, and never hand-edit generated output.

## File Map

- Modify `api/openapi.yaml`: add discovery sync, staging, pin/unpin, and homepage paths plus schemas.
- Modify `go.mod`, `go.sum`: add the generated HTTP runtime required by path-parameter routes.
- Modify `internal/adapters/http/health.go`: extend handler composition to include the catalog use case while preserving `/healthz` behavior.
- Modify `internal/adapters/http/health_test.go`: supply a catalog fake and preserve health error-sanitization coverage.
- Create `internal/application/catalog/model.go`: source/resource/endpoint/candidate types, stable identity, normalization, validation, and `ErrNotFound`.
- Create `internal/application/catalog/service.go`: `CatalogStore`, `Discoverer`, `UseCase`, sync report, and catalog service methods.
- Create `internal/application/catalog/model_test.go`: stable identity, default display name, validation, endpoint constraints, and normalization tests.
- Create `internal/application/catalog/service_test.go`: staging/homepage delegation, pin/unpin, sync upsert, and discoverer failure tests.
- Create `migrations/000002_catalog.up.sql`: `discovered_services`, `service_endpoints`, indexes, and foreign key.
- Create `migrations/000002_catalog.down.sql`: reverse catalog tables in dependency order.
- Modify `internal/storage/queries/catalog.sql`: sqlc queries for upsert, list, get, endpoint replacement, and pin state.
- Modify `internal/storage/sqlc/*.go`: regenerated sqlc models/query methods.
- Create `internal/adapters/sqlite/catalog.go`: transactional `CatalogStore` implementation and row conversion.
- Create `internal/adapters/sqlite/catalog_test.go`: catalog persistence integration tests on temporary SQLite databases.
- Create `internal/adapters/http/catalog.go`: generated API conversion and staging/homepage/sync handlers.
- Create `internal/adapters/http/catalog_test.go`: route and error behavior tests through the generated mux.
- Modify `cmd/balemoh/main.go`: compose SQLite catalog store and catalog service with no discoverers.
- Modify `cmd/balemoh/main_test.go`: verify catalog composition remains available after migration.
- Modify `README.md`: document API routes and source registry behavior.
- Create `docs/superpowers/plans/2026-08-17-balemoh-discovery-catalog.md`: this executable plan.

---

### Task 1: Implement catalog domain and application ports

**Files:**
- Create: `internal/application/catalog/model.go`
- Create: `internal/application/catalog/model_test.go`
- Create: `internal/application/catalog/service.go`
- Create: `internal/application/catalog/service_test.go`

**Interfaces:**
- Produces `SourceRef`, `ResourceRef`, `Endpoint`, `Candidate`, `StableID`, `NewCandidate`, `CatalogStore`, `Discoverer`, `UseCase`, `SyncResult`, and `Service`.
- `UseCase` methods are:

```go
type UseCase interface {
    ListStaging(context.Context) ([]Candidate, error)
    ListHomepage(context.Context) ([]Candidate, error)
    Pin(context.Context, string) (Candidate, error)
    Unpin(context.Context, string) (Candidate, error)
    Sync(context.Context) (SyncResult, error)
}
```

- `CatalogStore` methods are:

```go
type CatalogStore interface {
    Upsert(context.Context, Candidate) error
    List(context.Context, bool) ([]Candidate, error)
    SetPinned(context.Context, string, bool) (Candidate, error)
}
```

**Steps:**

- [x] **Step 1: Write failing model tests.** Test that equal source/resource identity produces equal IDs, a changed namespace/source changes the ID, blank display name defaults to resource name, empty metadata/endpoints normalize to non-nil collections, blank identity is rejected, URL parsing rejects malformed non-empty URLs, and ports outside `0..65535` are rejected.
- [x] **Step 2: Run model tests to verify failure.** Run `go test ./internal/application/catalog -run 'TestStableID|TestNewCandidate|TestCandidateValidate' -count=1`. Expected: package or symbols are missing; no production implementation exists yet.
- [x] **Step 3: Implement minimal model behavior.** Use canonical NUL-separated identity parts and SHA-256 hex output. Normalize trimmed identity fields, default display name, initialize maps/slices, validate non-empty source/resource identity, require a non-zero observation time, validate endpoint URLs with `net/url`, and accept port `0` as absent.
- [x] **Step 4: Run model tests to verify green.** Run the same focused command and then `go test ./internal/application/catalog -count=1`. Expected: all model tests pass.
- [x] **Step 5: Write failing service tests.** Use a real in-memory fake store and fake discoverers. Test `ListStaging` calls `List(false)`, `ListHomepage` calls `List(true)`, `Pin` and `Unpin` request the desired state, `Sync` validates/upserts candidates and counts sources/candidates, and a discoverer error is returned without exposing a platform-specific HTTP concern.
- [x] **Step 6: Run service tests to verify failure.** Run `go test ./internal/application/catalog -run 'TestService' -count=1`. Expected: service symbols are missing or behavior assertions fail.
- [x] **Step 7: Implement minimal service behavior.** Store the injected `CatalogStore` and ordered discoverers. Delegate list/pin operations. Sync each discoverer in order, validate each candidate before upsert, count configured sources and successful candidates, and wrap source errors with the discoverer name.
- [x] **Step 8: Run all catalog tests.** Run `go test ./internal/application/catalog -count=1`. Expected: model and service tests pass with no external dependencies.

### Task 2: Extend OpenAPI and regenerate the HTTP boundary

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `internal/api/generated/balemoh.gen.go` (generated)

**Interfaces:**
- Produces generated methods `GetStagingServices`, `PinStagingService`, `UnpinStagingService`, `GetHomepageServices`, and `SyncDiscovery`.
- Produces generated schemas `SourceRef`, `ResourceRef`, `ServiceEndpoint`, `ServiceCandidate`, `ServiceListResponse`, and `DiscoverySyncResponse`.

**Steps:**

- [x] **Step 1: Add the contract.** Define the five `/api/v1` routes with `200`, `404` where applicable, and `503` responses. Define candidate fields exactly as the domain projection: `id`, `source`, `resource`, `displayName`, `description`, `metadata`, `endpoints`, `observedAt`, `pinned`, and nullable `pinnedAt`. Define endpoint URL as a string that may be empty, port default `0`, and `provenance` as a string.
- [x] **Step 2: Regenerate and inspect the boundary.** Run `go generate ./internal/api`, then `gofmt -w internal/api/generated/balemoh.gen.go`. Verify with `rg -n 'GetStagingServices|PinStagingService|UnpinStagingService|GetHomepageServices|SyncDiscovery' internal/api/generated/balemoh.gen.go`. Expected: all methods and schemas exist.
- [x] **Step 3: Add the generated runtime dependency if required.** Run `go get github.com/oapi-codegen/runtime@v1.7.0` when generated code imports `github.com/oapi-codegen/runtime`; retain the pinned module in `go.mod`/`go.sum`.
- [x] **Step 4: Compile generated output.** Run `go test ./internal/api/... -count=1`. Expected: generated package compiles before handwritten handlers exist.

### Task 3: Add catalog migration and sqlc queries

**Files:**
- Create: `migrations/000002_catalog.up.sql`
- Create: `migrations/000002_catalog.down.sql`
- Modify: `internal/storage/queries/catalog.sql`
- Modify: `internal/storage/sqlc/*.go` (generated)

**Interfaces:**
- Produces generated row/parameter types for candidate and endpoint persistence.
- `discovered_services.id` is the stable candidate ID; `service_endpoints.service_id` references it with `ON DELETE CASCADE`.

**Steps:**

- [x] **Step 1: Write migration test first.** Extend migration integration coverage to expect schema version `2`, query both catalog tables, and confirm the foreign-key relationship is present through a failed orphan endpoint insert.
- [x] **Step 2: Run the focused migration test to verify failure.** Run `go test ./internal/adapters/sqlite -run 'TestRunMigrations|TestCatalogSchema' -count=1`. Expected: current migration remains at version `1` and the new schema assertion fails.
- [x] **Step 3: Add migration SQL.** Create `discovered_services` with source/resource identity, display fields, JSON metadata, observation/pin timestamps, and indexes. Create `service_endpoints` with URL/port/protocol/provenance and a cascading foreign key. Down migration drops endpoints before services.
- [x] **Step 4: Add sqlc queries.** Add named queries for candidate upsert preserving `pinned_at`, candidate list (all and pinned), candidate get, endpoint delete, endpoint insert, pin update, and unpin update. Keep timestamp and metadata values as strings at the storage boundary.
- [x] **Step 5: Regenerate storage output.** Run `go generate ./internal/storage` and format generated files. Inspect generated signatures before writing the adapter.
- [x] **Step 6: Run migration and storage package tests.** Run `go test ./internal/adapters/sqlite ./internal/storage/... -count=1`. Expected: migration version 2 and generated storage compile; adapter behavior tests remain pending until Task 4.

### Task 4: Implement SQLite catalog adapter

**Files:**
- Create: `internal/adapters/sqlite/catalog.go`
- Create: `internal/adapters/sqlite/catalog_test.go`

**Interfaces:**
- Implements `catalog.CatalogStore` with `NewCatalogStore(*sql.DB) catalog.CatalogStore`.
- Converts database rows to validated `catalog.Candidate` values and maps `sql.ErrNoRows` to `catalog.ErrNotFound`.

**Steps:**

- [x] **Step 1: Write failing persistence tests.** Start a temporary SQLite file, run migrations, construct the store, and test: upsert/list returns a candidate with metadata and endpoints; a second upsert replaces endpoints and metadata; pin survives the second upsert; homepage list includes only pinned rows; unknown pin returns `catalog.ErrNotFound`; close/reopen keeps data.
- [x] **Step 2: Run persistence tests to verify failure.** Run `go test ./internal/adapters/sqlite -run 'TestCatalogStore' -count=1`. Expected: constructor/methods are missing or tests fail against absent schema.
- [x] **Step 3: Implement row conversion.** Marshal metadata with `encoding/json`, format UTC timestamps as RFC3339Nano, parse stored metadata and nullable pin timestamps, load endpoint rows for each candidate, and return a descriptive internal error for corrupt stored JSON/timestamps.
- [x] **Step 4: Implement transactional upsert.** Begin a context transaction, run candidate upsert, delete all old endpoint rows, insert current endpoint rows, and commit. Roll back on every error. Never write `pinned_at` in the upsert statement.
- [x] **Step 5: Implement list and pin operations.** List all or pinned candidates through the generated queries, order deterministically by display name and ID, and set/clear pin timestamps with separate updates followed by a candidate read.
- [x] **Step 6: Run persistence tests to verify green.** Run `go test ./internal/adapters/sqlite -run 'TestCatalogStore' -count=1` and then `go test ./internal/adapters/sqlite -count=1`. Expected: all storage and migration tests pass.

### Task 5: Implement HTTP catalog adapter

**Files:**
- Modify: `internal/adapters/http/health.go`
- Create: `internal/adapters/http/catalog.go`
- Modify: `internal/adapters/http/health_test.go`
- Create: `internal/adapters/http/catalog_test.go`

**Interfaces:**
- `NewHandler(health.Checker, catalog.UseCase) generated.ServerInterface` composes health and catalog behavior.
- HTTP conversion maps domain candidate fields to generated API models without exposing storage errors or internal error text.

**Steps:**

- [x] **Step 1: Write failing HTTP tests.** Through `generated.HandlerFromMux`, test staging list `200`, homepage list `200`, pin/unpin `200`, sync `200`, unknown pin `404`, store failure `503`, and sync failure `503`. Assert JSON content type, candidate projection, and generic error bodies. Update health tests to pass the catalog fake while preserving `/healthz` assertions.
- [x] **Step 2: Run HTTP tests to verify failure.** Run `go test ./internal/adapters/http -count=1`. Expected: generated methods are not implemented by the handler and the new tests fail to compile or route.
- [x] **Step 3: Extend the handler dependency.** Add a catalog use-case field to the existing handler and update `NewHandler` plus test construction. Keep health behavior unchanged.
- [x] **Step 4: Implement projection helpers.** Convert source/resource/endpoint/candidate domain values to generated models; convert nil pin timestamps to the generated nullable representation; return empty metadata/endpoints collections.
- [x] **Step 5: Implement catalog handlers.** Delegate list, pin, unpin, and sync operations. Map `catalog.ErrNotFound` to `404`, all other errors to `503`, and use generic `ErrorResponse` messages.
- [x] **Step 6: Run HTTP tests to verify green.** Run `go test ./internal/adapters/http -count=1`. Expected: all health and catalog route tests pass.

### Task 6: Compose service and update developer documentation

**Files:**
- Modify: `cmd/balemoh/main.go`
- Modify: `cmd/balemoh/main_test.go`
- Modify: `README.md`

**Interfaces:**
- Composition creates `sqlite.NewCatalogStore(db)`, `catalog.NewService(store)`, and `httpadapter.NewHandler(healthService, catalogService)` with zero discoverers.
- Existing startup migration, health, graceful shutdown, and database lifecycle remain unchanged.

**Steps:**

- [x] **Step 1: Write failing composition test.** Construct a server with a temporary database and assert the generated mux serves `GET /api/v1/staging/services` as JSON `200` with an empty `services` collection after migrations.
- [x] **Step 2: Run composition test to verify failure.** Run `go test ./cmd/balemoh -run 'TestConstructServerCatalog' -count=1`. Expected: the route is absent or server construction still uses the old handler signature.
- [x] **Step 3: Wire the catalog.** Create the SQLite store and catalog service after migrations, pass both use cases into the HTTP adapter, and preserve the existing return values and cleanup behavior.
- [x] **Step 4: Update README.** Document the new routes, explain that sync has no sources until a Docker/Kubernetes adapter is configured, and link the roadmap/spec while retaining current run and health commands.
- [x] **Step 5: Run composition tests.** Run `go test ./cmd/balemoh -count=1`. Expected: startup and catalog route tests pass.

### Task 7: Regenerate, review, verify, and commit feature branch

**Files:**
- Modify: generated API/sqlc outputs only through generation.
- Review: all changed files against `docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md` and `docs/roadmap.md`.

**Steps:**

- [x] **Step 1: Regenerate all artifacts.** Run `go generate ./...`; run `gofmt` on handwritten Go files; verify generated output is stable with a second `go generate ./...` and `git diff --check`.
- [x] **Step 2: Run focused gates.** Run `go test ./internal/application/catalog ./internal/adapters/sqlite ./internal/adapters/http ./cmd/balemoh -count=1` and inspect all failures.
- [x] **Step 3: Run full gates.** Run `CGO_ENABLED=0 go test ./... -count=1`, `CGO_ENABLED=0 go build -o /tmp/balemoh-discovery ./cmd/balemoh`, `go vet ./...`, and `go test -race ./... -count=1`.
- [x] **Step 4: Inspect exact diff.** Run `git status --short`, `git diff --stat`, and `git diff --check`; confirm no generated drift, local database, binary, secret, or unrelated worktree files are included.
- [x] **Step 5: Commit implementation.** Run:

```bash
git add api go.mod go.sum internal/application/catalog internal/adapters/http internal/adapters/sqlite internal/api/generated/balemoh.gen.go internal/storage/queries internal/storage/sqlc migrations cmd/balemoh/main.go cmd/balemoh/main_test.go README.md docs/superpowers/plans/2026-08-17-balemoh-discovery-catalog.md
git commit -m "feat: add discovery catalog primitives"
```

- [ ] **Step 6: Review the committed feature branch.** Compare the implementation commit against its parent, re-read the spec requirements, check error leakage, transaction boundaries, pin preservation, stable identity, generated artifacts, and test coverage. Record findings before any final completion claim.

## Self-review checklist

- Spec domain contracts are implemented in Task 1.
- Spec API routes and schemas are implemented in Task 2.
- Spec migration and sqlc persistence are implemented in Task 3 and Task 4.
- Spec HTTP mapping and generic errors are implemented in Task 5.
- Spec composition with an empty registry is implemented in Task 6.
- Spec verification gates and explicit acceptance boundary are implemented in Task 7.
- No production code is added before its focused test cycle.
- No task contains unfinished placeholder markers or unspecified behavior.
