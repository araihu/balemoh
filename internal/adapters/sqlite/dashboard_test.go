package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/application/telemetry"
)

func TestDashboardSelectionSurvivesPruningAndRestart(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/dashboard.db"
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	hosts := NewTelemetryStore(db)
	store := NewDashboardSelections(db)
	provider := dashboard.HostProvider{Hosts: hosts, Selections: store}
	now := time.Now().UTC()
	source := telemetry.Source{Kind: "kubernetes", ID: "cluster"}
	if err := hosts.Ingest(ctx, telemetry.Batch{Source: source, ObservedAt: now, Hosts: []telemetry.Observation{{}, {Node: "node"}}}, now); err != nil {
		t.Fatal(err)
	}
	root, node := telemetry.ID(source, ""), telemetry.ID(source, "node")
	for _, id := range []string{root, node, node} {
		if err := provider.SetSelected(ctx, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := provider.SetSelected(ctx, "missing", true); !errors.Is(err, dashboard.ErrNotFound) {
		t.Fatal("missing host accepted", err)
	}
	if err := hosts.Prune(ctx, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	items, err := provider.Selected(ctx)
	if err != nil || len(items) != 2 {
		t.Fatal("lost selections", items, err)
	}
	missing := false
	for _, item := range items {
		if item.SourceID == node {
			missing = item.Error != ""
		}
	}
	if !missing {
		t.Fatal("pruned host not unavailable")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	provider.Selections = NewDashboardSelections(db)
	provider.Hosts = NewTelemetryStore(db)
	for range 2 {
		if err := provider.SetSelected(ctx, node, false); err != nil {
			t.Fatal(err)
		}
	}
	items, err = provider.Selected(ctx)
	if err != nil || len(items) != 1 || items[0].SourceID != root {
		t.Fatal("restart/removal", items, err)
	}
}
func TestDashboardSelectionLimitAndRepeatedAdd(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir()+"/limit.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewDashboardSelections(db)
	for i := range 128 {
		if err := store.Set(ctx, "host", fmt.Sprint(i), true); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Set(ctx, "host", "0", true); err != nil {
		t.Fatal("repeat should succeed at limit", err)
	}
	if err := store.Set(ctx, "host", "overflow", true); !errors.Is(err, dashboard.ErrLimit) {
		t.Fatal("limit", err)
	}
}
