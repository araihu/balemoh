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

Discovery sync currently has an empty source registry, so it returns zero sources
until a read-only Docker, Kubernetes, or extension adapter is composed into the
application. Discovered candidates remain in staging until the user explicitly
pins them. A port observation is not treated as an exact hostname; endpoint
provenance records which adapter supplied the evidence.

See the [product roadmap](docs/roadmap.md) and [discovery design](docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md)
for the adapter sequence and scope boundaries.
