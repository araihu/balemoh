package bff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	api "github.com/araihu/balemoh/client"
	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/balemoh/ui/internal/plugins"
	"github.com/araihu/balemoh/ui/internal/view"
	"github.com/araihu/goshtoso/components/icon"
	"github.com/araihu/goshtoso/components/icon/heroicons"
	"github.com/araihu/goshtoso/components/sidebar"
)

type pluginAppearanceKey struct{}

// Resolve once for plugin metadata and its standalone native document.
func (s *server) withPluginAppearance(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.presentation == nil {
			next(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		defer cancel()
		appearance := s.appearance(ctx, false)
		ctx = pluginui.WithDisplayContext(ctx, pluginui.DisplayContext{Theme: appearance.Theme, ColorScheme: appearance.Mode})
		ctx = context.WithValue(ctx, pluginAppearanceKey{}, appearance)
		next(w, r.WithContext(ctx))
	}
}

func validPluginRequest(r *http.Request) bool {
	return len(r.URL.RawQuery) <= 64<<10 && !strings.Contains(strings.ToLower(r.URL.EscapedPath()), "%2f")
}
func (s *server) pluginPage(w http.ResponseWriter, r *http.Request) {
	if !validPluginRequest(r) {
		http.Error(w, "Invalid plugin request", 400)
		return
	}
	page, _, active, err := s.plugins.ResolvePage(r.PathValue("pluginID"), r.PathValue("connectionID"), "/"+r.PathValue("pluginPath"), r.URL.Query())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentURL := strings.Replace(r.URL.EscapedPath(), "/pages/", "/page-content/", 1)
	if r.URL.RawQuery != "" {
		contentURL += "?" + r.URL.RawQuery
	}
	sum := sha256.Sum256([]byte(contentURL))
	description := page.Description
	if description == "" {
		description = "Content from an explicitly installed Balemoh application plugin."
	}
	s.render(w, r, view.PageData{Title: page.Title, Description: description, Active: active, Path: r.URL.Path, PluginContent: view.PluginSlot("plugin-page-"+hex.EncodeToString(sum[:]), page.Title, contentURL)}, 200)
}
func (s *server) pluginPageContent(w http.ResponseWriter, r *http.Request) {
	if !validPluginRequest(r) {
		http.Error(w, "Invalid plugin request", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	response, err := s.plugins.Page(ctx, r.PathValue("pluginID"), r.PathValue("connectionID"), "/"+r.PathValue("pluginPath"), r.URL.Query())
	s.pluginContent(w, r, response, err)
}
func (s *server) pluginWidgetContent(w http.ResponseWriter, r *http.Request) {
	if s.plugins.Persistent() {
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		defer cancel()
		response, err := s.plugins.PersistentWidget(ctx, r.PathValue("placementID"))
		s.pluginContent(w, r, response, err)
		return
	}
	ref, ok := s.plugins.Placement(r.PathValue("placementID"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	response, err := s.plugins.Widget(ctx, ref)
	s.pluginContent(w, r, response, err)
}

// pluginContent is the private trusted HTML boundary, never a data API field.
func (s *server) pluginContent(w http.ResponseWriter, r *http.Request, response pluginui.Response, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if errors.Is(err, plugins.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = view.PluginSlotFailure(r.URL.RequestURI()).Render(r.Context(), w)
		return
	}
	if response.Kind == "no-content" {
		w.WriteHeader(204)
		return
	}
	if response.Kind == "redirect" {
		e := s.plugins.Connection(r.PathValue("pluginID"), r.PathValue("connectionID"))
		if e == nil {
			http.NotFound(w, r)
			return
		}
		request := pluginui.Request{ConnectionID: e.Installation.ConnectionID}
		href, e2 := request.PageURL(e.Package.Manifest, response.Redirect.Page, response.Redirect.Params)
		if e2 != nil {
			http.Error(w, "Invalid plugin redirect", 502)
			return
		}
		if len(response.Redirect.Query) > 0 {
			href += "?" + response.Redirect.Query.Encode()
		}
		http.Redirect(w, r, href, 303)
		return
	}
	plugin, connection := r.PathValue("pluginID"), r.PathValue("connectionID")
	if r.PathValue("placementID") != "" {
		if p, e := s.plugins.CurrentPlacement(r.Context(), r.PathValue("placementID")); e == nil {
			plugin, connection = p.PluginID, p.ConnectionID
		} else if ref, ok := s.plugins.Placement(r.PathValue("placementID")); ok {
			plugin, connection = ref.PluginID, ref.ConnectionID
		}
	}
	assets, assetErr := s.plugins.ResponseAssets(plugin, connection, response.AssetIDs)
	if assetErr != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = view.PluginUnavailable().Render(r.Context(), w)
		return
	}
	w.WriteHeader(response.Status)
	if r.URL.Query().Get("__native") == "1" {
		styles := []string{}
		for _, asset := range assets {
			if asset.Kind == "style" {
				styles = append(styles, asset.URL)
			}
		}
		appearance, ok := r.Context().Value(pluginAppearanceKey{}).(view.Appearance)
		if !ok {
			display := pluginui.Display(r.Context())
			appearance = view.Appearance{Theme: display.Theme, Mode: display.ColorScheme}
		}
		_ = view.PluginDocument(response.HTML, appearance, styles).Render(r.Context(), w)
		return
	}
	if response.Kind == "fragment" {
		_, _ = w.Write(response.HTML)
		return
	}
	raw, _ := json.Marshal(assets)
	_ = view.PluginMount(r.URL.Path, string(raw), response.HTML).Render(r.Context(), w)
	if response.Status == 503 {
		_ = view.PluginRetry(r.URL.RequestURI()).Render(r.Context(), w)
	}
}

func (s *server) pluginRenderer(item api.DashboardWidget) (view.Widget, error) {
	var ref plugins.Reference
	if err := decodeWidget(item, &ref); err != nil {
		return view.Widget{}, err
	}
	if err := ref.Validate(); err != nil {
		return view.Widget{}, err
	}
	expected, ok := s.plugins.Placement(ref.PlacementID)
	if !s.plugins.Persistent() && (!ok || expected != ref) {
		return view.Widget{}, plugins.ErrNotFound
	}
	slotID := "plugin-slot-" + ref.PlacementID

	revision, _ := json.Marshal(item.Data)
	return view.Widget{ID: item.Id, Content: view.PluginWidgetSlot(slotID, item.Title, "/plugin-widgets/"+ref.PlacementID, string(revision))}, nil
}
func (s *server) pluginNavigation() []sidebar.Item {
	var convert func([]plugins.Navigation) []sidebar.Item
	convert = func(ns []plugins.Navigation) []sidebar.Item {
		out := []sidebar.Item{}
		for _, n := range ns {
			symbol := pluginIconSymbol(n.Icon, heroicons.Icon16SolidSquares2x2)
			out = append(out, sidebar.Item{ID: n.ID, Label: n.Title, Href: n.Href, Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: symbol, Decorative: true}), Items: convert(n.Children), Collapsible: len(n.Children) > 0})
		}
		return out
	}
	return convert(s.plugins.Navigation())
}
func (s *server) dashboardSnapshot(ctx context.Context) (api.DashboardSnapshot, error) {
	catalog, ok := s.catalog.(DashboardCatalog)
	if !ok {
		return api.DashboardSnapshot{}, errors.New("dashboard unavailable")
	}
	snapshot, err := catalog.Dashboard(ctx)
	if err != nil {
		return snapshot, err
	}
	// Explicit preview fixtures contribute references, never source data or pins.
	if s.plugins != nil && !s.plugins.Persistent() {
		snapshot.Items = append(snapshot.Items, s.plugins.Widgets()...)
	}
	return snapshot, nil
}

func (s *server) pluginFragment(w http.ResponseWriter, r *http.Request) {
	if !validPluginRequest(r) {
		http.Error(w, "Invalid plugin request", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	response, err := s.plugins.Fragment(ctx, r.PathValue("pluginID"), r.PathValue("connectionID"), r.PathValue("contributionID"), r.PathValue("region"), r.URL.Query())
	s.pluginContent(w, r, response, err)
}

func (s *server) pluginAsset(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/plugins/assets/"), "/", 3)
	if !strings.HasPrefix(r.URL.Path, "/plugins/assets/") || len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	data, media, err := s.plugins.ReadAsset(parts[0], parts[1], parts[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
	_, _ = w.Write(data)
}

func (s *server) pluginPagePost(w http.ResponseWriter, r *http.Request) {
	if !validPluginRequest(r) {
		http.Error(w, "Invalid plugin request", 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Plugin input exceeds limit", 413)
		return
	}
	form := url.Values{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		form, err = url.ParseQuery(string(body))
		if err != nil {
			http.Error(w, "Invalid plugin input", 400)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	response, err := s.plugins.PageRequest(ctx, r.PathValue("pluginID"), r.PathValue("connectionID"), "/"+r.PathValue("pluginPath"), "POST", r.URL.Query(), form, body)
	s.pluginContent(w, r, response, err)
}

// Resolve only bundled symbols. Producer strings never become URLs or markup.
func pluginIconSymbol(value string, fallback icon.Symbol) icon.Symbol {
	for _, glyph := range heroicons.Glyphs {
		if string(glyph.Symbol) == value {
			return glyph.Symbol
		}
	}
	return fallback
}
