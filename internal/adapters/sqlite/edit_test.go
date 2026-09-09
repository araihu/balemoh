package sqlite

import (
	"context"
	"errors"
	"github.com/araihu/balemoh/internal/application/catalog"
	"testing"
	"time"
)

func TestServiceEditSurvivesDiscovery(t *testing.T) {
	ctx := context.Background()
	db := newCatalogTestDatabase(t)
	store := NewCatalogStore(db)
	c := catalogTestCandidate("grafana", time.Now().UTC())
	if err := store.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	edit := catalog.Edit{DisplayName: "Monitoring", Description: "My dashboards", Address: "https://monitor.example.test/"}
	if err := store.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPinned(ctx, c.ID, true); err != nil {
		t.Fatal(err)
	}
	c.ObservedAt = c.ObservedAt.Add(time.Minute)
	c.DisplayName = "Rediscovered"
	c.Description = "Latest observation"
	if err := store.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.NewService(store).ListHomepage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DisplayName != edit.DisplayName || got[0].Description != edit.Description || got[0].Address != edit.Address || len(got[0].Endpoints) != len(c.Endpoints) {
		t.Fatalf("edit/discovery/pin mismatch: %#v", got)
	}
	edit.Address = ""
	if err := store.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.NewService(store).ListHomepage(ctx)
	if err != nil || got[0].Address != "" || got[0].Endpoints[0].URL != c.Endpoints[0].URL {
		t.Fatal("discovery address not restored", err)
	}
	if err := store.SaveEdit(ctx, "missing", edit); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	edit.Address = "javascript:alert(1)"
	if err := store.SaveEdit(ctx, c.ID, edit); !errors.Is(err, catalog.ErrInvalidEdit) {
		t.Fatalf("unsafe URL: %v", err)
	}
}
