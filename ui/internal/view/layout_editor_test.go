package view

import (
	"bytes"
	"github.com/a-h/templ"
	"strings"
	"testing"
)

func TestLayoutPreviewFirstKeepsMovementInOneContextualInspector(t *testing.T) {
	frame := LayoutFrame{Width: "full", Page: Arrangement{Type: "grid", Columns: 2, GapPx: 16}, PageOrder: "alphabetical", SelectedGroup: "builtin:widgets", SelectedItem: "plugin-a", SourceGroups: map[string]string{"host": "builtin:widgets", "service": "builtin:widgets", "other": "builtin:widgets"}, Groups: []EditorGroup{{ID: "builtin:widgets", Label: "Mixed", Title: "Mixed", Span: "auto", Order: "alphabetical", Arrangement: Arrangement{Type: "masonry", Columns: 2, GapPx: 16}, ShowHeading: true, Members: []EditorMember{{ID: "plugin-a", Label: "Plain plugin", State: "active", Widget: Widget{ID: "plugin-a", Content: templ.Raw("<aside>Arbitrary plugin content</aside>")}}, {ID: "host-inactive", Label: "Inactive host", State: "dormant"}}}}}
	render := func() string {
		t.Helper()
		var b bytes.Buffer
		if e := LayoutEditorFrame(frame).Render(t.Context(), &b); e != nil {
			t.Fatal(e)
		}
		return b.String()
	}
	active := render()
	if strings.Count(active, `id="layout-selected-movement"`) != 1 || !strings.Contains(active, "Arbitrary plugin content") {
		t.Fatal("selected movement missing or duplicated")
	}
	inspectorAt := strings.Index(active, `id="layout-editor-inspector"`)
	previewAt := strings.Index(active, `class="balemoh-layout-preview-column"`)
	movementAt := strings.Index(active, `id="layout-selected-movement"`)
	if !(previewAt < inspectorAt && inspectorAt < movementAt) {
		t.Fatal("preview must precede one contextual movement inspector")
	}
	if strings.Contains(active[previewAt:inspectorAt], `id="layout-selected-movement"`) {
		t.Fatal("movement controls change widget geometry")
	}
	if strings.Contains(active[inspectorAt:], `data-layout-group-fields-owner`) {
		t.Fatal("group and item settings compete in the inspector")
	}
	if !strings.Contains(active, `data-layout-panel="page-settings"`) || strings.Contains(active, `data-layout-panel="page-settings" open`) {
		t.Fatal("page settings must start collapsed")
	}
	frame.Groups[0].InitiallyCollapsed = true
	collapsed := render()
	detailsAt := strings.Index(collapsed, `<details data-editor-collapse=`)
	if detailsAt < 0 {
		t.Fatal("missing collapsed-group editor disclosure")
	}
	opening := collapsed[detailsAt : detailsAt+strings.Index(collapsed[detailsAt:], ">")+1]
	if !strings.Contains(opening, " open") {
		t.Fatal("selected active item became hidden inside closed disclosure", opening)
	}
	if !frame.Groups[0].InitiallyCollapsed {
		t.Fatal("editor changed saved collapse preference")
	}
	frame.SelectedItem = "host-inactive"
	dormant := render()
	if strings.Count(dormant, `id="layout-selected-movement"`) != 1 || strings.Index(dormant, `id="layout-selected-movement"`) < strings.Index(dormant, `id="layout-editor-inspector"`) {
		t.Fatal("dormant movement is not available in inspector")
	}
}

func TestLayoutGroupContextRetainsFieldsAndAccessibleModes(t *testing.T) {
	f := LayoutFrame{Width: "contained", Page: Arrangement{Type: "grid", Columns: 2, GapPx: 16}, PageOrder: "alphabetical", SelectedGroup: "builtin:hosts", SourceGroups: map[string]string{"host": "builtin:hosts", "service": "builtin:hosts", "other": "builtin:hosts"}, Groups: []EditorGroup{{ID: "builtin:hosts", Title: "Hosts", Label: "Hosts", Span: "auto", Order: "alphabetical", Arrangement: Arrangement{Type: "grid", Columns: 2, GapPx: 16}, Members: []EditorMember{}}}}
	var b bytes.Buffer
	if e := LayoutEditorFrame(f).Render(t.Context(), &b); e != nil {
		t.Fatal(e)
	}
	html := b.String()
	if !strings.Contains(html, `data-layout-group-fields-owner="builtin:hosts"`) || strings.Contains(html, `id="layout-selected-movement"`) {
		t.Fatal("group context does not own its controls")
	}
	for _, mode := range []string{"stack", "grid", "masonry"} {
		if !strings.Contains(html, `value="`+mode+`"`) {
			t.Fatal("missing accessible layout choice", mode)
		}
	}
	if !strings.Contains(html, "Drop items here") || !strings.Contains(html, "Default groups for new items") {
		t.Fatal("empty/default configuration guidance missing")
	}
}
