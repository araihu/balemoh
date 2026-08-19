package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
)

func newCatalogTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func catalogTestCandidate(name string, observedAt time.Time) catalog.Candidate {
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: "docker", ID: "host-1"},
		catalog.ResourceRef{Kind: "container", Name: name},
		observedAt,
	)
	candidate.Description = "discovered service"
	candidate.Metadata = map[string]string{"team": "infra"}
	candidate.Images = []string{"grafana/grafana:latest"}
	candidate.Endpoints = []catalog.Endpoint{
		{Name: "web", URL: "https://" + name + ".example.test/", Port: 443, Protocol: "https", Provenance: "traefik"},
		{Name: "metrics", Port: 9090, Protocol: "http", Provenance: "docker.port"},
	}
	return candidate
}

func TestCatalogStoreUpsertReplacesObservationAndPreservesPin(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	firstObservedAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	first := catalogTestCandidate("grafana", firstObservedAt)

	if err := store.Upsert(context.Background(), first); err != nil {
		t.Fatalf("Upsert() first error = %v", err)
	}
	if _, err := store.SetPinned(context.Background(), first.ID, true); err != nil {
		t.Fatalf("SetPinned() error = %v", err)
	}

	second := catalogTestCandidate("grafana", firstObservedAt.Add(time.Hour))
	second.Description = "updated service"
	second.Metadata = map[string]string{"team": "platform"}
	second.Images = []string{"grafana/grafana:11"}
	second.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://grafana.example.test/updated", Port: 443, Protocol: "https", Provenance: "kubernetes.httproute"}}
	if err := store.Upsert(context.Background(), second); err != nil {
		t.Fatalf("Upsert() second error = %v", err)
	}

	services, err := store.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("List() length = %d, want 1", len(services))
	}
	got := services[0]
	if got.Description != "updated service" || got.Metadata["team"] != "platform" {
		t.Fatalf("updated candidate = %#v, want latest observation", got)
	}
	if len(got.Images) != 1 || got.Images[0] != "grafana/grafana:11" {
		t.Fatalf("images = %#v, want latest observation", got.Images)
	}
	if got.PinnedAt == nil {
		t.Fatal("PinnedAt is nil after observation upsert")
	}
	if len(got.Endpoints) != 1 || got.Endpoints[0].URL != "https://grafana.example.test/updated" {
		t.Fatalf("endpoints = %#v, want replaced endpoint set", got.Endpoints)
	}

	homepage, err := store.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List(pinned) error = %v", err)
	}
	if len(homepage) != 1 || homepage[0].ID != first.ID {
		t.Fatalf("homepage = %#v, want pinned candidate", homepage)
	}
}

func TestCatalogStorePinUnknownCandidateReturnsNotFound(t *testing.T) {
	store := NewCatalogStore(newCatalogTestDatabase(t))

	_, err := store.SetPinned(context.Background(), "missing", true)
	if !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("SetPinned() error = %v, want %v", err, catalog.ErrNotFound)
	}
}

