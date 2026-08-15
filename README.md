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

Run the service:

```sh
go run ./cmd/balemoh
curl --fail http://127.0.0.1:8080/healthz
```

Defaults:

- `BALEMOH_HTTP_ADDR=:8080`
- `BALEMOH_DATABASE_PATH=./data/balemoh.db`

Runtime configuration is parsed once from environment variables before database
startup and migrations. See the [generated environment documentation](internal/config/environment.md)
for the complete configuration reference.
