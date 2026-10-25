package bff

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"uuid"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/plugins"
	"github.com/araihu/balemoh/ui/internal/view"
)

const widgetIndexURL = "/plugins/widgets"
const widgetNavigationID = "nav-plugin-widgets"

func widgetManagementURL(plugin, connection string) string {
	return "/plugins/" + plugin + "/connections/" + connection + "/widgets"
}
func widgetEditURL(id string) string { return "/plugin-widgets/" + id + "/edit" }
func widgetNewURL(plugin, connection, contribution string) string {
	return widgetManagementURL(plugin, connection) + "/" + contribution + "/new"
}

// Keep existing bookmarks useful without exposing every editor on one page.
func (s *server) pluginWidgets(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, widgetIndexURL, http.StatusFound)
}
func widgetDefinition(c plugins.WidgetCatalog, plugin, contribution string) (plugins.Definition, bool) {
	for _, d := range c.Definitions {
		if d.PluginID == plugin && d.ContributionID == contribution {
			return d, true
		}
	}
	return plugins.Definition{}, false
}
func (s *server) widgetNames(plugin, connection string) (string, string) {
	if e := s.plugins.Connection(plugin, connection); e != nil {
		return e.Package.Manifest.Name, e.ConnectionName
	}
	return plugin, connection
}
func (s *server) widgetConfigurationError(w http.ResponseWriter, r *http.Request, message string, status int) {
	s.render(w, r, view.PageData{Title: "Plugins/Widgets", Description: "Manage plugin widget configuration.", Path: r.URL.Path, Active: widgetNavigationID, PluginContent: view.PluginWidgetsRecovery(message)}, status)
}
func (s *server) pluginWidgetsIndex(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	c, err := s.plugins.WidgetCatalog(ctx)
	if err != nil {
		s.widgetConfigurationError(w, r, "Widget configuration is unavailable. Try again.", 503)
		return
	}
	page := view.PluginWidgetList{}
	for _, p := range c.Placements {
		d, ok := widgetDefinition(c, p.PluginID, p.ContributionID)
		title := d.Title
		if !ok {
			title = "Unavailable widget " + p.ContributionID
		}
		plugin, connection := s.widgetNames(p.PluginID, p.ConnectionID)
		page.Saved = append(page.Saved, view.PluginWidgetRow{ID: p.PlacementID, Title: title, Plugin: plugin, Connection: connection, URL: widgetEditURL(p.PlacementID), Enabled: p.Enabled})
	}
	for _, e := range s.plugins.Entries() {
		if !e.Compatible || e.Package == nil {
			continue
		}
		for _, d := range c.Definitions {
			if d.PluginID != e.Package.Manifest.ID {
				continue
			}
			page.Available = append(page.Available, view.PluginWidgetRow{ID: plugins.NavID(d.PluginID, e.Installation.ConnectionID, d.ContributionID), Title: d.Title, Plugin: e.Package.Manifest.Name, Connection: e.ConnectionName, URL: widgetNewURL(d.PluginID, e.Installation.ConnectionID, d.ContributionID)})
		}
	}
	sortRows := func(rows []view.PluginWidgetRow) {
		slices.SortFunc(rows, func(a, b view.PluginWidgetRow) int {
			return cmp.Or(strings.Compare(a.Plugin, b.Plugin), strings.Compare(a.Connection, b.Connection), strings.Compare(a.Title, b.Title), strings.Compare(a.ID, b.ID))
		})
	}
	sortRows(page.Saved)
	sortRows(page.Available)
	s.render(w, r, view.PageData{Title: "Plugins/Widgets", Description: "Manage plugin widgets and their individual configuration.", Path: r.URL.Path, Active: widgetNavigationID, PluginContent: view.PluginWidgetsIndex(page)}, 200)
}
func (s *server) pluginWidgetEdit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	p, err := s.plugins.CurrentPlacement(ctx, r.PathValue("placementID"))
	if err != nil {
		status := 503
		message := "Widget configuration is unavailable. Try again."
		if errors.Is(err, plugins.ErrNotFound) {
			status = 404
			message = "Saved widget not found."
		}
		s.widgetConfigurationError(w, r, message, status)
		return
	}
	s.renderPluginWidgetEditor(w, r, p, false, "", 200)
}
func (s *server) pluginWidgetNew(w http.ResponseWriter, r *http.Request) {
	p := plugins.PlacementRecord{PluginID: r.PathValue("pluginID"), ConnectionID: r.PathValue("connectionID"), ContributionID: r.PathValue("contributionID"), PlacementID: uuid.NewV4().String(), Options: json.RawMessage(`{}`)}
	s.renderPluginWidgetEditor(w, r, p, true, "", 200)
}
func (s *server) renderPluginWidgetEditor(w http.ResponseWriter, r *http.Request, p plugins.PlacementRecord, create bool, message string, status int) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	c, err := s.plugins.WidgetCatalog(ctx)
	if err != nil {
		s.widgetConfigurationError(w, r, "Widget configuration is unavailable. Try again.", 503)
		return
	}
	d, ok := widgetDefinition(c, p.PluginID, p.ContributionID)
	if create && (!ok || s.plugins.Connection(p.PluginID, p.ConnectionID) == nil) {
		s.widgetConfigurationError(w, r, "Widget or connection not found.", 404)
		return
	}
	title := d.Title
	if !ok {
		title = "Unavailable widget " + p.ContributionID
	}
	fields, fieldErr := manifest.FormFields(d.OptionsSchema)
	values := view.PluginValues(p.Options)
	values["enabled"] = p.Enabled
	action := "/plugin-widgets/" + p.PlacementID + "/options"
	reload := widgetEditURL(p.PlacementID)
	if create {
		action = widgetManagementURL(p.PluginID, p.ConnectionID) + "/" + p.ContributionID
		reload = widgetNewURL(p.PluginID, p.ConnectionID, p.ContributionID)
		title = "Add " + title
	}
	plugin, connection := s.widgetNames(p.PluginID, p.ConnectionID)
	page := view.PluginManagement{Name: title, PluginName: plugin, ConnectionName: connection, Error: message, ReloadURL: reload, OptionsAvailable: ok && fieldErr == nil, Form: view.PluginForm{ID: p.PlacementID, Revision: p.Revision, Title: title, Action: action, Fields: fields, Values: values, Enabled: p.Enabled, Create: create}}
	s.render(w, r, view.PageData{Title: title, Description: "Configure an individual plugin widget.", Path: r.URL.Path, Active: widgetNavigationID, PluginContent: view.PluginWidgets(page)}, status)
}
func (s *server) pluginPlacementWrite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if r.ParseForm() != nil {
		http.Error(w, "Invalid widget form", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	id := r.PathValue("placementID")
	create := id == ""
	var p plugins.PlacementRecord
	var err error
	if create {
		p = plugins.PlacementRecord{PluginID: r.PathValue("pluginID"), ConnectionID: r.PathValue("connectionID"), ContributionID: r.PathValue("contributionID"), PlacementID: r.PostForm.Get("placementId"), Options: json.RawMessage(`{}`)}
	} else {
		p, err = s.plugins.CurrentPlacement(ctx, id)
		if err != nil {
			status := 503
			message := "Widget configuration is unavailable. Try again."
			if errors.Is(err, plugins.ErrNotFound) {
				status = 404
				message = "Saved widget not found."
			}
			s.widgetConfigurationError(w, r, message, status)
			return
		}
		p.Revision = r.PostForm.Get("revision")
		if p.Revision == "" {
			http.Error(w, "Widget revision required", 428)
			return
		}
	}
	action := r.PathValue("action")
	if !create && action != "options" && action != "selection" && action != "delete" {
		http.NotFound(w, r)
		return
	}
	if action == "delete" {
		if r.PostForm.Get("confirm") != "true" {
			s.renderPluginWidgetEditor(w, r, p, create, "Confirm deletion of this saved widget.", 400)
			return
		}
		err = s.plugins.DeletePlacement(ctx, p.PlacementID, p.Revision)
	} else {
		enabled := r.PostForm["enabled"]
		if len(enabled) != 1 || (enabled[0] != "true" && enabled[0] != "false") {
			http.Error(w, "Explicit widget state required", 400)
			return
		}
		p.Enabled = enabled[0] == "true"
		if action != "selection" {
			catalog, e := s.plugins.WidgetCatalog(ctx)
			err = e
			for _, d := range catalog.Definitions {
				if d.PluginID == p.PluginID && d.ContributionID == p.ContributionID {
					p.Options, err = pluginFormOptions(d.OptionsSchema, r)
					break
				}
			}
		}
		if err == nil {
			_, err = s.plugins.SavePlacement(ctx, p, create)
		}
	}
	if err != nil {
		s.renderPluginWidgetEditor(w, r, p, create, "Check values or reload the current revision. Your submitted options are shown below.", 409)
		return
	}
	destination := widgetEditURL(p.PlacementID)
	if action == "delete" {
		destination = widgetIndexURL
	}
	http.Redirect(w, r, destination, 303)
}
func pluginFormOptions(schema []byte, r *http.Request) (raw json.RawMessage, err error) {
	fields, err := manifest.FormFields(schema)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	draft := map[string]any{}
	for _, f := range fields {
		if f.Type == "array" {
			xs := r.PostForm["option."+f.Name]
			if xs == nil {
				xs = []string{}
			}
			draft[f.Name] = xs
		} else {
			draft[f.Name] = r.PostForm.Get("option." + f.Name)
		}
	}
	defer func() {
		if err != nil {
			raw, _ = json.Marshal(draft)
		}
	}()
	for _, f := range fields {
		xs := r.PostForm["option."+f.Name]
		if f.Type == "array" {
			if xs == nil {
				xs = []string{}
			}
			values[f.Name] = xs
			continue
		}
		if len(xs) > 1 {
			return nil, plugins.ErrUnavailable
		}
		v := ""
		if len(xs) == 1 {
			v = xs[0]
		}
		if f.Type == "boolean" {
			if v != "" && v != "true" {
				return nil, plugins.ErrUnavailable
			}
			values[f.Name] = v == "true"
			continue
		}
		if v == "" && !f.Required {
			continue
		}
		switch f.Type {
		case "integer":
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil {
				return nil, e
			}
			values[f.Name] = n
		case "number":
			decoder := json.NewDecoder(bytes.NewBufferString(v))
			decoder.UseNumber()
			var value any
			if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
				return nil, plugins.ErrUnavailable
			}
			n, ok := value.(json.Number)
			if !ok {
				return nil, plugins.ErrUnavailable
			}
			values[f.Name] = n
		default:
			values[f.Name] = v
		}
	}
	for key := range r.PostForm {
		if strings.HasPrefix(key, "option.") {
			known := false
			for _, f := range fields {
				if key == "option."+f.Name {
					known = true
				}
			}
			if !known {
				return nil, plugins.ErrUnavailable
			}
		}
	}
	return json.Marshal(values)
}
