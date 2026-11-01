package bff

import (
	"context"
	"errors"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) dashboardSnapshotView(ctx context.Context, defaults bool) (api.DashboardSnapshot, error) {
	if defaults {
		if c, ok := s.catalog.(interface {
			DashboardView(context.Context, bool) (api.DashboardSnapshot, error)
		}); ok {
			return c.DashboardView(ctx, true)
		}
		return api.DashboardSnapshot{}, errors.New("temporary dashboard unavailable")
	}
	return s.dashboardSnapshot(ctx)
}
func arrangement(c api.LayoutContainer) view.Arrangement {
	return view.Arrangement{Type: string(c.Type), Columns: c.Columns, GapPx: int(c.GapPx)}
}
func (s *server) renderDashboard(ctx context.Context, snapshot api.DashboardSnapshot, defaults bool, cached *api.LayoutState) (*view.Dashboard, error) {
	page := renderWidgets(snapshot, s.widgets)
	page.DefaultView = defaults
	if snapshot.Groups == nil {
		return page, nil
	}
	geometry := &view.DashboardLayout{Width: "full", Page: view.Arrangement{Type: "stack", Columns: 1, GapPx: 16}}
	page.Layout = geometry
	var doc *api.LayoutDocument
	if !defaults {
		c, ok := s.catalog.(LayoutCatalog)
		if !ok || snapshot.LayoutRevision == nil {
			return nil, errors.New("layout unavailable")
		}
		if cached == nil || cached.Revision != *snapshot.LayoutRevision {
			v, e := c.DashboardLayout(ctx)
			if e != nil {
				return nil, e
			}
			if v.Revision != *snapshot.LayoutRevision {
				return nil, errors.New("layout changed during snapshot")
			}
			if cached != nil {
				*cached = v
			} else {
				cached = &v
			}
		}
		doc = &cached.Document
		geometry.Width = string(doc.Homepage.Width)
		geometry.Page = arrangement(doc.Homepage.Layout)
		geometry.Revision = cached.Revision
	}
	byID := map[string]view.Widget{}
	for _, w := range page.Widgets {
		byID[w.ID] = w
	}
	for _, g := range *snapshot.Groups {
		section := view.DashboardGroup{ID: g.GroupId, Title: g.Title, Label: g.DisplayLabel, ShowHeading: true, Arrangement: view.Arrangement{Type: "masonry", Columns: 3, GapPx: 16}}
		if doc != nil {
			for _, setting := range doc.Groups {
				if setting.Id == g.GroupId {
					section.Arrangement = arrangement(setting.Layout)
					section.ShowHeading = setting.ShowHeading
					section.InitiallyCollapsed = setting.InitiallyCollapsed
				}
			}
			for _, p := range doc.Homepage.Placements {
				if p.GroupId == g.GroupId {
					section.Span = string(p.Span)
				}
			}
		}
		for _, id := range g.ItemIds {
			if w, ok := byID[id]; ok {
				section.Widgets = append(section.Widgets, w)
			} else {
				return nil, errors.New("layout member missing from snapshot")
			}
		}
		geometry.Groups = append(geometry.Groups, section)
	}
	return page, nil
}
