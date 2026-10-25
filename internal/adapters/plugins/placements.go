package plugins

import (
	"encoding/json"
	app "github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
	"os"
	"sort"
)

func (r *Runtime) Definitions() []app.Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []app.Definition{}
	for _, p := range r.plugins {
		for _, w := range p.pkg.Manifest.Widgets {
			if !runtime.Available(p.pkg.Manifest, "widget", w.ID) {
				continue
			}
			root, e := os.OpenRoot(p.pkg.Root)
			if e != nil {
				continue
			}
			raw, e := root.ReadFile(w.OptionsSchema)
			root.Close()
			if e == nil {
				out = append(out, app.Definition{PluginID: p.pkg.Manifest.ID, ContributionID: w.ID, Title: w.Title, OptionsSchema: raw})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PluginID != out[j].PluginID {
			return out[i].PluginID < out[j].PluginID
		}
		return out[i].ContributionID < out[j].ContributionID
	})
	return out
}
func (r *Runtime) ValidateOptions(plugin, contribution string, options json.RawMessage) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.plugins[plugin]
	if p == nil {
		return app.Fail("not_found")
	}
	w, ok := p.pkg.Manifest.Widget(contribution)
	if !ok || !runtime.Available(p.pkg.Manifest, "widget", contribution) {
		return app.Fail("not_found")
	}
	if len(options) > 64<<10 || p.pkg.ValidateJSON(w.OptionsSchema, options) != nil {
		return app.Fail("invalid_argument")
	}
	return nil
}
