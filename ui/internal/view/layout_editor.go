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
		choices = append(choices, selectfield.Option{Value: v, Label: layoutChoiceLabel(v), Selected: v == value})
	}
	field := form.FieldGroupConfig{ID: id + "-field", Select: &selectfield.Config{ID: id, Label: label, Options: choices, InputAttrs: attrs}}
	field.Select.InputAttrs["data-layout-focus"] = field.FocusTargetID()
	return field
}
func (f LayoutFrame) previewContainer() layout.Config {
	c := geometry("layout-editor-groups", "Homepage groups", f.Page)
	c.Reorder = &layout.ReorderConfig{Group: "balemoh-layout-groups"}
	c.RootClass = "balemoh-layout-canvas"
	for _, g := range f.Groups {
		attrs := templ.Attributes{"id": "editor-group-" + DOMKey(g.ID), "data-editor-group": g.ID}
		if f.SelectedGroup == g.ID && f.SelectedItem == "" {
			attrs["aria-current"] = "true"
		}
		span := layout.SpanAuto
		if g.Span == "full" {
			span = layout.SpanFullWidth
		}
		c.Items = append(c.Items, layout.Item{ID: g.ID, Label: g.Label, Span: span, Content: LayoutPreviewGroup(f, g), RootClass: "balemoh-layout-group", RootAttrs: attrs})
	}
	return c
}
func (f LayoutFrame) itemsContainer(g EditorGroup) layout.Config {
	c := geometry("editor-items-"+DOMKey(g.ID), g.Label, g.Arrangement)
	c.Reorder = &layout.ReorderConfig{Group: "balemoh-layout-items"}
	c.RootClass = "balemoh-layout-group-items"
	if g.activeCount() == 0 {
		c.RootClass += " balemoh-layout-empty-target"
	}
	for _, m := range g.Members {
		if m.State != "active" {
			continue
		}
		attrs := templ.Attributes{"id": "editor-item-" + DOMKey(m.ID), "data-editor-item": m.ID}
		if f.SelectedItem == m.ID {
			attrs["aria-current"] = "true"
		}
		c.Items = append(c.Items, layout.Item{ID: m.ID, Label: m.Label, Content: LayoutPreviewItem(f, m), RootClass: "balemoh-layout-item", RootAttrs: attrs})
	}
	return c
}
func (f LayoutFrame) groupPosition() int {
	for i, g := range f.Groups {
		if g.ID == f.SelectedGroup {
			return i + 1
		}
	}
	return 0
}
func (f LayoutFrame) itemPosition() int {
	for i, m := range f.group().Members {
		if m.ID == f.SelectedItem {
			return i + 1
		}
	}
	return 0
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

func layoutModeLabel(value string) string {
	switch value {
	case "stack":
		return "List"
	case "masonry":
		return "Masonry"
	default:
		return "Grid"
	}
}
func layoutChoiceLabel(value string) string {
	switch value {
	case "full":
		return "Full width"
	case "contained":
		return "Centered"
	case "auto":
		return "Fit column"
	case "alphabetical":
		return "Alphabetical"
	case "custom":
		return "Manual"
	case "12":
		return "Tight"
	case "16":
		return "Normal"
	case "24":
		return "Wide"
	default:
		return value
	}
}
func sourceGroupLabel(kind string) string {
	switch kind {
	case "host":
		return "Hosts"
	case "service":
		return "Services"
	default:
		return "Widgets and other items"
	}
}
func (f LayoutFrame) widthLabel() string { return layoutChoiceLabel(f.Width) }
func (f LayoutFrame) selectionLabel() string {
	for _, g := range f.Groups {
		for _, m := range g.Members {
			if m.ID == f.SelectedItem {
				return m.Label
			}
		}
	}
	return f.group().Label
}
func (g EditorGroup) activeCount() int {
	n := 0
	for _, m := range g.Members {
		if m.State == "active" {
			n++
		}
	}
	return n
}
func (g EditorGroup) activeCountLabel() string {
	if n := g.activeCount(); n == 1 {
		return "1 item"
	} else {
		return strconv.Itoa(n) + " items"
	}
}
func (g EditorGroup) dormantCount() int { return len(g.Members) - g.activeCount() }
func (g EditorGroup) dormantCountLabel() string {
	return strconv.Itoa(g.dormantCount()) + " items not on homepage"
}
