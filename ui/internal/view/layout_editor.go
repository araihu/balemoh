package view

import (
	"encoding/json"
	"github.com/a-h/templ"
	"github.com/araihu/goshtoso/components/form"
	"github.com/araihu/goshtoso/components/layout"
	selectfield "github.com/araihu/goshtoso/components/select"
	"github.com/araihu/goshtoso/components/table"
	"strconv"
)

type LayoutEditor struct {
	Bootstrap string
	Frame     LayoutFrame
}
type LayoutFrame struct {
	Width                       string
	Page                        Arrangement
	PageOrder                   string
	Groups                      []EditorGroup
	SelectedGroup, SelectedItem string
	SourceGroups                map[string]string
	Targets                     map[string]string
}
type EditorGroup struct {
	ExplicitCount                   int
	ID, Title, Label, Span, Order   string
	Arrangement                     Arrangement
	ShowHeading, InitiallyCollapsed bool
	Members                         []EditorMember
}
type EditorMember struct {
	ID, Label, State string
	Widget           Widget
}

func (f LayoutFrame) group() EditorGroup {
	for _, g := range f.Groups {
		if g.ID == f.SelectedGroup {
			return g
		}
	}
	return EditorGroup{}
}
func (f LayoutFrame) groupOptions(selected string) []selectfield.Option {
	out := []selectfield.Option{}
	for _, g := range f.Groups {
		out = append(out, selectfield.Option{Value: g.ID, Label: g.Label, Selected: g.ID == selected})
	}
	return out
}
func (f LayoutFrame) itemOptions() []selectfield.Option {
	out := []selectfield.Option{{Value: "", Label: "Choose an item", Selected: f.SelectedItem == ""}}
	for _, m := range f.group().Members {
		label := m.Label
		if m.State == "dormant" {
			label += " · not currently on home"
		}
		out = append(out, selectfield.Option{Value: m.ID, Label: label, Selected: m.ID == f.SelectedItem})
	}
	return out
}
func (f LayoutFrame) targetsJSON() string { b, _ := json.Marshal(f.Targets); return string(b) }
func choice(id, label, value string, options []string, attrs templ.Attributes) form.FieldGroupConfig {
	choices := []selectfield.Option{}
	for _, v := range options {
		choices = append(choices, selectfield.Option{Value: v, Label: v, Selected: v == value})
	}
	field := form.FieldGroupConfig{ID: id + "-field", Select: &selectfield.Config{ID: id, Label: label, Options: choices, InputAttrs: attrs}}
	field.Select.InputAttrs["data-layout-focus"] = field.FocusTargetID()
	return field
}
func geometryChoice(prefix, path string, a Arrangement) []form.FieldGroupConfig {
	return []form.FieldGroupConfig{
		choice(prefix+"-type", "Arrangement", a.Type, []string{"stack", "grid", "masonry"}, templ.Attributes{"data-layout-field": path + "/type"}),
		choice(prefix+"-columns", "Maximum columns", strconv.Itoa(a.Columns), []string{"1", "2", "3", "4"}, templ.Attributes{"data-layout-field": path + "/columns", "data-layout-number": ""}),
		choice(prefix+"-gap", "Spacing", strconv.Itoa(a.GapPx), []string{"12", "16", "24"}, templ.Attributes{"data-layout-field": path + "/gapPx", "data-layout-number": ""}),
	}
}
func (f LayoutFrame) previewContainer() layout.Config {
	c := geometry("layout-editor-groups", "Homepage groups", f.Page)
	c.Reorder = &layout.ReorderConfig{Group: "balemoh-layout-groups"}
	for _, g := range f.Groups {
		attrs := templ.Attributes{"id": "editor-group-" + DOMKey(g.ID), "data-editor-group": g.ID}
		if f.SelectedGroup == g.ID {
			attrs["aria-current"] = "true"
		}
		span := layout.SpanAuto
		if g.Span == "full" {
			span = layout.SpanFullWidth
		}
		c.Items = append(c.Items, layout.Item{ID: g.ID, Label: g.Label, Span: span, Content: LayoutPreviewGroup(f, g), RootAttrs: attrs})
	}
	return c
}
func (f LayoutFrame) itemsContainer(g EditorGroup) layout.Config {
	c := geometry("editor-items-"+DOMKey(g.ID), g.Label, g.Arrangement)
	c.Reorder = &layout.ReorderConfig{Group: "balemoh-layout-items"}
	for _, m := range g.Members {
		if m.State != "active" {
			continue
		}
		attrs := templ.Attributes{"id": "editor-item-" + DOMKey(m.ID), "data-editor-item": m.ID}
		if f.SelectedItem == m.ID {
			attrs["aria-current"] = "true"
		}
		c.Items = append(c.Items, layout.Item{ID: m.ID, Label: m.Label, Content: LayoutPreviewItem(f, m), RootAttrs: attrs})
	}
	return c
}
func layoutCapacity(f LayoutFrame) string {
	count := 0
	explicit := 0
	for _, g := range f.Groups {
		count += len(g.Members)
		explicit += g.ExplicitCount
	}
	return strconv.Itoa(len(f.Groups)) + " of 32 groups. " + strconv.Itoa(count) + " complete members. " + strconv.Itoa(explicit) + " of 512 explicit references. Larger default groups remain visible; complete custom ordering must fit this limit."
}

type LayoutComparisonRow struct {
	ID, Setting, Draft, Saved string
	Changed                   bool
}

func layoutComparisonTable(rows []LayoutComparisonRow, changedOnly bool) table.Config {
	config := table.Config{ID: "layout-comparison-all", Caption: "All layout settings", Columns: []table.Column{{Key: "setting", Label: "Setting"}, {Key: "draft", Label: "Your draft"}, {Key: "saved", Label: "Saved version"}}, Mobile: &table.MobileConfig{PrimaryColumn: "setting"}, Rows: []table.Row{}}
	if changedOnly {
		config.ID = "layout-comparison-differences"
		config.Caption = "Draft and saved layout differences"
	}
	for _, r := range rows {
		if changedOnly && !r.Changed {
			continue
		}
		config.Rows = append(config.Rows, table.Row{ID: config.ID + "-" + DOMKey(r.ID), Cells: map[string]table.Cell{"setting": {Text: r.Setting}, "draft": {Text: r.Draft}, "saved": {Text: r.Saved}}})
	}
	return config
}

func (f LayoutFrame) selectedItemDormant() bool {
	for _, g := range f.Groups {
		for _, m := range g.Members {
			if m.ID == f.SelectedItem {
				return m.State == "dormant"
			}
		}
	}
	return false
}

func (f LayoutFrame) selectedActiveGroup(id string) bool {
	for _, g := range f.Groups {
		if g.ID == id {
			for _, m := range g.Members {
				if m.ID == f.SelectedItem && m.State == "active" {
					return true
				}
			}
		}
	}
	return false
}
