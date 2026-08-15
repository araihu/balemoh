# Balemoh Foundation Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

Goal: Build the smallest runnable Balemoh foundation: API-first Go HTTP service, embedded SQLite migrations, sqlc storage, typed configuration, and a database-backed /healthz endpoint.

Architecture: api/openapi.yaml remains authoritative. oapi-codegen produces the standard net/http boundary. The application health service depends on a narrow Pinger port. The SQLite adapter owns database/sql, embedded golang-migrate, and generated sqlc code. cmd/balemoh composes everything and owns graceful shutdown.

Tech stack: Go 1.26, net/http, oapi-codegen std-http-server, sqlc SQLite engine, github.com/golang-migrate/migrate/v4, modernc.org/sqlite, github.com/caarlos0/env/v11, and github.com/g4s8/envdoc.

Spec: docs/superpowers/specs/2026-08-15-balemoh-foundation-design.md

## Global Constraints

- Module path: github.com/araihu/balemoh.
- Go module directive: 1.26; local toolchain: go1.26.6.
- Runtime pins: migrate/v4 v4.19.1, modernc.org/sqlite v1.54.0, env/v11 v11.4.1.
- Tool pins: oapi-codegen v2.7.2, sqlc v1.31.1, envdoc v1.10.0.
- Manage tools with Go 1.24+ tool directives and invoke them through go tool.
- Use modernc.org/sqlite; prove CGO_ENABLED=0 builds.
- Import golang-migrate as a library; never invoke its CLI.
- Embed root migrations/*.sql through migrations/embed.go.
- Commit generated API, sqlc, and environment documentation output.
- Keep HTTP, SQLite, and generated transport dependencies out of internal/application.
- Read environment once at startup; never log secrets.
- No business CRUD, auth, deployment, release, push, or cleanup work.
- Begin implementation in an isolated worktree using superpowers:using-git-worktrees.
- End each implementation task with focused verification and a local Git commit; final verification only records evidence.

## File Map

- go.mod, go.sum: module, runtime dependencies, and Go tools.
- .gitignore: Go artifacts, data/secrets, editors, macOS metadata.
- README.md: generation, test, run, and health commands.
- api/openapi.yaml: /healthz contract.
- api/oapi-codegen.yaml: generated models and standard HTTP server.
- internal/api/generate.go: API go:generate directive.
- internal/api/generated/balemoh.gen.go: committed API output.
- sqlc.yaml: SQLite query generation.
- internal/storage/generate.go: sqlc go:generate directive.
- internal/storage/queries/health.sql: Ping query.
- internal/storage/sqlc/health.sql.go: committed sqlc output.
- migrations/embed.go and migrations/*.sql: embedded migration filesystem and bootstrap migration.
- internal/config/options.go, options_test.go, generate.go, environment.md: typed config, tests, generation, docs.
- internal/application/health/service.go and service_test.go: use case, ports, tests.
- internal/adapters/sqlite/*.go: connection, migrations, sqlc Pinger, tests.
- internal/adapters/http/*.go: generated server implementation and tests.
- cmd/balemoh/main.go: composition root and server lifecycle.

## Stable Interfaces

Use these signatures across tasks:

    type Pinger interface {
        Ping(context.Context) error
    }

    type Checker interface {
        Check(context.Context) error
    }

    func NewService(Pinger) *Service
    func (Service) Check(context.Context) error

    type Options struct {
        HTTPAddr string
        DatabasePath string
    }

Options tags must be exactly env=BALEMOH_HTTP_ADDR with default :8080 and env=BALEMOH_DATABASE_PATH with default ./data/balemoh.db.

    func Load() (Options, error)
    func ParseEnvironment(map[string]string) (Options, error)
    func (Options) Validate() error

    func Open(context.Context, string) (*sql.DB, error)
    func RunMigrations(*sql.DB) error
    func NewPinger(*sqlc.Queries) health.Pinger

Runtime flow:

    config.Load
      -> sqlite.Open
      -> sqlite.RunMigrations
      -> sqlc.New
      -> health.NewService(sqlite.NewPinger(...))
      -> httpadapter.NewHandler
      -> generated.HandlerFromMux
      -> http.Server

## Task 1: Module, tools, ignore rules, README

Files:
- Create go.mod, go.sum, .gitignore, README.md.

Produces: module identity, pinned dependencies/tools, and local developer commands.

- [ ] Initialize module.

    go mod init github.com/araihu/balemoh
    go mod edit -go=1.26

- [ ] Add runtime dependencies.

    go get github.com/caarlos0/env/v11@v11.4.1
    go get github.com/golang-migrate/migrate/v4@v4.19.1
    go get modernc.org/sqlite@v1.54.0

- [ ] Add Go tools.

    go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2
    go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
    go get -tool github.com/g4s8/envdoc@v1.10.0

Expected: go.mod contains module, go directive, require entries, and tool block.

- [ ] Create .gitignore containing Go binaries/tests/coverage, /data/, *.db, .env with .env.example exception, editor files, and macOS entries .DS_Store, .AppleDouble, .LSOverride, ._*, .Spotlight-V100, .Trashes, .DocumentRevisions-V100, .TemporaryItems, .VolumeIcon.icns, .com.apple.timemachine.donotpresent, and .fseventsd.

- [ ] Create README with these commands:

    go generate ./...
    go test ./...
    go vet ./...
    go run ./cmd/balemoh
    curl --fail http://127.0.0.1:8080/healthz

Document defaults BALEMOH_HTTP_ADDR=:8080 and BALEMOH_DATABASE_PATH=./data/balemoh.db. Link generated environment documentation when Task 4 creates it.

- [ ] Normalize and verify. Do not run go mod tidy before source imports exist; it would remove unreferenced runtime dependencies.

    go test ./...
    go vet ./...
    git diff --check

- [ ] Commit.

    git add go.mod go.sum .gitignore README.md
    git commit -m "chore: bootstrap balemoh module"

## Task 2: OpenAPI contract and generated HTTP boundary

Files:
- Create api/openapi.yaml, api/oapi-codegen.yaml, internal/api/generate.go.
- Generate internal/api/generated/balemoh.gen.go.

Produces: generated ServerInterface with GetHealthz, HealthResponse, ErrorResponse, and HandlerFromMux.

- [ ] Write api/openapi.yaml with OpenAPI 3.0.3, title Balemoh API, version 0.1.0, GET /healthz, operationId GetHealthz, 200 and 503 JSON responses, HealthResponse status enum ok, and ErrorResponse code/message strings.

- [ ] Write api/oapi-codegen.yaml:

    package: generated
    output: ../../internal/api/generated/balemoh.gen.go
    generate:
      models: true
      std-http-server: true
    output-options:
      skip-prune: true

- [ ] Write internal/api/generate.go:

    package api

    //go:generate go tool oapi-codegen -config ../../api/oapi-codegen.yaml ../../api/openapi.yaml

- [ ] Generate and compile.

    go generate ./internal/api
    gofmt -w internal/api/generated/balemoh.gen.go
    go test ./...

Expected: generated output contains the named server method/models and compiles.

- [ ] Commit.

    git add api internal/api/generated/balemoh.gen.go
    git commit -m "feat: define health API contract"

## Task 3: Embedded migration and sqlc generation

Files:
- Create sqlc.yaml, migrations/embed.go, migrations/000001_bootstrap.up.sql, migrations/000001_bootstrap.down.sql, internal/storage/queries/health.sql, internal/storage/generate.go.
- Generate internal/storage/sqlc/health.sql.go.

Produces: migrations.FS and sqlc.Queries.Ping(context.Context) returning integer and error.

- [ ] Create sqlc.yaml:

    version: '2'
    sql:
      - engine: sqlite
        schema: migrations
        queries: internal/storage/queries
        gen:
          go:
            package: sqlc
            out: internal/storage/sqlc
            sql_package: database/sql
            emit_json_tags: true

- [ ] Create both bootstrap migration files with exactly SELECT 1;.

- [ ] Create migrations/embed.go:

    package migrations

    import "embed"

    // FS contains migrations shipped with Balemoh.
    //go:embed *.sql
    var FS embed.FS

- [ ] Create internal/storage/queries/health.sql:

    -- name: Ping :one
    SELECT 1;

- [ ] Create internal/storage/generate.go:

    package storage

    //go:generate go tool sqlc generate -f ../../sqlc.yaml

- [ ] Generate and inspect:

    go generate ./internal/storage
    gofmt -w internal/storage/sqlc/health.sql.go
    go test ./...
    rg -n "func \\(.*\\) Ping\\(" internal/storage/sqlc/health.sql.go

Expected: generated Ping accepts context.Context and returns an integer/error.

- [ ] Commit.

    git add sqlc.yaml migrations internal/storage
    git commit -m "feat: add embedded migration and sqlc health query"

## Task 4: Typed environment config and docs

Files:
- Create internal/config/options.go, options_test.go, generate.go.
- Generate internal/config/environment.md.
- Modify README.md.

Produces: config.Options, Load, ParseEnvironment, and Validate.

- [ ] Write tests first for default values, explicit overrides, and whitespace-only values rejected.

    func TestParseEnvironmentUsesDefaults(t *testing.T) {
        got, err := ParseEnvironment(map[string]string{})
        if err != nil { t.Fatal(err) }
        if got.HTTPAddr != ":8080" { t.Fatalf("HTTPAddr = %q", got.HTTPAddr) }
        if got.DatabasePath != "./data/balemoh.db" { t.Fatalf("DatabasePath = %q", got.DatabasePath) }
    }

    func TestParseEnvironmentRejectsWhitespaceOnlyValues(t *testing.T) {
        _, err := ParseEnvironment(map[string]string{"BALEMOH_HTTP_ADDR": "   "})
        if err == nil { t.Fatal("expected validation error") }
    }

- [ ] Run RED check.

    go test ./internal/config -run TestParseEnvironment -count=1

Expected: FAIL because Options and ParseEnvironment are absent.

- [ ] Implement Options with the exact tags/defaults in Stable Interfaces. Parse with env.ParseAs and env.ParseAsWithOptions using an injected Environment map. Validate trimmed HTTPAddr and DatabasePath are non-empty.

- [ ] Create internal/config/generate.go:

    package config

    //go:generate go tool envdoc -output environment.md -files options.go -types Options

- [ ] Generate, format, test, and link README.

    go generate ./internal/config
    gofmt -w internal/config
    go test ./internal/config -count=1

Expected: environment.md documents exactly both BALEMOH variables and defaults.

- [ ] Commit.

    git add internal/config README.md
    git commit -m "feat: add typed runtime configuration"

## Task 5: Application health use case

Files:
- Create internal/application/health/service.go and service_test.go.

Produces: narrow Pinger and Checker ports plus Service.

- [ ] Write fake-Pinger tests for success and error propagation.

    type fakePinger struct { err error }
    func (f fakePinger) Ping(context.Context) error { return f.err }

    func TestServiceCheckReturnsNilWhenPingerSucceeds(t *testing.T) {
        service := NewService(fakePinger{})
        if err := service.Check(context.Background()); err != nil { t.Fatal(err) }
    }

    func TestServiceCheckReturnsPingerError(t *testing.T) {
        want := errors.New("database unavailable")
        service := NewService(fakePinger{err: want})
        if !errors.Is(service.Check(context.Background()), want) { t.Fatal("expected pinger error") }
    }

- [ ] Run RED.

    go test ./internal/application/health -run TestServiceCheck -count=1

Expected: FAIL because health package is absent.

- [ ] Implement Service:

    package health

    import "context"

    type Pinger interface { Ping(context.Context) error }
    type Checker interface { Check(context.Context) error }
    type Service struct { pinger Pinger }

    func NewService(pinger Pinger) *Service { return &Service{pinger: pinger} }
    func (s Service) Check(ctx context.Context) error { return s.pinger.Ping(ctx) }

    var _ Checker = Service{}

- [ ] Format, test, commit.

    gofmt -w internal/application/health
    go test ./internal/application/health -count=1
    git add internal/application/health
    git commit -m "feat: add health application service"

## Task 6: SQLite adapter and migrations

Files:
- Create internal/adapters/sqlite/connection.go, migrate.go, health.go, migrate_test.go, health_test.go.

Produces: Open, RunMigrations, and sqlc-backed health.Pinger.

- [ ] Write migration tests first. Use a temporary file, call Open, call RunMigrations twice, assert schema_migrations version 1 and dirty false. Add invalid fstest.MapFS with 000001_bad.up.sql containing NOT VALID SQL; and assert unexported runMigrations returns an error.

- [ ] Run RED.

    go test ./internal/adapters/sqlite -run TestRunMigrations -count=1

Expected: FAIL because adapter functions are absent.

- [ ] Implement Open with database/sql, blank import modernc.org/sqlite, driver name sqlite, one open/idle connection, PRAGMA foreign_keys = ON, PingContext, and close-on-error.

- [ ] Implement RunMigrations and runMigrations:

    func RunMigrations(db *sql.DB) error { return runMigrations(db, migrations.FS) }

    func runMigrations(db *sql.DB, files fs.FS) error {
        source, err := iofs.New(files, ".")
        if err != nil { return err }
        driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
        if err != nil { return err }
        migrator, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
        if err != nil { return err }
        defer func() { _, _ = migrator.Close() }()
        if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) { return err }
        return nil
    }

- [ ] Implement sqlc Pinger:

    type Pinger struct { queries *sqlc.Queries }
    func NewPinger(q *sqlc.Queries) health.Pinger { return Pinger{queries: q} }
    func (p Pinger) Ping(ctx context.Context) error {
        _, err := p.queries.Ping(ctx)
        return err
    }
    var _ health.Pinger = Pinger{}

- [ ] Add adapter test: open temporary DB, migrate, create sqlc.New(db), call NewPinger(...).Ping(context.Background()), expect nil.

- [ ] Format and verify.

    gofmt -w internal/adapters/sqlite
    go test ./internal/adapters/sqlite -count=1
    CGO_ENABLED=0 go test ./internal/adapters/sqlite -count=1

- [ ] Commit.

    git add internal/adapters/sqlite
    git commit -m "feat: add sqlite migration and health adapter"

## Task 7: Generated HTTP health adapter

Files:
- Create internal/adapters/http/health.go and health_test.go.

Produces: NewHandler(health.Checker) generated.ServerInterface.

- [ ] Write tests with a fake Checker. Mount generated.HandlerFromMux(NewHandler(fake), http.NewServeMux()). Assert GET /healthz returns 200, application/json, and HealthResponse status ok. Assert dependency error returns 503 and response excludes the underlying error string.

- [ ] Run RED.

    go test ./internal/adapters/http -run TestHealthz -count=1

Expected: FAIL because NewHandler is absent.

- [ ] Implement Handler with checker health.Checker, NewHandler returning generated.ServerInterface, GetHealthz, and one writeJSON helper. Success uses HealthResponse status ok. Failure uses ErrorResponse code unavailable and message service unavailable. Do not expose the dependency error. Assert generated.ServerInterface at compile time.

- [ ] Format, test, commit.

    gofmt -w internal/adapters/http
    go test ./internal/adapters/http -count=1
    git add internal/adapters/http
    git commit -m "feat: add generated health HTTP adapter"

## Task 8: Command wiring and lifecycle

Files:
- Create cmd/balemoh/main.go.
- Modify README.md.

Produces: go run ./cmd/balemoh serving GET /healthz.

- [ ] Implement composition order: config.Load, MkdirAll for database parent, sqlite.Open, deferred DB close, sqlite.RunMigrations, sqlc.New, health.NewService, sqlite.NewPinger, httpadapter.NewHandler, generated.HandlerFromMux, and http.Server.

- [ ] Add signal.NotifyContext for os.Interrupt and SIGTERM. Run ListenAndServe in a goroutine. On cancellation, call Shutdown with five-second timeout. Treat http.ErrServerClosed as success and return other server errors.

- [ ] Smoke-test using task-specific variables and loopback:

    database_dir=$(mktemp -d)
    BALEMOH_DATABASE_PATH="$database_dir/balemoh.db" BALEMOH_HTTP_ADDR="127.0.0.1:18080" go run ./cmd/balemoh &
    server_pid=$!
    trap 'kill "$server_pid" 2>/dev/null || true' EXIT
    curl --fail --silent --show-error http://127.0.0.1:18080/healthz

Expected: JSON contains status ok and the database contains migration bookkeeping.

- [ ] Update README with generation, test, vet, run, curl, automatic migration, and bootstrap migration behavior.

- [ ] Format, build, test, commit.

    gofmt -w cmd/balemoh/main.go
    go test ./...
    go build ./cmd/balemoh
    git add cmd/balemoh/main.go README.md
    git commit -m "feat: wire balemoh service startup"

## Task 9: Final generation and verification

- [ ] Regenerate and format.

    go mod tidy
    go generate ./...
    gofmt -w $(rg --files -g '*.go')

- [ ] Prove generated tree is clean.

    git diff --check
    git diff --exit-code

Expected: no diff after generation and formatting.

- [ ] Run pure-Go tests/build and vet.

    CGO_ENABLED=0 go test ./...
    CGO_ENABLED=0 go build ./cmd/balemoh
    go vet ./...

- [ ] Run race tests.

    go test -race ./...

- [ ] Record final identity/status.

    git status --short --branch
    git log -1 --format='%H%n%s'
    git diff --check

Expected: clean worktree, final commit recorded, no push or remote mutation.

## Spec Coverage Review

- Foundation scope and exclusions: Tasks 1-9 and Global Constraints.
- OpenAPI authority and generated standard server: Task 2.
- Hexagonal health ports and dependency direction: Tasks 5-7.
- SQLite and pure-Go driver: Task 6.
- sqlc query and committed output: Tasks 3 and 6.
- Embedded golang-migrate and reversible bootstrap: Tasks 3 and 6.
- Typed environment config and envdoc: Task 4.
- Go/macOS ignore rules: Task 1.
- Startup, graceful shutdown, and /healthz: Task 8.
- Generation, test, vet, race, CGO-disabled, and clean-tree evidence: Task 9.