func TestCatalogStorePinIsIdempotent(t *testing.T) {
	store := NewCatalogStore(newCatalogTestDatabase(t))
	candidate := catalogTestCandidate("grafana", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if err := store.Upsert(context.Background(), candidate); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	first, err := store.SetPinned(context.Background(), candidate.ID, true)
	if err != nil {
		t.Fatalf("first SetPinned() error = %v", err)
	}
	second, err := store.SetPinned(context.Background(), candidate.ID, true)
	if err != nil {
		t.Fatalf("second SetPinned() error = %v", err)
	}
	if first.PinnedAt == nil || second.PinnedAt == nil || !first.PinnedAt.Equal(*second.PinnedAt) {
		t.Fatalf("pin timestamps = %v and %v, want unchanged timestamp", first.PinnedAt, second.PinnedAt)
	}

	first, err = store.SetPinned(context.Background(), candidate.ID, false)
	if err != nil {
		t.Fatalf("first Unpin() error = %v", err)
	}
	second, err = store.SetPinned(context.Background(), candidate.ID, false)
	if err != nil {
		t.Fatalf("second Unpin() error = %v", err)
	}
	if first.PinnedAt != nil || second.PinnedAt != nil {
		t.Fatalf("unpin timestamps = %v and %v, want nil", first.PinnedAt, second.PinnedAt)
	}
}

func TestCatalogStoreIgnoresOlderObservation(t *testing.T) {
	store := NewCatalogStore(newCatalogTestDatabase(t))
	newer := catalogTestCandidate("grafana", time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC))
	newer.Metadata = map[string]string{"version": "new"}
	newer.Images = []string{"example/new:latest"}
	newer.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://new.example.test/", Port: 443, Protocol: "https", Provenance: "new"}}
	if err := store.Upsert(context.Background(), newer); err != nil {
		t.Fatalf("newer Upsert() error = %v", err)
	}

	older := catalogTestCandidate("grafana", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	older.Metadata = map[string]string{"version": "old"}
	older.Images = []string{"example/old:latest"}
	older.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://old.example.test/", Port: 443, Protocol: "https", Provenance: "old"}}
	if err := store.Upsert(context.Background(), older); err != nil {
		t.Fatalf("older Upsert() error = %v", err)
	}

	services, err := store.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 1 || services[0].Metadata["version"] != "new" || services[0].Endpoints[0].URL != "https://new.example.test/" || services[0].Images[0] != "example/new:latest" {
		t.Fatalf("stored observation = %#v, want newer observation", services)
	}
}

func TestCatalogServiceImportSnapshotReconcilesSQLiteSourceAndPreservesPins(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	source := catalog.SourceRef{Kind: "container", ID: "remote-host"}
	stale := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "gone"}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	pinned := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "pinned"}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	unrelated := catalogTestCandidate("unrelated", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	for _, candidate := range []catalog.Candidate{stale, pinned, unrelated} {
		if err := store.Upsert(context.Background(), candidate); err != nil {
			t.Fatalf("Upsert(%q) error = %v", candidate.Resource.Name, err)
		}
	}
	if _, err := store.SetPinned(context.Background(), pinned.ID, true); err != nil {
		t.Fatalf("SetPinned() error = %v", err)
	}

	current := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "current"}, time.Date(2026, 8, 17, 14, 0, 0, 0, time.UTC))
	service := catalog.NewService(store)
	result, err := service.ImportSnapshot(context.Background(), catalog.Snapshot{
		Source:     source,
		Candidates: []catalog.Candidate{current},
		ObservedAt: current.ObservedAt,
	})
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if result.Sources != 1 || result.Candidates != 1 {
		t.Fatalf("ImportSnapshot() result = %#v, want sources=1 candidates=1", result)
	}

	services, err := store.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	byID := make(map[string]catalog.Candidate, len(services))
	for _, service := range services {
		byID[service.ID] = service
	}
	if _, ok := byID[stale.ID]; ok {
		t.Fatal("stale unpinned candidate remains in SQLite")
	}
	if got, ok := byID[pinned.ID]; !ok || got.PinnedAt == nil {
		t.Fatalf("pinned candidate = %#v, want preserved pin", got)
	}
	if _, ok := byID[current.ID]; !ok {
		t.Fatal("current candidate missing from SQLite")
	}
	if _, ok := byID[unrelated.ID]; !ok {
		t.Fatal("unrelated source candidate was deleted")
	}
}

func TestCatalogServiceImportSnapshotIgnoresOlderCompleteSnapshot(t *testing.T) {
	store := NewCatalogStore(newCatalogTestDatabase(t))
	source := catalog.SourceRef{Kind: "container", ID: "remote-host"}
	newerAt := time.Date(2026, 8, 17, 14, 0, 0, 0, time.UTC)
	first := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "first"}, newerAt)
	second := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "second"}, newerAt)
	second.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://second.example.test/", Port: 443, Protocol: "https", Provenance: "snapshot"}}
	service := catalog.NewService(store)
	if _, err := service.ImportSnapshot(context.Background(), catalog.Snapshot{
		Source:     source,
		Candidates: []catalog.Candidate{first, second},
		ObservedAt: newerAt,
	}); err != nil {
		t.Fatalf("newer ImportSnapshot() error = %v", err)
	}

	olderAt := newerAt.Add(-time.Hour)
	if _, err := service.ImportSnapshot(context.Background(), catalog.Snapshot{
		Source:     source,
		Candidates: nil,
		ObservedAt: olderAt,
	}); err != nil {
		t.Fatalf("older ImportSnapshot() error = %v", err)
	}
	services, err := store.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	byID := make(map[string]catalog.Candidate, len(services))
	for _, candidate := range services {
		byID[candidate.ID] = candidate
	}
	if len(byID) != 2 {
		t.Fatalf("candidates after older snapshot = %#v, want both newer candidates", byID)
	}
	if len(byID[second.ID].Endpoints) != 1 {
		t.Fatalf("second endpoints after older snapshot = %#v, want preserved", byID[second.ID].Endpoints)
	}
}

