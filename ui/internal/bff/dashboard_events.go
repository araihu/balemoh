package bff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) dashboardEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("layoutEditor") == "true" {
		s.layoutEditorEvents(w, r)
		return
	}
	s.streamDashboard(w, r, 5*time.Second)
}
func (s *server) streamDashboard(w http.ResponseWriter, r *http.Request, interval time.Duration) {
	_, ok := s.catalog.(DashboardCatalog)
	if !ok {
		http.Error(w, "Dashboard unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var previous [sha256.Size]byte
	var retained []api.DashboardWidget
	var published []api.DashboardWidget
	var publishedLayout string
	var cachedLayout api.LayoutState
	defaults := r.URL.Query().Get("layoutView") == "default"
	failed := false
	for {
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		snapshot, err := s.dashboardSnapshotView(ctx, defaults)
		cancel()
		var component templ.Component
		if err != nil {
			if !failed {
				component = view.DashboardLiveStatus("Updates unavailable. Retrying; showing last received values.")
			}
			failed = true
		} else {
			// Preserve only failed providers; successful providers still update or remove items.
			unavailable := map[string]bool{}
			for _, kind := range snapshot.Unavailable {
				unavailable[kind] = true
			}
			currentIDs := map[string]int{}
			for i, w := range snapshot.Items {
				currentIDs[w.Id] = i
			}
			for _, item := range retained {
				if !unavailable[item.SourceKind] {
					continue
				}
				if index, ok := currentIDs[item.Id]; ok {
					snapshot.Items[index].Data = item.Data
					snapshot.Items[index].Error = item.Error
				} else if snapshot.Groups == nil {
					snapshot.Items = append(snapshot.Items, item)
				}
			}

			encoded, err := json.Marshal(snapshot)
			if err != nil {
				return
			}
			digest := sha256.Sum256(encoded)
			if digest != previous || failed {
				message := "Live updates connected"
				if len(snapshot.Unavailable) > 0 {
					message = "Some sources unavailable; showing last received values."
				}
				if previous == ([sha256.Size]byte{}) && len(snapshot.Unavailable) > 0 && snapshot.Groups == nil {
					// On initial connect/reconnect the DOM may already contain data
					// absent from this stream's retained snapshot. Do not erase it.
					component = view.DashboardLiveUnavailable(snapshot.Unavailable)
				} else {
					changed, structureChanged := dashboardWidgetChanges(published, snapshot.Items)
					layoutRaw, _ := json.Marshal(struct {
						Revision *api.LayoutRevision
						Groups   *[]api.DashboardGroup
					}{snapshot.LayoutRevision, snapshot.Groups})
					structureChanged = structureChanged || string(layoutRaw) != publishedLayout
					if previous == ([sha256.Size]byte{}) || structureChanged {
						renderCtx, renderCancel := context.WithTimeout(r.Context(), s.timeout)
						page, renderErr := s.renderDashboard(renderCtx, snapshot, defaults, &cachedLayout)
						renderCancel()
						if renderErr != nil {
							component = view.DashboardLiveStatus("Layout changed during refresh. Retrying; showing last received values.")
							failed = true
						} else {
							component = view.DashboardLiveSnapshot(*page, message)
						}
					} else {
						component = view.DashboardLiveWidgets(*renderWidgets(api.DashboardSnapshot{Items: changed, Unavailable: snapshot.Unavailable}, s.widgets), message)
					}
					if !failed {
						published = snapshot.Items
						publishedLayout = string(layoutRaw)
						previous = digest
					}
				}
			}
			retained = snapshot.Items
			failed = false
		}
		var frame bytes.Buffer
		if component == nil {
			frame.WriteString(": keepalive\n\n")
		} else {
			var html bytes.Buffer
			if err := component.Render(r.Context(), &html); err != nil {
				return
			}
			for line := range bytes.SplitSeq(html.Bytes(), []byte("\n")) {
				frame.WriteString("data: ")
				frame.Write(line)
				frame.WriteByte('\n')
			}
			frame.WriteByte('\n')
		}
		if err := controller.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return
		}
		if _, err := w.Write(frame.Bytes()); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

// A membership, ordering or source change needs section reconciliation. Values
// alone update only their widget, preserving the grid and browser scroll anchor.
func dashboardWidgetChanges(previous, current []api.DashboardWidget) ([]api.DashboardWidget, bool) {
	if len(previous) != len(current) {
		return current, true
	}
	var changed []api.DashboardWidget
	for i, item := range current {
		if previous[i].Id != item.Id || previous[i].SourceKind != item.SourceKind || previous[i].Kind != item.Kind || previous[i].Presentation != item.Presentation || previous[i].Title != item.Title {
			return current, true
		}
		before, _ := json.Marshal(previous[i])
		after, _ := json.Marshal(item)
		if !bytes.Equal(before, after) {
			changed = append(changed, item)
		}
	}
	return changed, false
}
