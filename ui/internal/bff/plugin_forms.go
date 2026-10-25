package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/plugins"
	"github.com/araihu/balemoh/ui/internal/view"
	"io"
	"net/http"
	"strconv"
	"strings"
	"uuid"
)

func widgetManagementURL(plugin, connection string) string {
	return "/plugins/" + plugin + "/connections/" + connection + "/widgets"
}
func (s *server) pluginWidgets(w http.ResponseWriter, r *http.Request) {
	s.renderPluginWidgets(w, r, r.PathValue("pluginID"), r.PathValue("connectionID"), nil, "", 200)
}
func (s *server) renderPluginWidgets(w http.ResponseWriter, r *http.Request, plugin, connection string, failed *plugins.PlacementRecord, message string, status int) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	c, err := s.plugins.WidgetCatalog(ctx)
	if err != nil {
		s.render(w, r, view.PageData{Title: "Plugin widgets", Error: "Widget configuration is unavailable."}, 503)
		return
	}
	page := view.PluginManagement{PluginID: plugin, ConnectionID: connection, Name: plugin, URL: widgetManagementURL(plugin, connection), Error: message}
	if e := s.plugins.Connection(plugin, connection); e != nil {
		page.Name = e.ConnectionName
	}
	definitions := map[string]plugins.Definition{}
	for _, d := range c.Definitions {
		if d.PluginID == plugin {
			definitions[d.ContributionID] = d
		}
	}
	makeForm := func(p plugins.PlacementRecord, create bool) view.PluginForm {
		d := definitions[p.ContributionID]
		fields, _ := manifest.FormFields(d.OptionsSchema)
		title := d.Title
		if title == "" {
			title = "Unavailable widget " + p.ContributionID
		}
		values := view.PluginValues(p.Options)
		values["enabled"] = p.Enabled
		action := "/plugin-widgets/" + p.PlacementID + "/options"
		if create {
			action = page.URL + "/" + p.ContributionID
		}
		return view.PluginForm{ID: p.PlacementID, Revision: p.Revision, Title: title, Action: action, Fields: fields, Values: values, Enabled: p.Enabled, Create: create}
	}
	found := false
	for _, p := range c.Placements {
		if p.PluginID == plugin && p.ConnectionID == connection {
			if failed != nil && failed.PlacementID == p.PlacementID {
				p = *failed
				found = true
			}
			page.Forms = append(page.Forms, makeForm(p, false))
		}
	}
	for _, d := range c.Definitions {
		if d.PluginID != plugin {
			continue
		}
		p := plugins.PlacementRecord{PluginID: plugin, ConnectionID: connection, ContributionID: d.ContributionID, PlacementID: uuid.NewV4().String(), Options: json.RawMessage(`{}`)}
		if failed != nil && !found && failed.ContributionID == d.ContributionID {
			p = *failed
		}
		page.Forms = append(page.Forms, makeForm(p, true))
	}
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, r, view.PageData{Title: "Plugin widgets", Description: "Configure independent widgets and choose what appears on home.", Path: r.URL.Path, PluginContent: view.PluginWidgets(page)}, status)
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
			http.NotFound(w, r)
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
			s.renderPluginWidgets(w, r, p.PluginID, p.ConnectionID, &p, "Confirm deletion of this saved widget.", 400)
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
		s.renderPluginWidgets(w, r, p.PluginID, p.ConnectionID, &p, "Unable to save widget. Check values or reload the current revision.", 409)
		return
	}
	http.Redirect(w, r, widgetManagementURL(p.PluginID, p.ConnectionID), 303)
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
