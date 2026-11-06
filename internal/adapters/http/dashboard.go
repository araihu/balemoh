package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/dashboard"
)

func (h Handler) WithDashboard(service *dashboard.Service) Handler { h.dashboard = service; return h }
func (h Handler) GetDashboard(w http.ResponseWriter, r *http.Request, params generated.GetDashboardParams) {
	w.Header().Set("Cache-Control", "no-store")
	defaults := params.LayoutView != nil
	if defaults && (*params.LayoutView != "default" || len(r.URL.Query()["layoutView"]) != 1) {
		writeLayoutError(w, dashboard.LayoutFailure(400, "invalid_request", "Unknown layout view."))
		return
	}
	if h.dashboard == nil {
		writeCatalogUnavailable(w)
		return
	}
	snapshot := h.dashboard.SnapshotView(r.Context(), defaults)
	if snapshot.LayoutError != nil {
		writeLayoutError(w, snapshot.LayoutError)
		return
	}
	result := generated.DashboardSnapshot{Items: []generated.DashboardWidget{}, Unavailable: snapshot.Unavailable}
	if snapshot.LayoutRevision != "" {
		result.LayoutRevision = &snapshot.LayoutRevision
	}
	if snapshot.Groups != nil {
		groups := []generated.DashboardGroup{}
		for _, g := range snapshot.Groups {
			groups = append(groups, generated.DashboardGroup{GroupId: g.GroupID, Title: g.Title, DisplayLabel: g.DisplayLabel, ItemIds: g.ItemIDs})
		}
		result.Groups = &groups
	}
	for _, item := range snapshot.Items {
		data := item.Data
		// Keep the existing public service DTO at the HTTP boundary.
		if candidate, ok := data.(catalog.Candidate); ok {
			data = serviceCandidate(candidate)
		}
		payload := map[string]interface{}{}
		if data != nil {
			raw, err := json.Marshal(data)
			if err != nil {
				writeCatalogUnavailable(w)
				return
			}
			if err = json.Unmarshal(raw, &payload); err != nil {
				writeCatalogUnavailable(w)
				return
			}
		}
		result.Items = append(result.Items, generated.DashboardWidget{Id: item.ID, Kind: item.Kind, Presentation: item.Presentation, SourceKind: item.SourceKind, SourceId: item.SourceID, Title: item.Title, Data: payload, Error: item.Error})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}
func (h Handler) SetDashboardSelection(w http.ResponseWriter, r *http.Request, kind, id string) {
	if h.dashboard == nil {
		writeCatalogUnavailable(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decoder.Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, "Invalid selection", 400)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid selection", 400)
		return
	}
	err := h.dashboard.SetSelected(r.Context(), kind, id, *body.Enabled)
	switch {
	case errors.Is(err, dashboard.ErrSource):
		http.Error(w, "Unsupported widget source", 400)
	case errors.Is(err, dashboard.ErrNotFound), errors.Is(err, catalog.ErrNotFound):
		http.Error(w, "Source not found", 404)
	case errors.Is(err, dashboard.ErrLimit), errors.Is(err, catalog.ErrConflict):
		http.Error(w, "Selection limit reached or source state changed", 409)
	case err != nil:
		writeCatalogUnavailable(w)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
