package sqlite

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/araihu/balemoh/internal/application/icons"
	p "github.com/araihu/balemoh/internal/application/presentation"
	"github.com/araihu/goshtoso/themes"
)

const themeOne = "theme:11111111-1111-4111-8111-111111111111"
const themeTwo = "theme:22222222-2222-4222-8222-222222222222"
const fontOne = "font:11111111-1111-4111-8111-111111111111"

func command(n string) string { return n + "0000000-0000-4000-8000-000000000000" }
func requireCode(t *testing.T, e error, code string) {
	t.Helper()
	var v *p.Error
	if !errors.As(e, &v) || v.Code != code {
		t.Fatalf("error %v, want %s", e, code)
	}
}
func TestPresentationAtomicSaveReplayAndConflicts(t *testing.T) {
	s := NewPresentationStore(newCatalogTestDatabase(t))
	ctx := t.Context()
	state, e := s.State(ctx)
	if e != nil || state.Settings.Revision != "0" || state.Settings.Selection.Key != "goshtoso" {
		t.Fatalf("default %+v %v", state, e)
	}
	d, _ := themes.Preset(themes.KeyGoshtoso)
	c := p.Save{CommandID: command("1"), ExpectedSettingsRevision: "0", ColorMode: "dark", Selection: p.Selection{Kind: "newCustom", ID: themeOne, Name: "My theme", Definition: &d, ExpectedCatalogRevision: themes.CatalogRevision()}}
	r, e := s.Save(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	retry, e := s.Save(ctx, c)
	if e != nil || jsonText(r) != jsonText(retry) {
		t.Fatal("immediate replay mutated state", retry, e)
	}
	changed := c
	changed.ColorMode = "light"
	_, e = s.Save(ctx, changed)
	requireCode(t, e, "conflict")
	_, e = s.RenameTheme(ctx, themeOne, p.Rename{Name: "Renamed", ExpectedRevision: "1"})
	if e != nil {
		t.Fatal(e)
	}
	draft := p.Draft{Name: "Stale draft", Definition: d}
	stale := p.Save{CommandID: command("2"), ExpectedSettingsRevision: "1", ColorMode: "light", Selection: p.Selection{Kind: "custom", ID: themeOne, ExpectedThemeRevision: "1", Draft: &draft}}
	_, e = s.Save(ctx, stale)
	requireCode(t, e, "conflict")
	state, e = s.State(ctx)
	if e != nil || state.Settings.ColorMode != "dark" || state.Settings.Revision != "1" {
		t.Fatal("partial save", state, e)
	}
	e = s.DeleteTheme(ctx, themeOne, "2")
	requireCode(t, e, "in_use")
	activate := p.Save{CommandID: command("3"), ExpectedSettingsRevision: "1", ColorMode: "system", Selection: p.Selection{Kind: "builtin", Key: "arctic", ExpectedCatalogRevision: themes.CatalogRevision()}}
	if _, e = s.Save(ctx, activate); e != nil {
		t.Fatal(e)
	}
	_, e = s.Save(ctx, c)
	requireCode(t, e, "conflict")
	if e = s.DeleteTheme(ctx, themeOne, "2"); e != nil {
		t.Fatal(e)
	}
}
func TestPresentationFontReferencesCopiesAndRestart(t *testing.T) {
	db := newCatalogTestDatabase(t)
	s := NewPresentationStore(db)
	ctx := t.Context()
	f := p.Font{ID: fontOne, Name: "Font", Family: "Test", Weight: "400", Style: "normal", Format: "truetype", MIME: "font/ttf", Data: []byte("validated upstream")}
	saved, created, e := s.CreateFont(ctx, f)
	if e != nil || !created {
		t.Fatal(e)
	}
	_, created, e = s.CreateFont(ctx, f)
	if e != nil || created {
		t.Fatal("font replay", e)
	}
	f.Name = "Changed"
	_, _, e = s.CreateFont(ctx, f)
	requireCode(t, e, "conflict")
	d, _ := themes.Preset(themes.KeyGoshtoso)
	d.Typography.Title = themes.FontRef{Source: "upload", ID: fontOne}
	a, created, e := s.CreateTheme(ctx, p.Create{ID: themeOne, Name: "Original", Definition: &d})
	if e != nil || !created {
		t.Fatal(e)
	}
	b, _, e := s.CreateTheme(ctx, p.Create{ID: themeTwo, Name: "Copy", CopyFrom: &p.CopyFrom{ID: themeOne, ExpectedRevision: "1"}})
	if e != nil {
		t.Fatal(e)
	}
	if jsonText(a.Definition) != jsonText(b.Definition) {
		t.Fatal("copy inherited instead of snapshot")
	}
	e = s.DeleteFont(ctx, fontOne, saved.Revision)
	requireCode(t, e, "in_use")
	fonts, e := s.Fonts(ctx)
	if e != nil || len(fonts) != 1 || len(fonts[0].UsedBy) != 2 || len(fonts[0].Data) != 0 {
		t.Fatal("reference catalog", fonts, e)
	}
	if e = s.DeleteTheme(ctx, themeOne, "1"); e != nil {
		t.Fatal(e)
	}
	again := NewPresentationStore(db)
	still, e := again.GetTheme(ctx, themeTwo)
	if e != nil || jsonText(still.Definition) != jsonText(b.Definition) {
		t.Fatal("copy changed", e)
	}
	if e = again.DeleteTheme(ctx, themeTwo, "1"); e != nil {
		t.Fatal(e)
	}
	if e = again.DeleteFont(ctx, fontOne, "1"); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRowContext(ctx, "SELECT count(*) FROM presentation_theme_fonts").Scan(&count); e != nil || count != 0 {
		t.Fatal("orphan references", count, e)
	}
}
func TestPresentationRejectsIncompleteSnapshotWithoutMutation(t *testing.T) {
	s := NewPresentationStore(newCatalogTestDatabase(t))
	d, _ := themes.Preset(themes.KeyGoshtoso)
	delete(d.Tokens, "primary")
	_, _, e := s.CreateTheme(t.Context(), p.Create{ID: themeOne, Name: "Bad", Definition: &d})
	requireCode(t, e, "invalid_request")
	catalog, e := s.Catalog(t.Context())
	if e != nil || len(catalog.Custom) != 0 {
		t.Fatal("invalid partial insert", e)
	}
	var decoded themes.Definition
	b, _ := json.Marshal(d)
	_ = json.Unmarshal(b, &decoded)
}

func TestPresentationWALBackupAndReopenPreserveAssets(t *testing.T) {
	ctx := t.Context()
	path := t.TempDir() + "/live.db"
	backup := t.TempDir() + "/backup.db"
	db, e := Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	if e = RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); e != nil {
		t.Fatal(e)
	}
	s := NewPresentationStore(db)
	file := []byte("immutable validated file bytes")
	_, _, e = s.CreateFont(ctx, p.Font{ID: fontOne, Name: "Snapshot font", Family: "Snapshot", Weight: "400", Style: "normal", Format: "truetype", MIME: "font/ttf", Data: file})
	if e != nil {
		t.Fatal(e)
	}
	d, _ := themes.Preset(themes.KeyGoshtoso)
	d.Typography.Body = themes.FontRef{Source: "upload", ID: fontOne}
	command := p.Save{CommandID: command("1"), ExpectedSettingsRevision: "0", ColorMode: "dark", Selection: p.Selection{Kind: "newCustom", ID: themeOne, Name: "Backup theme", Definition: &d, ExpectedCatalogRevision: themes.CatalogRevision()}}
	if _, e = s.Save(ctx, command); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, "VACUUM INTO ?", backup); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	for _, location := range []string{path, backup} {
		restored, e := Open(ctx, location)
		if e != nil {
			t.Fatal(e)
		}
		store := NewPresentationStore(restored)
		state, e := store.State(ctx)
		if e != nil || state.Settings.Selection.ID != themeOne || state.Settings.Revision != "1" || state.Settings.ColorMode != "dark" || state.Definition.Typography.Body.ID != fontOne {
			t.Fatal("restore lost presentation", state, e)
		}
		font, e := store.Font(ctx, fontOne)
		if e != nil || string(font.Data) != string(file) {
			t.Fatal("restore lost font", e)
		}
		if e = restored.Close(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestPresentationQuotasDoNotBlockExactReplay(t *testing.T) {
	ctx := t.Context()
	t.Run("themes", func(t *testing.T) {
		s := NewPresentationStore(newCatalogTestDatabase(t))
		d, _ := themes.Preset(themes.KeyGoshtoso)
		first := p.Create{ID: themeOne, Name: "First", Definition: &d}
		if _, _, e := s.CreateTheme(ctx, first); e != nil {
			t.Fatal(e)
		}
		for i := 1; i < p.MaxThemes; i++ {
			if _, _, e := s.CreateTheme(ctx, p.Create{ID: fmt.Sprintf("theme:%08x-0000-4000-8000-000000000000", i), Name: "Stored snapshot", Definition: &d}); e != nil {
				t.Fatal(e)
			}
		}
		_, _, e := s.CreateTheme(ctx, p.Create{ID: themeTwo, Name: "Over quota", Definition: &d})
		requireCode(t, e, "limit_exceeded")
		if _, created, e := s.CreateTheme(ctx, first); e != nil || created {
			t.Fatal("replay at quota", e)
		}
	})
	for _, quota := range []struct {
		name          string
		count, length int
	}{{"font count", p.MaxFonts, 1}, {"font bytes", 52, 10 << 20}} {
		t.Run(quota.name, func(t *testing.T) {
			db := newCatalogTestDatabase(t)
			s := NewPresentationStore(db)
			// Seed accounting totals without allocating half a GiB of fixture bytes.
			_, e := db.ExecContext(ctx, `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<?)
   INSERT INTO presentation_fonts SELECT printf('font:%08x-0000-4000-8000-000000000000',x),'Fixture','Fixture','400','normal',0,'truetype','font/ttf',?,'digest',x'00',1,'creation' FROM n`, quota.count, quota.length)
			if e != nil {
				t.Fatal(e)
			}
			_, _, e = s.CreateFont(ctx, p.Font{ID: fontOne, Name: "Over quota", Data: []byte{0}})
			requireCode(t, e, "limit_exceeded")
		})
	}
}

func TestPresentationConcurrentSaveHasOneWinner(t *testing.T) {
	s := NewPresentationStore(newCatalogTestDatabase(t))
	ctx := t.Context()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, n := range []string{"1", "2"} {
		go func() {
			<-start
			_, e := s.Save(ctx, p.Save{CommandID: command(n), ExpectedSettingsRevision: "0", ColorMode: "light", Selection: p.Selection{Kind: "builtin", Key: "arctic", ExpectedCatalogRevision: themes.CatalogRevision()}})
			results <- e
		}()
	}
	close(start)
	wins := 0
	for range 2 {
		if e := <-results; e == nil {
			wins++
		} else {
			requireCode(t, e, "conflict")
		}
	}
	state, e := s.State(ctx)
	if e != nil || wins != 1 || state.Settings.Revision != "1" {
		t.Fatal("concurrent save", wins, state.Settings, e)
	}
}

func TestThemeBrandReferencesAreDurableAndProtectImages(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewPresentationStore(db)
	images := NewIconStore(db)
	ctx := t.Context()
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" fill="#336699"/></svg>`)
	image, err := images.Save(ctx, "", "Logo", "theme-brand", "", data)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := themes.Preset(themes.KeyGoshtoso)
	command1 := p.Save{CommandID: command("1"), ExpectedSettingsRevision: "0", ColorMode: "system", Selection: p.Selection{Kind: "newCustom", ID: themeOne, Name: "Brand theme", Definition: &d, BrandIconID: image.ID, ExpectedCatalogRevision: themes.CatalogRevision()}}
	if _, err = store.Save(ctx, command1); err != nil {
		t.Fatal(err)
	}
	state, err := store.State(ctx)
	if err != nil || state.BrandIconID != image.ID || state.BrandLogoID != image.ID {
		t.Fatal("brand not selected", state, err)
	}
	if _, err = store.Save(ctx, command1); err != nil {
		t.Fatal("brand replay", err)
	}
	_, err = images.Save(ctx, image.ID, image.Name, image.Tags, image.Digest, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="2"/></svg>`))
	if !errors.Is(err, icons.ErrConflict) {
		t.Fatal("referenced image was replaced", err)
	}
	if err = images.Delete(ctx, image.ID, image.Digest); !errors.Is(err, icons.ErrConflict) {
		t.Fatal("referenced image deleted", err)
	}
	list, err := images.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].UsedBy) != 1 || list[0].UsedBy[0] != "Theme: Brand theme" {
		t.Fatal("brand reference not visible", list, err)
	}
	copied, _, err := store.CreateTheme(ctx, p.Create{ID: themeTwo, Name: "Copy", CopyFrom: &p.CopyFrom{ID: themeOne, ExpectedRevision: "1"}})
	if err != nil || copied.BrandIconID != image.ID {
		t.Fatal("brand not copied", copied, err)
	}
	// An older client omitting the new field must preserve saved branding.
	draft := p.Draft{Name: "Renamed theme", Definition: d}
	edit := p.Save{CommandID: command("2"), ExpectedSettingsRevision: "1", ColorMode: "system", Selection: p.Selection{Kind: "custom", ID: themeOne, ExpectedThemeRevision: "1", Draft: &draft}}
	if _, err = store.Save(ctx, edit); err != nil {
		t.Fatal(err)
	}
	again, err := NewPresentationStore(db).GetTheme(ctx, themeOne)
	if err != nil || again.BrandIconID != image.ID {
		t.Fatal("older draft cleared branding", again, err)
	}
	empty := ""
	draft.BrandIconID = &empty
	draft.BrandLogoID = &empty
	edit.CommandID = command("3")
	edit.ExpectedSettingsRevision = "2"
	edit.Selection.ExpectedThemeRevision = "2"
	if _, err = store.Save(ctx, edit); err != nil {
		t.Fatal(err)
	}
	state, err = store.State(ctx)
	if err != nil || state.BrandIconID != "" {
		t.Fatal("default branding not restored", state, err)
	}
	if err = images.Delete(ctx, image.ID, image.Digest); !errors.Is(err, icons.ErrConflict) {
		t.Fatal("copy lost reference protection", err)
	}
	if err = store.DeleteTheme(ctx, themeTwo, "1"); err != nil {
		t.Fatal(err)
	}
	if err = images.Delete(ctx, image.ID, image.Digest); err != nil {
		t.Fatal("unreferenced image cannot be deleted", err)
	}
}

func TestThemeBrandRejectsUnknownImageWithoutSaving(t *testing.T) {
	store := NewPresentationStore(newCatalogTestDatabase(t))
	d, _ := themes.Preset(themes.KeyGoshtoso)
	_, err := store.Save(t.Context(), p.Save{CommandID: command("1"), ExpectedSettingsRevision: "0", ColorMode: "light", Selection: p.Selection{Kind: "newCustom", ID: themeOne, Name: "Invalid image", Definition: &d, BrandIconID: "upload:missing", ExpectedCatalogRevision: themes.CatalogRevision()}})
	requireCode(t, err, "invalid_request")
	state, err := store.State(t.Context())
	if err != nil || state.Settings.Revision != "0" {
		t.Fatal("partial brand save", state, err)
	}
}

func TestThemeLogoAndIconAreIndependent(t *testing.T) {
	db := newCatalogTestDatabase(t)
	store := NewPresentationStore(db)
	images := NewIconStore(db)
	ctx := t.Context()
	logo, err := images.Save(ctx, "", "Logo", "", "", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="32" height="16"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	icon, err := images.Save(ctx, "", "Icon", "", "", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="8"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := themes.Preset(themes.KeyGoshtoso)
	save := p.Save{CommandID: command("1"), ExpectedSettingsRevision: "0", ColorMode: "system", Selection: p.Selection{Kind: "newCustom", ID: themeOne, Name: "Independent", Definition: &d, BrandLogoID: &logo.ID, BrandIconID: icon.ID, ExpectedCatalogRevision: themes.CatalogRevision()}}
	if _, err = store.Save(ctx, save); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Save(ctx, save); err != nil {
		t.Fatal("replay", err)
	}
	state, err := store.State(ctx)
	if err != nil || state.BrandLogoID != logo.ID || state.BrandIconID != icon.ID {
		t.Fatal("independent state", state, err)
	}
	copy, _, err := store.CreateTheme(ctx, p.Create{ID: themeTwo, Name: "Copy", CopyFrom: &p.CopyFrom{ID: themeOne, ExpectedRevision: "1"}})
	if err != nil || copy.BrandLogoID != logo.ID || copy.BrandIconID != icon.ID {
		t.Fatal("copy", copy, err)
	}
	for _, image := range []icons.Icon{logo, icon} {
		if err = images.Delete(ctx, image.ID, image.Digest); !errors.Is(err, icons.ErrConflict) {
			t.Fatal("unprotected image", image.ID, err)
		}
	}
	empty := ""
	save = p.Save{CommandID: command("2"), ExpectedSettingsRevision: "1", ColorMode: "system", Selection: p.Selection{Kind: "custom", ID: themeOne, ExpectedThemeRevision: "1", Draft: &p.Draft{Name: "Independent", Definition: d, BrandIconID: &empty}}}
	if _, err = store.Save(ctx, save); err != nil {
		t.Fatal(err)
	}
	state, err = store.State(ctx)
	if err != nil || state.BrandLogoID != logo.ID || state.BrandIconID != "" {
		t.Fatal("icon reset changed logo", state, err)
	}
	save.CommandID = command("3")
	save.ExpectedSettingsRevision = "2"
	save.Selection.ExpectedThemeRevision = "2"
	save.Selection.Draft.BrandLogoID = &empty
	save.Selection.Draft.BrandIconID = &icon.ID
	if _, err = store.Save(ctx, save); err != nil {
		t.Fatal(err)
	}
	state, err = store.State(ctx)
	if err != nil || state.BrandLogoID != "" || state.BrandIconID != icon.ID {
		t.Fatal("logo reset changed icon", state, err)
	}
	bad := "upload:missing"
	save.CommandID = command("4")
	save.ExpectedSettingsRevision = "3"
	save.Selection.ExpectedThemeRevision = "3"
	save.Selection.Draft.BrandLogoID = &bad
	_, err = store.Save(ctx, save)
	requireCode(t, err, "invalid_request")
	state, err = store.State(ctx)
	if err != nil || state.Settings.Revision != "3" || state.BrandLogoID != "" || state.BrandIconID != icon.ID {
		t.Fatal("invalid logo partially saved", state, err)
	}
}
