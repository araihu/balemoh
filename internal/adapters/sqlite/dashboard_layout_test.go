package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/internal/application/dashboard"
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
