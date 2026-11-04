package view

import (
	"bytes"
	"github.com/a-h/templ"
	"regexp"
	"strings"
	"testing"
)

func TestLayoutPreviewFirstKeepsMovementInOneContextualInspector(t *testing.T) {
	frame := LayoutFrame{Width: "full", Page: Arrangement{Type: "grid", Columns: 2, GapPx: 16}, PageOrder: "alphabetical", SelectedGroup: "builtin:widgets", SelectedItem: "plugin-a", SourceGroups: map[string]string{"host": "builtin:widgets", "service": "builtin:widgets", "other": "builtin:widgets"}, Groups: []EditorGroup{{ID: "builtin:widgets", Label: "Mixed", Title: "Mixed", Span: "auto", Order: "alphabetical", Arrangement: Arrangement{Type: "masonry", Columns: 2, GapPx: 16}, ShowHeading: true, Members: []EditorMember{{ID: "plugin-a", Label: "Plain plugin", State: "active", Widget: Widget{ID: "plugin-a", Content: templ.Raw(`<aside><a href="/plugins/example">Arbitrary plugin content</a><button type="button">Widget action</button></aside>`)}}, {ID: "host-inactive", Label: "Inactive host", State: "dormant"}}}}}
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
	if !strings.Contains(active, `<div data-layout-preview-content inert>`) || strings.Count(active, `data-layout-content-drag="true"`) != 2 {
		t.Fatal("card bodies must be inactive; groups and cards must support independent body dragging")
	}
	if strings.Index(active, `data-layout-select-item`) > strings.Index(active, `data-layout-preview-content inert`) {
		t.Fatal("item settings button must stay outside inactive card content")
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
	for _, label := range []string{"Page", "Group", "Item"} {
		if !strings.Contains(active, `role="tabpanel" aria-label="`+label+`"`) {
			t.Fatal("missing tab panel", label)
		}
	}
	if !strings.Contains(active, `right-0 top-0 bottom-0 border-l`) || strings.Contains(active, `left-0 top-0 bottom-0 border-r`) {
		t.Fatal("settings drawer must open from the right")
	}
	if !strings.Contains(active, `aria-label="Preview actions"`) || !strings.Contains(active, `data-goshtoso-action-group`) {
		t.Fatal("preview actions must use the shared action group")
	}
	if strings.Count(active, `role="dialog"`) != 1 || !strings.Contains(active, `layoutSettingsIsOpen: false`) {
		t.Fatal("settings must start in a closed drawer")
	}
	if strings.Contains(active, "Layout limits") || strings.Contains(active, "data-layout-capacity") {
		t.Fatal("documentation limits leaked into editor")
	}
	if !strings.Contains(active, `id="layout-group-position"`) || !strings.Contains(active, `id="layout-item-position"`) || !strings.Contains(active, `type="number"`) {
		t.Fatal("missing group/item numeric ordering")
	}

	frame.Groups[0].Collapsible = true
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

func TestLayoutActionCommandsRenderNativeButtonsIncludingOverflow(t *testing.T) {
	var b bytes.Buffer
	for _, component := range []templ.Component{LayoutPrimaryActions(), LayoutPageArrangement(LayoutFrame{})} {
		if err := component.Render(t.Context(), &b); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"layout-action-save", "layout-action-cancel", "layout-action-cancel-overflow", "layout-action-create", "layout-action-create-overflow"} {
		opening := regexp.MustCompile(`<button\b[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`).FindString(b.String())
		if opening == "" {
			t.Fatalf("%s must be a native command button", id)
		}
		if id == "layout-action-save" && !regexp.MustCompile(`\sdisabled(?:[=>\s])`).MatchString(opening) {
			t.Fatal("Save must start disabled")
		}
		if id != "layout-action-save" && regexp.MustCompile(`\sdisabled(?:[=>\s])`).MatchString(opening) {
			t.Fatalf("%s has static disabled treatment", id)
		}
	}
}

func TestEmptyEditorGroupAlwaysExposesDropTarget(t *testing.T) {
	for _, members := range [][]EditorMember{nil, {{ID: "dormant", State: "dormant", Label: "Dormant item"}}} {
		g := EditorGroup{ID: "empty", Label: "Empty", Collapsible: true, InitiallyCollapsed: true, Arrangement: Arrangement{Type: "masonry", Columns: 2, GapPx: 16}, Members: members}
		f := LayoutFrame{SelectedGroup: "other"}
		var b bytes.Buffer
		if err := LayoutPreviewGroup(f, g).Render(t.Context(), &b); err != nil {
			t.Fatal(err)
		}
		html := b.String()
		if strings.Contains(html, "<details") || !strings.Contains(html, "balemoh-layout-empty-target") || !strings.Contains(html, "Drop items here") || !strings.Contains(html, `data-layout-content-drag="true"`) {
			t.Fatal("empty group must expose its drop target independently of selection/collapse")
		}
		if !g.InitiallyCollapsed {
			t.Fatal("editor changed saved homepage collapse preference")
		}
	}
}

func TestGroupAndItemUseSameExplicitEditAction(t *testing.T) {
	var b bytes.Buffer
	f := LayoutFrame{SelectedGroup: "g"}
	g := EditorGroup{ID: "g", Label: "Group", Arrangement: Arrangement{Type: "stack", Columns: 1, GapPx: 16}}
	if err := LayoutPreviewGroup(f, g).Render(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	if err := LayoutPreviewItem(f, EditorMember{ID: "i", Label: "Item", Widget: Widget{ID: "i", Content: templ.Raw("<p>Card</p>")}}).Render(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	if strings.Count(html, "balemoh-layout-edit") != 2 {
		t.Fatal("groups and cards must share their edit control")
	}
	if !strings.Contains(html, `data-layout-select-group="g"`) || !strings.Contains(html, `data-layout-select-item="i"`) {
		t.Fatal("explicit edit actions missing")
	}
	heading := regexp.MustCompile(`<h3[^>]*>Group</h3>`).FindString(html)
	if heading == "" || strings.Contains(heading, "data-layout-select") {
		t.Fatal("group heading must be plain draggable content")
	}
}

func TestGroupPresentationOptInsAreIndependent(t *testing.T) {
	for _, heading := range []bool{false, true} {
		for _, accordion := range []bool{false, true} {
			g := dashboardSection{ID: "test", Label: "Named group", ShowHeading: heading, Collapsible: accordion, Widgets: []Widget{{ID: "one", Content: templ.Raw("One")}}}
			var b bytes.Buffer
			if err := DashboardSection(g).Render(t.Context(), &b); err != nil {
				t.Fatal(err)
			}
			html := b.String()
			if strings.Contains(html, "<details") != accordion || strings.Contains(html, `class="sr-only"`) == heading || !strings.Contains(html, `aria-label="Named group"`) {
				t.Fatalf("wrong heading=%v accordion=%v presentation: %s", heading, accordion, html)
			}
			if accordion && !heading && !strings.Contains(html, "1 item") {
				t.Fatal("hidden heading left an unnamed visible disclosure")
			}
			if accordion && !strings.Contains(html, `data-dashboard-disclosure="test" open`) {
				t.Fatal("accordion must start open by default")
			}
		}
	}
	var b bytes.Buffer
	if err := DashboardSection(dashboardSection{ID: "closed", Label: "Closed", Collapsible: true, InitiallyCollapsed: true}).Render(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), `data-dashboard-disclosure="closed" open`) {
		t.Fatal("initially collapsed preference ignored")
	}
}
