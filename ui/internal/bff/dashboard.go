package bff

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

type DashboardCatalog interface {
	Dashboard(context.Context) (api.DashboardSnapshot, error)
	SelectWidget(context.Context, string, string, bool) error
}

func (c *APIClient) Dashboard(ctx context.Context) (api.DashboardSnapshot, error) {
	r, err := c.client.GetDashboardWithResponse(ctx)
	if err != nil {
		return api.DashboardSnapshot{}, &upstreamError{operation: "dashboard", cause: err}
	}
	if r.JSON200 == nil {
		return api.DashboardSnapshot{}, &upstreamError{operation: "dashboard", status: r.StatusCode()}
	}
	return *r.JSON200, nil
}
func (c *APIClient) SelectWidget(ctx context.Context, kind, id string, enabled bool) error {
	r, err := c.client.SetDashboardSelectionWithResponse(ctx, kind, id, api.SetDashboardSelectionJSONRequestBody{Enabled: enabled})
	if err != nil {
		return &upstreamError{operation: "widget selection", cause: err}
	}
	if r.StatusCode() != 204 {
		return &upstreamError{operation: "widget selection", status: r.StatusCode()}
	}
	return nil
}

// Renderers own the component, including whether it has any card chrome.
type WidgetRenderer func(api.DashboardWidget) (view.Widget, error)

func widgetRenderers() map[string]WidgetRenderer {
	return map[string]WidgetRenderer{
		"service-links/card": func(item api.DashboardWidget) (view.Widget, error) {
			var service api.ServiceCandidate
			if err := decodeWidget(item, &service); err != nil {
				return view.Widget{}, err
			}
			value := mapService(service)
			return view.Widget{ID: item.Id, Content: view.ServiceCard(value, view.ServiceCardIcon(value))}, nil
		},
		"host-metrics/card": func(item api.DashboardWidget) (view.Widget, error) {
			var host api.Host
			if err := decodeWidget(item, &host); err != nil {
				return view.Widget{}, err
			}
			host.Id = item.SourceId
			host.Name = item.Title
			return view.Widget{ID: item.Id, Content: view.HostWidget(hostView(host), item.Error)}, nil
		},
	}
}
func decodeWidget(item api.DashboardWidget, target any) error {
	raw, err := json.Marshal(item.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
func renderWidgets(snapshot api.DashboardSnapshot, renderers map[string]WidgetRenderer) *view.Dashboard {
	result := &view.Dashboard{Unavailable: snapshot.Unavailable}
	for _, item := range snapshot.Items {
		widget := view.Widget{ID: item.Id, Content: view.UnavailableWidget(item.Title)}
		if render, ok := renderers[item.Kind+"/"+item.Presentation]; ok {
			if rendered, err := render(item); err == nil {
				widget = rendered
			}
		}
		result.Widgets = append(result.Widgets, widget)
		if item.Kind == "service-links" && item.Error == "" {
			var service api.ServiceCandidate
			if decodeWidget(item, &service) == nil {
				result.Services = append(result.Services, mapService(service))
			}
		}
	}
	return result
}
func (s *server) homepage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := view.PageData{Title: "Homepage", Description: "Pinned services and host metrics across your homelab.", Active: "nav-home", Notice: noticeText(r.URL.Query().Get("notice"))}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	catalog, ok := s.catalog.(DashboardCatalog)
	if !ok {
		data.Error = "Unable to load dashboard. Refresh to retry."
		s.render(w, r, data, 503)
		return
	}
	snapshot, err := catalog.Dashboard(ctx)
	if err != nil {
		data.Error = "Unable to load dashboard. Refresh to retry."
		s.render(w, r, data, 503)
		return
	}
	data.Dashboard = renderWidgets(snapshot, s.widgets)
	data.Services = data.Dashboard.Services
	s.render(w, r, data, 200)
}
func (s *server) hostSelection(ctx context.Context, id string) (bool, error) {
	catalog, ok := s.catalog.(DashboardCatalog)
	if !ok {
		return false, errors.New("dashboard unavailable")
	}
	snapshot, err := catalog.Dashboard(ctx)
	if err != nil {
		return false, err
	}
	for _, source := range snapshot.Unavailable {
		if source == "host" {
			return false, errors.New("host selection unavailable")
		}
	}
	for _, item := range snapshot.Items {
		if item.SourceKind == "host" && item.SourceId == id {
			return true, nil
		}
	}
	return false, nil
}
func (s *server) selectHost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	id := r.PathValue("hostID")
	window := "1h"
	message := ""
	status := 200
	desired := false
	if err := r.ParseForm(); err != nil {
		message = "Unable to read selection."
		status = 400
	} else {
		if r.PostForm.Get("range") == "24h" {
			window = "24h"
		}
		values := r.PostForm["enabled"]
		if len(values) == 0 && r.PostForm.Get("selection_form") == "host" {
			desired = false
		} else if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			message = "Invalid selection. Refresh and try again."
			status = 400
		} else {
			desired = values[0] == "true"
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	catalog, ok := s.catalog.(DashboardCatalog)
	if !ok {
		message = "Selection unavailable. Refresh to retry."
		status = 503
	}
	if message == "" {
		if err := catalog.SelectWidget(ctx, "host", id, desired); err != nil {
			message = "Could not save selection. Try again."
			status = 503
			var upstream *upstreamError
			if errors.As(err, &upstream) {
				switch upstream.status {
				case 400, 404, 409:
					status = upstream.status
					message = "Host unavailable or selection limit reached. Refresh before retrying."
				}
			}
		}
	}
	selected, selectErr := s.hostSelection(ctx, id)
	if selectErr != nil {
		selected = !desired
		if status == 200 {
			selected = desired
		}
	}
	if status == 200 && r.Header.Get("HX-Request-Type") != "partial" {
		target := "/hosts/" + url.PathEscape(id) + "?range=" + window
		redirect(w, r, target)
		return
	}
	if r.Header.Get("HX-Request-Type") == "partial" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = view.HostSelection(id, selected, false, window, message, true).Render(r.Context(), w)
		return
	}
	data := view.PageData{Title: "Host selection", Description: "Manage homepage widgets.", Error: message, Path: "/hosts/" + url.PathEscape(id) + "?range=" + window, Active: "nav-hosts"}
	s.render(w, r, data, status)
}
func selectionWarning(err error) string {
	if err != nil {
		return "Homepage selection is unavailable. Refresh to retry."
	}
	return ""
}

