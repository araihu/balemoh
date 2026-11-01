package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/internal/application/dashboard"
	"github.com/araihu/balemoh/internal/application/plugins"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func layoutCode(t *testing.T, e error, code string) {
	t.Helper()
	var v *dashboard.LayoutError
	if !errors.As(e, &v) || v.Code != code {
		t.Fatalf("error %v, want %s", e, code)
	}
}
func TestLayoutVirtualCASNoOpDormantAndRestart(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewLayoutStore(db)
	service := &dashboard.Layouts{Store: store}
	ctx := t.Context()
	state, e := service.Get(ctx)
	if e != nil || state.Revision != "0" {
		t.Fatal(state, e)
	}
	var count int
	if e = db.QueryRow(`SELECT count(*) FROM dashboard_layout`).Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
	if _, e = db.Exec(`INSERT INTO dashboard_selections(source_kind,source_id) VALUES('host','offline')`); e != nil {
		t.Fatal(e)
	}
	d := state.Document
	d.Groups[0].Placements = []dashboard.GroupPlacement{{ItemID: "host-offline"}}
	d.Groups[0].Ordering = dashboard.Ordering{Mode: "custom", IDs: []string{"host-offline"}}
	saved, e := service.Replace(ctx, "0", d)
	if e != nil || saved.Revision != "1" {
		t.Fatal(saved, e)
	}
	same, e := service.Replace(ctx, "1", d)
	if e != nil || same.Revision != "1" {
		t.Fatal(same, e)
	}
	_, e = service.Replace(ctx, "0", d)
	layoutCode(t, e, "revision_conflict")
	if _, e = db.Exec(`DELETE FROM dashboard_selections`); e != nil {
		t.Fatal(e)
	}
	preview, e := service.Preview(ctx, dashboard.LayoutPreviewRequest{RequestID: "11111111-1111-4111-8111-111111111111", ExpectedRevision: "1", Document: d})
	if e != nil || preview.ResolvedGroups[0].Members[0].State != "dormant" {
		t.Fatal(preview, e)
	}
	again, e := (&dashboard.Layouts{Store: NewLayoutStore(db)}).Get(ctx)
	a, _ := json.Marshal(saved)
	b, _ := json.Marshal(again)
	if e != nil || string(a) != string(b) {
		t.Fatal("restart changed state", e)
	}
	if _, e = db.Exec(`UPDATE dashboard_layout SET document='{}'`); e != nil {
		t.Fatal(e)
	}
	_, e = service.Get(ctx)
	layoutCode(t, e, "layout_unreadable")
	if _, e = store.ReadDefault(ctx); e != nil {
		t.Fatal(e)
	}
	var raw string
	_ = db.QueryRow(`SELECT document FROM dashboard_layout`).Scan(&raw)
	if raw != "{}" {
		t.Fatal("default recovery overwrote saved layout")
	}
}
func TestLayoutConcurrentFirstWriteAndReservationCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.db")
	db, e := Open(t.Context(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	other, e := Open(t.Context(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { other.Close() })
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, store := range []*LayoutStore{NewLayoutStore(db), NewLayoutStore(other)} {
		wg.Go(func() {
			<-start
			d := dashboard.DefaultLayout()
			d.Homepage.Width = "contained"
			_, e := (&dashboard.Layouts{Store: store}).Replace(t.Context(), "0", d)
			results <- e
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else {
			layoutCode(t, e, "revision_conflict")
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	tx, e := db.BeginTx(t.Context(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`UPDATE dashboard_layout SET revision=revision WHERE 0`); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Millisecond)
	defer cancel()
	before := time.Now()
	_, e = (&dashboard.Layouts{Store: NewLayoutStore(other)}).Replace(ctx, "1", dashboard.DefaultLayout())
	layoutCode(t, e, "unavailable")
	if time.Since(before) > time.Second {
		t.Fatal("reservation ignored cancellation")
	}
}
func TestLayoutMembershipTokenComparedInsideTransaction(t *testing.T) {
	db := newCatalogTestDatabase(t)
	s := &dashboard.Layouts{Store: NewLayoutStore(db)}
	d := dashboard.DefaultLayout()
	preview, e := s.Preview(t.Context(), dashboard.LayoutPreviewRequest{RequestID: "11111111-1111-4111-8111-111111111111", ExpectedRevision: "0", Document: d})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO dashboard_selections(source_kind,source_id) VALUES('host','cold')`); e != nil {
		t.Fatal(e)
	}
	_, e = s.OrderItems(t.Context(), "0", "builtin:hosts", preview.MembershipTokens["builtin:hosts"], dashboard.Ordering{Mode: "alphabetical", IDs: []string{}})
	layoutCode(t, e, "membership_changed")
	preview, e = s.Preview(t.Context(), dashboard.LayoutPreviewRequest{RequestID: "11111111-1111-4111-8111-111111111111", ExpectedRevision: "0", Document: d})
	if e != nil {
		t.Fatal(e)
	}
	saved, e := s.OrderItems(t.Context(), "0", "builtin:hosts", preview.MembershipTokens["builtin:hosts"], dashboard.Ordering{Mode: "custom", IDs: []string{"host-cold"}})
	if e != nil || saved.Revision != "1" || len(saved.Document.Groups[0].Placements) != 1 {
		t.Fatal(saved, e)
	}
	_, e = s.Move(t.Context(), "1", "host-cold", dashboard.LayoutMove{GroupID: "builtin:widgets", BeforeItemID: json.RawMessage("null")})
	if e != nil {
		t.Fatal(e)
	}
}

func TestLayoutDeletedBuiltinIdentityCannotBeResurrected(t *testing.T) {
	db := newCatalogTestDatabase(t)
	s := &dashboard.Layouts{Store: NewLayoutStore(db)}
	d := dashboard.DefaultLayout()
	d.Groups = d.Groups[1:]
	d.Homepage.Placements = d.Homepage.Placements[1:]
	d.SourceGroups.Host = "builtin:services"
	first, e := s.Replace(t.Context(), "0", d)
	if e != nil || first.Revision != "1" {
		t.Fatal(first, e)
	}
	_, e = s.Replace(t.Context(), "1", dashboard.DefaultLayout())
	layoutCode(t, e, "invalid_layout")
	state, e := s.Get(t.Context())
	if e != nil || state.Revision != "1" || len(state.Document.Groups) != 2 {
		t.Fatal(state, e)
	}
}

type layoutFailedProvider string

func (p layoutFailedProvider) Kind() string { return string(p) }
func (layoutFailedProvider) Selected(context.Context) ([]dashboard.Widget, error) {
	return nil, errors.New("observation unavailable")
}
func (layoutFailedProvider) SetSelected(context.Context, string, bool) error { return nil }
func TestLayoutLocalEligibilityAndColdProviderOutage(t *testing.T) {
	db := newCatalogTestDatabase(t)
	ctx := t.Context()
	catalogStore := NewCatalogStore(db)
	ids := []string{}
	for _, name := range []string{"live-layout", "hidden-layout", "missing-layout"} {
		c := catalogTestCandidate(name, time.Now().UTC())
		if e := catalogStore.Upsert(ctx, c); e != nil {
			t.Fatal(e)
		}
		if _, e := catalogStore.SetPinned(ctx, c.ID, true); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, c.ID)
	}
	if e := NewDashboardSelections(db).Set(ctx, "host", "cold-host", true); e != nil {
		t.Fatal(e)
	}
	pluginsStore := NewPluginStore(db)
	connections, e := pluginsStore.Reconcile(ctx, []plugins.Provision{{PluginID: "test.fixture", Key: "main", Name: "Fixture", Digest: "hash", ManifestDigest: "m", Version: "0.0.1"}})
	if e != nil {
		t.Fatal(e)
	}
	placementID := "11111111-1111-4111-8111-111111111111"
	if _, _, e = pluginsStore.PutPlacement(ctx, placementID, plugins.PlacementBody{PluginID: "test.fixture", ConnectionID: connections[0].ID, ContributionID: "summary", Options: []byte("{}"), Enabled: true}, ""); e != nil {
		t.Fatal(e)
	}
	service := &dashboard.Layouts{Store: NewLayoutStore(db)}
	document := dashboard.DefaultLayout()
	for _, id := range ids {
		document.Groups[1].Placements = append(document.Groups[1].Placements, dashboard.GroupPlacement{ItemID: "service-" + id})
	}
	saved, e := service.Replace(ctx, "0", document)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := json.Marshal(saved)
	if _, e = db.Exec(`UPDATE discovered_services SET hidden=1 WHERE id=?`, ids[1]); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE discovered_services SET missing=1 WHERE id=?`, ids[2]); e != nil {
		t.Fatal(e)
	}
	snapshot := dashboard.New(layoutFailedProvider("service"), layoutFailedProvider("host"), layoutFailedProvider("plugin")).WithLayouts(service).Snapshot(ctx)
	if snapshot.LayoutError != nil || len(snapshot.Items) != 3 || len(snapshot.Unavailable) != 3 {
		t.Fatalf("cold outage lost eligible identities: %+v", snapshot)
	}
	active := map[string]bool{}
	for _, w := range snapshot.Items {
		active[w.ID] = true
		if w.Error == "" || w.Data != nil {
			t.Fatal("cold outage missing placeholder", w)
		}
	}
	for _, id := range []string{"service-" + ids[0], "host-cold-host", "plugin-" + placementID} {
		if !active[id] {
			t.Fatal("missing active identity", id)
		}
	}
	again, e := service.Get(ctx)
	after, _ := json.Marshal(again)
	if e != nil || string(before) != string(after) {
		t.Fatal("selection/lifecycle changed configuration or revision", e)
	}
	preview, e := service.Preview(ctx, dashboard.LayoutPreviewRequest{RequestID: "22222222-2222-4222-8222-222222222222", ExpectedRevision: saved.Revision, Document: saved.Document})
	if e != nil {
		t.Fatal(e)
	}
	dormant := 0
	for _, g := range preview.ResolvedGroups {
		for _, m := range g.Members {
			if m.State == "dormant" {
				dormant++
			}
		}
	}
	if dormant != 2 {
		t.Fatal("hidden/missing pinned services must be dormant", dormant)
	}
}
