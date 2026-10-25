package sqlite

import (
	"context"
	"fmt"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/application/plugins"
	"sync"
	"testing"
	"uuid"
)

func TestPlacementsReplayRevisionAndSharedBudget(t *testing.T) {
	ctx := t.Context()
	db, e := Open(ctx, t.TempDir()+"/placements.db")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	s := NewPluginStore(db)
	cs, e := s.Reconcile(ctx, []plugins.Provision{{PluginID: "test.fixture", Key: "main", Name: "Fixture", Digest: "hash", ManifestDigest: "m", Version: "0.0.1"}})
	if e != nil {
		t.Fatal(e)
	}
	b := plugins.PlacementBody{PluginID: "test.fixture", ConnectionID: cs[0].ID, ContributionID: "summary", Options: []byte(`{"b":2,"a":1}`), Enabled: true}
	id := uuid.NewV4().String()
	p, created, e := s.PutPlacement(ctx, id, b, "")
	if e != nil || !created || p.Revision != "1" {
		t.Fatalf("create: %+v %v", p, e)
	}
	b.Options = []byte(`{"a":1,"b":2}`)
	_, created, e = s.PutPlacement(ctx, id, b, "")
	if e != nil || created {
		t.Fatalf("replay: %v", e)
	}
	b.Enabled = false
	if _, _, e = s.PutPlacement(ctx, id, b, ""); plugins.ErrorCode(e) != "conflict" {
		t.Fatalf("conflict: %v", e)
	}
	p, _, e = s.PutPlacement(ctx, id, b, "1")
	if e != nil || p.Revision != "2" {
		t.Fatalf("update: %v %+v", e, p)
	}
	if _, _, e = s.PutPlacement(ctx, id, b, "1"); plugins.ErrorCode(e) != "precondition_failed" {
		t.Fatalf("stale: %v", e)
	}
	if e = s.DeletePlacement(ctx, id, ""); plugins.ErrorCode(e) != "precondition_required" {
		t.Fatal(e)
	}
	hosts := NewDashboardSelections(db)
	for i := 0; i < 126; i++ {
		if e = hosts.Set(ctx, "host", fmt.Sprint(i), true); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			var e error
			if i%2 == 0 {
				e = hosts.Set(ctx, "host", fmt.Sprint(200+i), true)
			} else {
				_, _, e = s.PutPlacement(ctx, uuid.NewV4().String(), b, "")
			}
			mu.Lock()
			defer mu.Unlock()
			if e == nil {
				success++
			} else if e != dashboard.ErrLimit && plugins.ErrorCode(e) != "conflict" {
				t.Errorf("addition: %v", e)
			}
		})
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("shared budget allowed %d", success)
	}
	if _, _, e = s.PutPlacement(context.Background(), id, p.PlacementBody, ""); e != nil {
		t.Fatalf("replay at capacity: %v", e)
	}
	if e = s.DeletePlacement(ctx, id, "2"); e != nil {
		t.Fatal(e)
	}
	if e = s.DeletePlacement(ctx, id, "2"); plugins.ErrorCode(e) != "not_found" {
		t.Fatal(e)
	}
}

func TestDisableMissingPackageWithSemanticOptions(t *testing.T) {
	db, err := Open(t.Context(), t.TempDir()+"/placements.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewPluginStore(db)
	cs, err := store.Reconcile(t.Context(), []plugins.Provision{{PluginID: "example.fixture", Key: "main", Name: "Fixture", Digest: "hash", ManifestDigest: "m", Version: "0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	body := plugins.PlacementBody{PluginID: "example.fixture", ConnectionID: cs[0].ID, ContributionID: "summary", Options: []byte(`{"n":9007199254740993,"a":1}`), Enabled: true}
	id := uuid.NewV4().String()
	p, _, err := store.PutPlacement(t.Context(), id, body, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Reconcile(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	body.Enabled = false
	body.Options = []byte(`{"a":1.0,"n":9007199254740993.0}`)
	saved, _, err := plugins.New(store, nil).PutPlacement(t.Context(), id, body, p.Revision)
	if err != nil || saved.Enabled {
		t.Fatal("missing package prevented semantic disable", err)
	}
	body.Options = []byte(`{"a":1,"n":9007199254740994}`)
	if _, _, err = plugins.New(store, nil).PutPlacement(t.Context(), id, body, saved.Revision); plugins.ErrorCode(err) != "conflict" {
		t.Fatal("disable accepted changed options", err)
	}
}
