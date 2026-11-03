package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
	"io"
	"net/http"
	"strconv"
	"uuid"
)

type layoutSelection struct {
	GroupID string `json:"groupId"`
	ItemID  string `json:"itemId"`
}

func (s *server) layoutClient(w http.ResponseWriter) (LayoutCatalog, bool) {
	w.Header().Set("Cache-Control", "no-store")
	c, ok := s.catalog.(LayoutCatalog)
	if !ok {
		writeLayoutBFFError(w, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Layout service is unavailable."}})
	}
	return c, ok
}
func writeLayoutBFFError(w http.ResponseWriter, e error) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	var v *layoutFailure
	if !errors.As(e, &v) {
		v = &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Layout request could not be completed."}}
	}
	status := v.status
	if v.uncertain {
		// Browsers can retry a 408 POST before JavaScript receives the response.
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		api.ErrorResponse
		Uncertain bool `json:"uncertain"`
	}{v.detail, v.uncertain})
}
func decodeLayoutBFF(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	d.DisallowUnknownFields()
	e := d.Decode(v)
	if e == nil && d.Decode(new(any)) != io.EOF {
		e = errors.New("trailing JSON")
	}
	if e != nil {
		status, code := 400, "invalid_request"
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			status, code = 413, "request_too_large"
		}
		writeLayoutBFFError(w, &layoutFailure{status: status, detail: api.ErrorResponse{Code: code, Message: "Invalid or oversized layout request."}})
		return false
	}
	return true
}
func (s *server) layoutEditor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	data := view.PageData{Title: "Homepage layout", Path: "/settings/homepage/layout", Description: "Arrange homepage groups and items.", Active: "nav-homepage-layout"}
	c, ok := s.catalog.(LayoutCatalog)
	if !ok {
		data.Error = "Layout editor is unavailable."
		s.render(w, r, data, 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	var state api.LayoutState
	var preview api.LayoutPreview
	for attempt := 0; attempt < 2; attempt++ {
		var e error
		state, e = c.DashboardLayout(ctx)
		if e != nil {
			data.Error = e.Error()
			s.render(w, r, data, 503)
			return
		}
		raw, _ := json.Marshal(map[string]any{"requestId": uuid.NewV4().String(), "draftGeneration": 0, "expectedRevision": state.Revision, "document": state.Document})
		preview, e = c.PreviewLayout(ctx, raw)
		if e != nil {
			data.Error = e.Error()
			s.render(w, r, data, 503)
			return
		}
		if !preview.Stale && preview.LayoutRevision == state.Revision {
			break
		}
		if attempt == 1 {
			data.Error = "Layout changed while loading. Refresh to read the saved version."
			s.render(w, r, data, 503)
			return
		}
	}

	selection := layoutSelection{}
	if len(preview.ResolvedGroups) > 0 {
		selection.GroupID = preview.ResolvedGroups[0].GroupId
	}
	frame := s.layoutFrame(ctx, preview, selection)
	bootstrap, _ := json.Marshal(map[string]any{"state": state, "preview": preview, "selection": selection})
	data.LayoutEditor = &view.LayoutEditor{Bootstrap: string(bootstrap), Frame: frame}
	s.render(w, r, data, 200)
}
func (s *server) layoutPreview(w http.ResponseWriter, r *http.Request) {
	c, ok := s.layoutClient(w)
	if !ok {
		return
	}
	var body struct {
		Request   json.RawMessage `json:"request"`
		Selection layoutSelection `json:"selection"`
	}
	if !decodeLayoutBFF(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	preview, e := c.PreviewLayout(ctx, body.Request)
	if e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	frame := s.layoutFrame(ctx, preview, body.Selection)
	var markup bytes.Buffer
	if e = view.LayoutInspector(frame).Render(r.Context(), &markup); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	var deleteChoices bytes.Buffer
	if e = view.LayoutDeleteDestinations(frame).Render(r.Context(), &deleteChoices); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	var pageMarkup bytes.Buffer
	if e = view.LayoutPageArrangement(frame).Render(r.Context(), &pageMarkup); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	var movementMarkup bytes.Buffer
	if e = view.LayoutItemMovement(frame).Render(r.Context(), &movementMarkup); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	var previewMarkup bytes.Buffer
	if e = view.LayoutPreview(frame).Render(r.Context(), &previewMarkup); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"preview": preview, "pageHtml": pageMarkup.String(), "movementHtml": movementMarkup.String(), "inspectorHtml": markup.String(), "previewHtml": previewMarkup.String(), "deleteHtml": deleteChoices.String()})
}
func (s *server) layoutSave(w http.ResponseWriter, r *http.Request) {
	c, ok := s.layoutClient(w)
	if !ok {
		return
	}
	var body struct {
		ExpectedRevision string          `json:"expectedRevision"`
		Document         json.RawMessage `json:"document"`
	}
	if !decodeLayoutBFF(w, r, &body) {
		return
	}
	n, e := strconv.ParseInt(body.ExpectedRevision, 10, 64)
	if e != nil || n < 0 || strconv.FormatInt(n, 10) != body.ExpectedRevision {
		writeLayoutBFFError(w, &layoutFailure{status: 400, detail: api.ErrorResponse{Code: "invalid_request", Message: "Invalid expected revision."}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	state, e := c.SaveLayout(ctx, body.ExpectedRevision, body.Document)
	if e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
func (s *server) layoutSaved(w http.ResponseWriter, r *http.Request) {
	c, ok := s.layoutClient(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	state, e := c.DashboardLayout(ctx)
	if e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
func (s *server) layoutFrame(ctx context.Context, p api.LayoutPreview, selection layoutSelection) view.LayoutFrame {
	f := view.LayoutFrame{Width: string(p.Document.Homepage.Width), Page: arrangement(p.Document.Homepage.Layout), PageOrder: string(p.Document.Homepage.Ordering.Mode), SelectedGroup: selection.GroupID, SelectedItem: selection.ItemID, SourceGroups: map[string]string{"host": p.Document.SourceGroups.Host, "service": p.Document.SourceGroups.Service, "other": p.Document.SourceGroups.Other}, Targets: map[string]string{}}
	widgets := map[string]view.Widget{}
	if snapshot, e := s.dashboardSnapshot(ctx); e == nil {
		for _, w := range renderWidgets(snapshot, s.widgets).Widgets {
			widgets[w.ID] = w
		}
	}
	foundGroup, foundItem := false, false
	for _, r := range p.ResolvedGroups {
		g := view.EditorGroup{ID: r.GroupId, Title: r.Title, Label: r.DisplayLabel, Span: "auto", Order: "alphabetical"}
		if r.GroupId == selection.GroupID {
			foundGroup = true
		}
		for _, v := range p.Document.Groups {
			if v.Id == r.GroupId {
				g.Arrangement = arrangement(v.Layout)
				g.ShowHeading = v.ShowHeading
				g.InitiallyCollapsed = v.InitiallyCollapsed
				g.Order = string(v.Ordering.Mode)
			}
		}
		for _, v := range p.Document.Homepage.Placements {
			if v.GroupId == r.GroupId {
				g.Span = string(v.Span)
			}
		}
		for _, m := range r.Members {
			w, ok := widgets[m.ItemId]
			if !ok {
				w = view.Widget{ID: m.ItemId, Title: m.DisplayLabel, Content: view.UnavailableWidget(m.DisplayLabel)}
			}
			g.Members = append(g.Members, view.EditorMember{ID: m.ItemId, Label: m.DisplayLabel, State: string(m.State), Widget: w})
			if m.ItemId == selection.ItemID {
				foundItem = true
				f.SelectedGroup = r.GroupId
			}
		}
		f.Groups = append(f.Groups, g)
	}
	if !foundGroup && len(f.Groups) > 0 && f.SelectedItem == "" {
		f.SelectedGroup = f.Groups[0].ID
	}
	if !foundItem {
		f.SelectedItem = ""
	}
	if f.SelectedGroup == "" && len(f.Groups) > 0 {
		f.SelectedGroup = f.Groups[0].ID
	}
	return f
}
