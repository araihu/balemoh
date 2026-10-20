package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
)

func manualInput() catalog.ManualService {
	return catalog.ManualService{RequestID: "0123456789abcdef0123456789abcdef", Edit: catalog.Edit{DisplayName: "My router", Description: "Local admin", Address: "http://10.0.0.1/"}}
}

func TestManualServiceLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	service := catalog.NewService(store)
	input := manualInput()
	c, err := service.CreateManual(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if c.Source.Kind != "manual" || c.Address != input.Edit.Address || c.PinnedAt != nil || c.Status() != "live" {
		t.Fatalf("created: %+v", c)
	}
	if _, err = service.Pin(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	edit := input.Edit
	edit.DisplayName, edit.Address = "Edited router", "https://router.example.test/"
	if err = service.Edit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err = service.Lifecycle(ctx, c.ID, "hide"); err != nil {
		t.Fatal(err)
	}
	replay, err := service.CreateManual(ctx, input)
	if err != nil || replay.ID != c.ID || replay.DisplayName != edit.DisplayName || replay.Address != edit.Address || !replay.Hidden || replay.PinnedAt == nil {
		t.Fatalf("replay overwrote current record: %+v %v", replay, err)
	}
	rows, err := service.ListStaging(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatalf("duplicate creation: %d %v", len(rows), err)
	}
	if home, err := service.ListHomepage(ctx); err != nil || len(home) != 0 {
		t.Fatal("hidden service leaked", home, err)
	}
	if err = service.Lifecycle(ctx, c.ID, "show"); err != nil {
		t.Fatal(err)
	}
	// Ordinary discovery cannot make manual records disappear.
	if _, err = service.ImportSnapshot(ctx, catalog.Snapshot{Source: catalog.SourceRef{Kind: "container", ID: "host"}, ObservedAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	home, err := service.ListHomepage(ctx)
	if err != nil || len(home) != 1 || home[0].ID != c.ID || home[0].DisplayName != edit.DisplayName {
		t.Fatal(home, err)
	}
	forged := catalog.Snapshot{Source: c.Source, ObservedAt: time.Now().Add(time.Hour)}
	if _, err = service.ImportSnapshot(ctx, forged); !errors.Is(err, catalog.ErrInvalidSnapshot) {
		t.Fatal("manual source imported", err)
	}
	if _, err = store.ReplaceSourceSnapshot(ctx, forged); err == nil {
		t.Fatal("manual source accepted directly")
	}
	if err = store.Upsert(ctx, c); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("manual source upsert accepted", err)
	}
	hosts, err := NewTelemetryStore(db).List(ctx, time.Now())
	if err != nil || len(hosts) != 0 {
		t.Fatal("manual service generated a host", hosts, err)
	}
	edit.Address = ""
	if err = service.Edit(ctx, c.ID, edit); !errors.Is(err, catalog.ErrInvalidEdit) {
		t.Fatal("blank manual address accepted", err)
	}
}

func TestManualServiceValidationAndAtomicity(t *testing.T) {
	ctx := context.Background()
	db := newCatalogTestDatabase(t)
	service := catalog.NewService(NewCatalogStore(db))
	for _, address := range []string{"", "javascript:alert(1)", "file:///tmp/a", "https://user:pass@example.test/", "not-a-url"} {
		input := manualInput()
		input.Edit.Address = address
		if _, err := service.CreateManual(ctx, input); !errors.Is(err, catalog.ErrInvalidEdit) {
			t.Fatalf("address %q: %v", address, err)
		}
	}
	input := manualInput()
	input.RequestID = "existing-discovered-id"
	if _, err := service.CreateManual(ctx, input); !errors.Is(err, catalog.ErrInvalidEdit) {
		t.Fatal("bad key", err)
	}
	input = manualInput()
	badIcon := "upload:nonexistent"
	input.Edit.Icon = &badIcon
	if _, err := service.CreateManual(ctx, input); !errors.Is(err, catalog.ErrInvalidEdit) {
		t.Fatal("bad icon", err)
	}
	if rows, _ := service.ListStaging(ctx); len(rows) != 0 {
		t.Fatal("partial record after invalid icon", rows)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_manual_edit BEFORE INSERT ON service_edits BEGIN SELECT RAISE(ABORT, 'test storage failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateManual(ctx, manualInput()); err == nil {
		t.Fatal("storage failure ignored")
	}
	if rows, _ := service.ListStaging(ctx); len(rows) != 0 {
		t.Fatal("partial record after storage failure", rows)
	}
}

func TestManualServiceSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/manual.db"
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	c, err := catalog.NewService(NewCatalogStore(db)).CreateManual(ctx, manualInput())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	replay, err := catalog.NewService(NewCatalogStore(db)).CreateManual(ctx, manualInput())
	if err != nil || replay.ID != c.ID || replay.Address != c.Address {
		t.Fatal("restart lost manual record", replay, err)
	}
}
