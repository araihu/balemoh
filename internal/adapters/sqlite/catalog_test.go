package sqlite

import (
	"context"
	"database/sql"
	"errors"
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