func TestCatalogStoreSourceReplacementRollsBackOnEndpointFailure(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	source := catalog.SourceRef{Kind: "container", ID: "remote-host"}
	candidate := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "new"}, time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC))
	candidate.Endpoints = []catalog.Endpoint{{Name: "web", URL: "https://new.example.test/", Port: 443, Protocol: "https", Provenance: "fail"}}
	if _, err := db.ExecContext(context.Background(), `
		CREATE TRIGGER fail_snapshot_endpoint
		BEFORE INSERT ON service_endpoints
		WHEN NEW.provenance = 'fail'
		BEGIN
			SELECT RAISE(ABORT, 'forced endpoint failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	_, err := store.ReplaceSourceSnapshot(context.Background(), catalog.Snapshot{
		Source:     source,
		Candidates: []catalog.Candidate{candidate},
		ObservedAt: candidate.ObservedAt,
	})
	if err == nil {
		t.Fatal("ReplaceSourceSnapshot() error = nil, want rollback failure")
	}
	services, listErr := store.List(context.Background(), false)
	if listErr != nil {
		t.Fatalf("List() after rollback error = %v", listErr)
	}
	for _, service := range services {
		if service.ID == candidate.ID {
			t.Fatal("failed source replacement left the candidate committed")
		}
	}
	var watermarkCount int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM discovery_source_snapshots WHERE source_kind = ? AND source_id = ?`, source.Kind, source.ID).Scan(&watermarkCount); err != nil {
		t.Fatalf("query source watermark after rollback: %v", err)
	}
	if watermarkCount != 0 {
		t.Fatalf("source watermark count = %d, want 0 after rollback", watermarkCount)
	}
}

func TestCatalogStoreConcurrentSourceReplacementsDoNotMixSnapshots(t *testing.T) {
	store := NewCatalogStore(newCatalogTestDatabase(t))
	source := catalog.SourceRef{Kind: "container", ID: "remote-host"}
	observedAt := time.Date(2026, 8, 17, 16, 0, 0, 0, time.UTC)
	first := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "first"}, observedAt)
	second := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "container", Name: "second"}, observedAt)

	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, candidate := range []catalog.Candidate{first, second} {
		candidate := candidate
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := store.ReplaceSourceSnapshot(context.Background(), catalog.Snapshot{
				Source:     source,
				Candidates: []catalog.Candidate{candidate},
				ObservedAt: observedAt,
			})
			errorsCh <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent ReplaceSourceSnapshot() error = %v", err)
		}
	}

	services, err := store.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() after concurrent replacements error = %v", err)
	}
	if len(services) != 1 || (services[0].ID != first.ID && services[0].ID != second.ID) {
		t.Fatalf("concurrent replacement result = %#v, want exactly one complete snapshot", services)
	}
}

func TestCatalogStoreSurvivesDatabaseRestart(t *testing.T) {
	databasePath := t.TempDir() + "/balemoh.db"
	db, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations() error = %v", err)
	}
	candidate := catalogTestCandidate("whoami", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if err := NewCatalogStore(db).Upsert(context.Background(), candidate); err != nil {
		db.Close()
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() restart error = %v", err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() restart error = %v", err)
	}
	services, err := NewCatalogStore(db).List(context.Background(), false)
	if err != nil {
		t.Fatalf("List() after restart error = %v", err)
	}
	if len(services) != 1 || services[0].ID != candidate.ID {
		t.Fatalf("services after restart = %#v, want candidate %q", services, candidate.ID)
	}
}
