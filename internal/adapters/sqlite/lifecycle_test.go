package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
)

func TestServiceLifecyclePreservesSettingsAndGuardsDeletion(t *testing.T) {
	ctx := context.Background()
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	service := catalog.NewService(store)
	at := time.Now().UTC()
	c := catalogTestCandidate("lifecycle", at)
	apply := func(candidates []catalog.Candidate) {
		t.Helper()
		at = at.Add(time.Minute)
		_, err := store.ReplaceSourceSnapshot(ctx, catalog.Snapshot{Source: c.Source, ObservedAt: at, Candidates: candidates})
		if err != nil {
			t.Fatal(err)
		}
	}
	state := func(want string) {
		t.Helper()
		rows, err := service.ListStaging(ctx)
		if err != nil || len(rows) != 1 || rows[0].Status() != want {
			t.Fatalf("state: %v %v", rows, err)
		}
	}
	home := func(want int) {
		t.Helper()
		rows, err := service.ListHomepage(ctx)
		if err != nil || len(rows) != want {
			t.Fatalf("homepage length %d, want %d: %v", len(rows), want, err)
		}
	}
	action := func(name string) {
		t.Helper()
		if err := service.Lifecycle(ctx, c.ID, name); err != nil {
			t.Fatal(err)
		}
	}
	apply([]catalog.Candidate{c})
	if _, err := service.Pin(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	icon := "selfhst:appflowy"
	edit := catalog.Edit{DisplayName: "My app", Description: "My description", Address: "https://custom.test", Icon: &icon}
	if err := service.Edit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"purge", "invalid"} {
		if err := service.Lifecycle(ctx, c.ID, a); !errors.Is(err, catalog.ErrConflict) {
			t.Fatalf("%s live: %v", a, err)
		}
	}
	action("hide")
	action("hide")
	state("hidden")
	home(0)
	if _, err := service.Pin(ctx, c.ID); !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("pin hidden: %v", err)
	}
	apply([]catalog.Candidate{c})
	state("hidden")
	action("show")
	action("show")
	state("live")
	home(1)
	apply(nil)
	state("missing")
	home(0)
	if err := service.Lifecycle(ctx, c.ID, "hide"); !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("hide missing: %v", err)
	}
	// Reappearance may retain its original observation timestamp. Presence comes
	// from the complete snapshot, not the timestamp on one observation.
	apply([]catalog.Candidate{c})
	state("live")
	home(1)
	if err := service.Lifecycle(ctx, c.ID, "purge"); !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("stale purge after return: %v", err)
	}
	rows, _ := service.ListStaging(ctx)
	got := rows[0]
	if got.DisplayName != edit.DisplayName || got.Description != edit.Description || got.Address != edit.Address || got.Icon != icon || got.PinnedAt == nil {
		t.Fatalf("settings lost: %#v", got)
	}
	action("hide")
	apply(nil)
	state("hidden")
	action("show")
	state("missing")
	action("purge")
	if err := service.Lifecycle(ctx, c.ID, "purge"); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("repeated purge: %v", err)
	}
	var count int
	for _, table := range []string{"discovered_services", "service_edits", "service_endpoints"} {
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s remains: %d %v", table, count, err)
		}
	}
	// An old snapshot cannot recreate an entry the operator just purged.
	_, err := store.ReplaceSourceSnapshot(ctx, catalog.Snapshot{Source: c.Source, ObservedAt: at.Add(-time.Minute), Candidates: []catalog.Candidate{c}})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ = service.ListStaging(ctx)
	if len(rows) != 0 {
		t.Fatal("old snapshot resurrected service")
	}
}

func TestGroupedLifecycleIsAtomicAndKeepsLiveSharedResources(t *testing.T) {
	ctx := context.Background()
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	svc := catalog.NewService(store)
	at := time.Now().UTC()
	source := catalog.SourceRef{Kind: "kubernetes", ID: "cluster"}
	c := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "service", Namespace: "apps", Name: "app"}, at)
	pod := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "pod", Namespace: "apps", Name: "app-1"}, at)
	pod.Metadata = map[string]string{"kubernetes.services": "apps/app"}
	apply := func(cs []catalog.Candidate) {
		t.Helper()
		at = at.Add(time.Minute)
		if _, err := store.ReplaceSourceSnapshot(ctx, catalog.Snapshot{Source: source, ObservedAt: at, Candidates: cs}); err != nil {
			t.Fatal(err)
		}
	}
	apply([]catalog.Candidate{c, pod})
	if _, err := svc.Pin(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TRIGGER reject_hide BEFORE UPDATE OF hidden ON discovered_services WHEN NEW.resource_kind = 'pod' BEGIN SELECT RAISE(ABORT, 'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Lifecycle(ctx, c.ID, "hide"); err == nil {
		t.Fatal("expected failure")
	}
	rows, _ := store.List(ctx, false)
	for _, r := range rows {
		if r.Hidden {
			t.Fatal("partial hide committed")
		}
	}
	if _, err := db.Exec("DROP TRIGGER reject_hide"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Lifecycle(ctx, c.ID, "hide"); err != nil {
		t.Fatal(err)
	}
	newer := catalog.NewCandidate(source, catalog.ResourceRef{Kind: "pod", Namespace: "apps", Name: "app-2"}, at)
	newer.Metadata = pod.Metadata
	apply([]catalog.Candidate{c, newer})
	home, _ := svc.ListHomepage(ctx)
	if len(home) != 0 {
		t.Fatal("new pod bypassed hidden service")
	}
	if _, err := svc.Pin(ctx, newer.ID); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("pin child bypass: %v", err)
	}
	if err := svc.Lifecycle(ctx, c.ID, "show"); err != nil {
		t.Fatal(err)
	}
	// The Service is gone but its shared/current Pod must not be hard-deleted.
	apply([]catalog.Candidate{newer})
	if err := svc.Lifecycle(ctx, c.ID, "purge"); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.List(ctx, false)
	if len(rows) != 1 || rows[0].ID != newer.ID || rows[0].Missing {
		t.Fatalf("live member lost: %#v", rows)
	}
}
