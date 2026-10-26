package bff

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/plugins"
	"github.com/araihu/balemoh/ui/internal/view"
	guuid "github.com/google/uuid"
)

// A not-yet-configured row has a stable submission identity across tabs/retries.
// Existing placement UUIDs and their independent options remain authoritative.
func defaultWidgetID(plugin, connection, contribution string) string {
	return guuid.NewSHA1(guuid.NameSpaceURL, []byte("balemoh/widget/default/v1/"+plugin+"/"+connection+"/"+contribution)).String()
}
func widgetKey(plugin, connection, contribution string) string {
	return plugin + "\x00" + connection + "\x00" + contribution
}
func (s *server) savedWidgetRow(c plugins.WidgetCatalog, p plugins.PlacementRecord) view.PluginWidgetRow {
	d, ok := widgetDefinition(c, p.PluginID, p.ContributionID)
	title := d.Title
	if !ok {
		title = "Unavailable widget " + p.ContributionID
	}
	plugin, connection := s.widgetNames(p.PluginID, p.ConnectionID)
	return view.PluginWidgetRow{ID: p.PlacementID, Title: title, Plugin: plugin, Connection: connection, URL: widgetEditURL(p.PlacementID), SelectionURL: "/plugin-widgets/" + p.PlacementID + "/selection", Revision: p.Revision, Enabled: p.Enabled, Saved: true, PluginID: p.PluginID, ConnectionID: p.ConnectionID, ContributionID: p.ContributionID}
}
func (s *server) availableWidgetRow(c plugins.WidgetCatalog, plugin, connection, contribution string) (view.PluginWidgetRow, bool) {
	e := s.plugins.Connection(plugin, connection)
	d, ok := widgetDefinition(c, plugin, contribution)
	if e == nil || !ok {
		return view.PluginWidgetRow{}, false
	}
	name, connectionName := s.widgetNames(plugin, connection)
	return view.PluginWidgetRow{ID: defaultWidgetID(plugin, connection, contribution), Title: d.Title, Plugin: name, Connection: connectionName, URL: widgetNewURL(plugin, connection, contribution), SelectionURL: widgetManagementURL(plugin, connection) + "/" + contribution + "/selection", PluginID: plugin, ConnectionID: connection, ContributionID: contribution}, true
}
func (s *server) widgetRows(c plugins.WidgetCatalog) []view.PluginWidgetRow {
	rows := []view.PluginWidgetRow{}
	represented := map[string]bool{}
	for _, p := range c.Placements {
		rows = append(rows, s.savedWidgetRow(c, p))
		represented[widgetKey(p.PluginID, p.ConnectionID, p.ContributionID)] = true
	}
	for _, e := range s.plugins.Entries() {
		if !e.Compatible || e.Package == nil {
			continue
		}
		for _, d := range c.Definitions {
			if d.PluginID != e.Package.Manifest.ID || represented[widgetKey(d.PluginID, e.Installation.ConnectionID, d.ContributionID)] {
				continue
			}
			if row, ok := s.availableWidgetRow(c, d.PluginID, e.Installation.ConnectionID, d.ContributionID); ok {
				rows = append(rows, row)
			}
		}
	}
	slices.SortFunc(rows, func(a, b view.PluginWidgetRow) int {
		return cmp.Or(strings.Compare(a.Plugin, b.Plugin), strings.Compare(a.Connection, b.Connection), strings.Compare(a.Title, b.Title), strings.Compare(a.ID, b.ID))
	})
	return rows
}
func defaultWidgetOptions(schema []byte) (json.RawMessage, error) {
	fields, err := manifest.FormFields(schema)
	if err != nil {
		return nil, err
	}
	values := map[string]json.RawMessage{}
	for _, f := range fields {
		if len(f.Default) > 0 {
			values[f.Name] = f.Default
		}
	}
	return json.Marshal(values)
}
func (s *server) pluginWidgetSelection(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	status, message := 200, ""
	if r.ParseForm() != nil {
		status = 400
		message = "Unable to read selection."
	}
	desired := false
	values := r.PostForm["enabled"]
	if message == "" {
		if len(values) == 0 && r.PostForm.Get("selection_form") == "plugin-widget" {
			desired = false
		} else if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			status = 400
			message = "Invalid selection. Refresh and try again."
		} else {
			desired = values[0] == "true"
		}
	}
	list := r.PostForm.Get("selection_context") == "widgets"
	id := r.PathValue("placementID")
	create := id == ""
	if create {
		id = defaultWidgetID(r.PathValue("pluginID"), r.PathValue("connectionID"), r.PathValue("contributionID"))
	}
	row := view.PluginWidgetRow{ID: id, Title: "widget", SelectionURL: r.URL.Path, Enabled: r.PostForm.Get("previous") == "true", Disabled: true}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	catalog, err := s.plugins.WidgetCatalog(ctx)
	var placement plugins.PlacementRecord
	found := false
	if err == nil {
		for _, p := range catalog.Placements {
			if p.PlacementID == id {
				placement = p
				row = s.savedWidgetRow(catalog, p)
				found = true
				break
			}
		}
		if !found && create {
			var ok bool
			row, ok = s.availableWidgetRow(catalog, r.PathValue("pluginID"), r.PathValue("connectionID"), r.PathValue("contributionID"))
			if !ok {
				row = view.PluginWidgetRow{ID: id, Title: "widget", SelectionURL: r.URL.Path, Disabled: true}
				status = 404
				message = "Widget unavailable. Refresh before retrying."
			} else {
				placement = plugins.PlacementRecord{PlacementID: id, PluginID: row.PluginID, ConnectionID: row.ConnectionID, ContributionID: row.ContributionID}
				// A newly configured sibling replaces the available row. A stale list may
				// not silently create an additional independent placement.
				for _, p := range catalog.Placements {
					if widgetKey(p.PluginID, p.ConnectionID, p.ContributionID) == widgetKey(placement.PluginID, placement.ConnectionID, placement.ContributionID) {
						status = 409
						message = "Widget list changed. Refresh before retrying."
						row.Disabled = true
						break
					}
				}
			}
		} else if !found {
			status = 404
			message = "Saved widget not found. Refresh before retrying."
			row.Disabled = true
		}
	} else {
		status = 503
		message = "Selection unavailable. Refresh before retrying."
	}
	revisionMatched := found && r.PostForm.Get("revision") == placement.Revision
	if message == "" && (!create || desired) {
		placement.Enabled = desired
		if create && !found {
			d, _ := widgetDefinition(catalog, placement.PluginID, placement.ContributionID)
			placement.Options, err = defaultWidgetOptions(d.OptionsSchema)
		} else if !create {
			placement.Revision = r.PostForm.Get("revision")
			if placement.Revision == "" {
				status = 428
				message = "Widget revision required. Refresh before retrying."
			}
		}
		if err == nil && message == "" {
			var saved plugins.PlacementRecord
			saved, err = s.plugins.SavePlacement(ctx, placement, create)
			if err == nil {
				row = s.savedWidgetRow(catalog, saved)
			}
		}
		if err != nil {
			status = 409
			message = "Could not save selection. Check widget options or refresh before retrying."
			row.Disabled = true
			if current, e := s.plugins.WidgetCatalog(ctx); e == nil {
				for _, p := range current.Placements {
					if p.PlacementID == id {
						row = s.savedWidgetRow(current, p)
						break
					}
				}
			}
		}
	}
	row.Error = message
	if status == 200 && r.Header.Get("HX-Request-Type") != "partial" {
		target := widgetIndexURL
		if !list && row.Saved {
			target = widgetEditURL(row.ID)
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	if r.Header.Get("HX-Request-Type") == "partial" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = view.PluginWidgetSelection(row, list, true).Render(r.Context(), w)
		if list && create && row.Saved {
			_ = view.PluginWidgetNameUpdate(row).Render(r.Context(), w)
		}
		if !list && status == 200 && revisionMatched {
			if d, ok := widgetDefinition(catalog, row.PluginID, row.ContributionID); ok {
				if _, err := manifest.FormFields(d.OptionsSchema); err == nil {
					_ = view.PluginWidgetOptionsSelectionUpdate(row).Render(r.Context(), w)
				}
			}
		}
		return
	}
	s.widgetConfigurationError(w, r, message, status)
}
