package sqlite

import (
	"bytes"
	"errors"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/icons"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestIconLifecycleAndServiceReferences(t *testing.T) {
	ctx := t.Context()
	db := newCatalogTestDatabase(t)
	store := NewIconStore(db)
	services := NewCatalogStore(db)
	c := catalogTestCandidate("demo", time.Now().UTC())
	if err := services.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	var imageBytes bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&imageBytes, im); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, "", "Bad", "", "", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)); !errors.Is(err, icons.ErrInvalid) {
		t.Fatalf("unsafe upload: %v", err)
	}
	uploaded, err := store.Save(ctx, "", "Demo", "tools", "", imageBytes.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	edit := catalog.Edit{DisplayName: c.DisplayName, Description: c.Description, Icon: &uploaded.ID}
	if err := services.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, uploaded.ID, uploaded.Digest); !errors.Is(err, icons.ErrConflict) {
		t.Fatalf("deleted used icon: %v", err)
	}
	// Replacing metadata retains image bytes, changes revision, and preserves references.
	updated, err := store.Save(ctx, uploaded.ID, "New label", "tools", uploaded.Digest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, uploaded.ID, "Stale", "", uploaded.Digest, nil); !errors.Is(err, icons.ErrConflict) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	actual, err := store.Get(ctx, updated.ID)
	if err != nil || !bytes.Equal(actual.Data, imageBytes.Bytes()) {
		t.Fatal("image bytes changed", err)
	}
	list, err := store.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].UsedBy) != 1 {
		t.Fatalf("usage: %+v %v", list, err)
	}
	c.ObservedAt = c.ObservedAt.Add(time.Minute)
	if err := services.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.NewService(services).ListStaging(ctx)
	if err != nil || len(got) != 1 || got[0].Icon != uploaded.ID {
		t.Fatalf("discovery lost selection: %+v %v", got, err)
	}
	blank := ""
	edit.Icon = &blank
	if err := services.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, updated.ID, updated.Digest); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, updated.ID); !errors.Is(err, icons.ErrNotFound) {
		t.Fatal(err)
	}
	edit.Icon = &uploaded.ID
	if err := services.SaveEdit(ctx, c.ID, edit); !errors.Is(err, catalog.ErrInvalidEdit) {
		t.Fatalf("missing icon accepted: %v", err)
	}
	builtin := "selfhst:appflowy"
	edit.Icon = &builtin
	if err := services.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	edit.Icon = nil
	edit.DisplayName = "Updated name"
	if err := services.SaveEdit(ctx, c.ID, edit); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.NewService(services).ListStaging(ctx)
	if err != nil || got[0].Icon != builtin {
		t.Fatal("omitted icon cleared selection", err)
	}
}
