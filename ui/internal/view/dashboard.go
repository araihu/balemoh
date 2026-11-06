package view

import (
	"encoding/hex"
	"github.com/a-h/templ"
	"github.com/araihu/goshtoso/components/layout"
)

type Arrangement struct {
	Type           string
	Columns, GapPx int
}
type DashboardLayout struct {
	Width, Revision string
	Page            Arrangement
	Groups          []dashboardSection
}
type dashboardSection struct {
	ID, Title, Label, Span                       string
	Widgets                                      []Widget
	Arrangement                                  Arrangement
	ShowHeading, Collapsible, InitiallyCollapsed bool
}

// DashboardGroup is the BFF's independent view of one resolved group.
type DashboardGroup = dashboardSection

func DOMKey(id string) string { return hex.EncodeToString([]byte(id)) }
func geometry(id, label string, a Arrangement) layout.Config {
	mode := layout.Mode(a.Type)
	if a.Type == "stack" || a.Type == "" {
		mode = layout.ModeStack
	}
	gap := layout.GapMedium
	if a.GapPx == 12 {
		gap = layout.GapSmall
	}
	if a.GapPx == 24 {
		gap = layout.GapLarge
	}
	columns := a.Columns
	if columns == 0 {
		columns = 1
	}
	return layout.Config{ID: id, Label: label, Mode: mode, Columns: columns, Gap: gap, Items: []layout.Item{}}
}
func dashboardSections(widgets []Widget) []dashboardSection {
	sections := []dashboardSection{{ID: "hosts", Title: "Hosts"}, {ID: "services", Title: "Services"}, {ID: "other", Title: "Widgets"}}
	for _, widget := range widgets {
		index := 2
		switch widget.SourceKind {
		case "host":
			index = 0
		case "service":
			index = 1
		}
		sections[index].Widgets = append(sections[index].Widgets, widget)
	}
	result := []dashboardSection{}
	for _, s := range sections {
		if len(s.Widgets) > 0 {
			s.Label = s.Title
			s.Arrangement = Arrangement{"masonry", 3, 16}
			result = append(result, s)
		}
	}
	return result
}
func dashboardContainer(page Dashboard) layout.Config {
	a := Arrangement{"stack", 1, 16}
	sections := dashboardSections(page.Widgets)
	if page.Layout != nil {
		a = page.Layout.Page
		sections = page.Layout.Groups
	}
	c := geometry("homepage-layout", "Homepage groups", a)
	for _, s := range sections {
		span := layout.SpanAuto
		if s.Span == "full" {
			span = layout.SpanFullWidth
		}
		c.Items = append(c.Items, layout.Item{ID: s.ID, Label: s.Label, Span: span, Content: DashboardSection(s), RootAttrs: templ.Attributes{"id": "layout-group-" + DOMKey(s.ID), "data-dashboard-group": s.ID}})
	}
	return c
}
func groupContainer(s dashboardSection) layout.Config {
	c := geometry("layout-items-"+DOMKey(s.ID), s.Label, s.Arrangement)
	c.RootClass = "balemoh-service-grid"
	for _, w := range s.Widgets {
		label := w.Title
		if label == "" {
			label = w.ID
		}
		c.Items = append(c.Items, layout.Item{ID: w.ID, Label: label, Content: WidgetContent(w), RootAttrs: templ.Attributes{"id": "layout-widget-" + DOMKey(w.ID)}})
	}
	return c
}

func dashboardStreamURL(defaults bool) string {
	if defaults {
		return "/dashboard/events?layoutView=default"
	}
	return "/dashboard/events"
}
