package view

import (
	"bytes"
	"github.com/a-h/templ"
	"strings"
	"testing"
)

func TestLayoutMovementBelongsToSelectedContentAndDormantFallback(t *testing.T) {
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
	if strings.Index(active, `id="layout-selected-movement"`) < strings.Index(active, `id="editor-item-`+DOMKey("plugin-a")+`"`) {
		t.Fatal("active movement escaped its selected preview item")
	}
	if strings.Index(active, `id="layout-editor-page-controls"`) > strings.Index(active, `class="balemoh-layout-workspace"`) {
		t.Fatal("page arrangement no longer precedes preview workspace")
	}
	if !strings.Contains(active, `data-layout-group-fields-owner="builtin:widgets"`) {
		t.Fatal("group fields lost their stable owner")
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
	if strings.Count(dormant, `id="layout-selected-movement"`) != 1 || strings.Index(dormant, `id="layout-selected-movement"`) > strings.Index(dormant, `class="balemoh-layout-preview-column"`) {
		t.Fatal("dormant movement is not available in inspector")
	}
}