// refreshWidget re-resolves one selected widget without replacing its neighbours
// or restarting the dashboard stream. It reads the latest agent observations.
func (s *server) refreshWidget(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request-Type") != "partial" {
		redirect(w, r, "/")
		return
	}
	kind, id := r.PathValue("sourceKind"), r.PathValue("sourceID")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	failure := func(status int, message string) {
		w.Header().Set("HX-Retarget", "#widget-refresh-feedback-"+kind+"-"+id)
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(status)
		_ = view.WidgetRefreshFeedback(message).Render(r.Context(), w)
	}
	catalog, ok := s.catalog.(DashboardCatalog)
	if !ok {
		failure(503, "Refresh failed. Try again.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	snapshot, err := catalog.Dashboard(ctx)
	if err != nil {
		failure(503, "Refresh failed. Try again.")
		return
	}
	for _, unavailable := range snapshot.Unavailable {
		if unavailable == kind {
			failure(503, "Source unavailable. Try again.")
			return
		}
	}
	for _, item := range snapshot.Items {
		if item.SourceKind == kind && item.SourceId == id {
			page := renderWidgets(api.DashboardSnapshot{Items: []api.DashboardWidget{item}}, s.widgets)
			_ = view.WidgetRefreshed(page.Widgets[0]).Render(r.Context(), w)
			return
		}
	}
	failure(404, "Widget is no longer selected. Reload home.")
}
