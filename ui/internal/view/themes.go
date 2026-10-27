package view

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/araihu/goshtoso/components/icon/heroicons"

	selectfield "github.com/araihu/goshtoso/components/select"
	"github.com/araihu/goshtoso/components/themeeditor"
	"github.com/araihu/goshtoso/themes"
)

type Appearance struct {
	BrandLogoID string
	BrandIconID string
	Theme       string
	Mode        string
	CSS         string
	Warning     string
	Recovery    bool
	FontWatch   []themes.FontOption
}
type ThemeItem struct {
	BrandLogoID string            `json:"brandLogoId"`
	BrandIconID string            `json:"brandIconId"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Revision    string            `json:"revision"`
	Definition  themes.Definition `json:"definition"`
}
type FontItem struct {
	ID       string
	Name     string
	Weight   string
	Style    string
	Variable bool
	Revision string
	UsedBy   []string
}
type ThemesPage struct {
	Creating         bool
	Available        bool
	Error            string
	SettingsRevision string
	CatalogRevision  string
	SelectedID       string
	ColorMode        string
	Initial          themes.Definition
	Items            []ThemeItem
	Fonts            []themes.FontOption
	Uploads          []FontItem
	Recovery         bool
}

func (d ThemesPage) editor() themeeditor.Config {
	return themeeditor.Config{ID: "presentation-theme-editor", Initial: d.Initial, Fonts: d.Fonts, PreviewID: "presentation-theme-preview", InitialPreviewMode: "light", InputName: "definition"}
}
func (d ThemesPage) options() []selectfield.Option {
	config := d.editor()
	for _, item := range d.Items {
		config.Catalog = append(config.Catalog, themeeditor.CatalogItem{ID: item.ID, Label: item.Name, Definition: item.Definition})
	}
	out := config.PresetOptions()
	for i := range out {
		out[i].Selected = out[i].Value == d.SelectedID
		out[i].Trailing = ThemeKindBadge(strings.HasPrefix(out[i].Value, "builtin:"))
	}
	return out
}
func (d ThemesPage) modeOptions() []selectfield.Option {
	return []selectfield.Option{{Value: "system", Label: "System", Selected: d.ColorMode == "system"}, {Value: "light", Label: "Light", Selected: d.ColorMode == "light"}, {Value: "dark", Label: "Dark", Selected: d.ColorMode == "dark"}}
}
func (d ThemesPage) bootstrap() string {
	b, _ := json.Marshal(struct {
		SettingsRevision string              `json:"settingsRevision"`
		CatalogRevision  string              `json:"catalogRevision"`
		SelectedID       string              `json:"selectedID"`
		ColorMode        string              `json:"colorMode"`
		Items            []ThemeItem         `json:"items"`
		Fonts            []themes.FontOption `json:"fonts"`
		Recovery         bool                `json:"recovery"`
		Creating         bool                `json:"creating"`
	}{d.SettingsRevision, d.CatalogRevision, d.SelectedID, d.ColorMode, d.Items, d.Fonts, d.Recovery, d.Creating})
	return string(b)
}
func (d ThemesPage) editorURL(create bool) string {
	values := url.Values{"themeID": {d.SelectedID}}
	if create {
		values.Set("new", "1")
	}
	if d.Recovery {
		values.Set("appearance", "recovery")
	}
	return "/presentation/themes/editor?" + values.Encode()
}
func (d ThemesPage) themesURL() string {
	if d.Recovery {
		return "/presentation?appearance=recovery#themes"
	}
	return "/presentation#themes"
}
func themeEditorTitle(d ThemesPage) string {
	if d.Creating {
		return "Create theme"
	}
	return "Edit theme"
}
func (d ThemesPage) brandIconID() string {
	for _, item := range d.Items {
		if item.ID == d.SelectedID {
			return item.BrandIconID
		}
	}
	return ""
}
func (d ThemesPage) brandLogoID() string {
	for _, item := range d.Items {
		if item.ID == d.SelectedID {
			return item.BrandLogoID
		}
	}
	return ""
}
func (a Appearance) iconURL() string {
	if a.BrandIconID != "" {
		return IconImageURL(a.BrandIconID)
	}
	return "/ui/brand/favicon.svg"
}
func brandManifestURL(id string) string {
	return "/presentation/brand/manifest?iconID=" + url.QueryEscape(id)
}
func fontFileURL(id string) string   { return "/presentation/fonts/" + url.PathEscape(id) + "/file" }
func fontDeleteURL(id string) string { return "/presentation/fonts/" + url.PathEscape(id) + "/delete" }

func themePreviewDashboard() Dashboard {
	observed := time.Date(2026, time.October, 27, 12, 30, 0, 0, time.UTC)
	cpu, memory := 24.6, 42.8
	hosts := []Host{
		{ID: "preview-nas", Name: "nas-01", Kind: "Docker host", Status: "fresh", Architecture: "amd64", CPU: &cpu, Memory: &memory, ObservedAt: &observed},
	}
	services := []Service{
		{ID: "preview-jellyfin", DisplayName: "Jellyfin", Description: "Movies, music and TV shows", Address: "https://jellyfin.example.test", Icon: LibraryIcon{ID: "preview-media", SpriteURL: heroicons.SpriteURL, Symbol: string(heroicons.Icon16SolidPlay)}},
		{ID: "preview-grafana", DisplayName: "Grafana", Description: "Metrics and dashboards", Endpoints: []Endpoint{{URL: "https://grafana.example.test"}, {URL: "http://grafana.local:3000"}}, Icon: LibraryIcon{ID: "preview-metrics", SpriteURL: heroicons.SpriteURL, Symbol: string(heroicons.Icon16SolidChartBar)}},
	}
	page := Dashboard{}
	for _, host := range hosts {
		page.Widgets = append(page.Widgets, Widget{ID: "host-" + host.ID, SourceKind: "host", Content: HostWidget(host, "")})
	}
	for _, service := range services {
		page.Widgets = append(page.Widgets, Widget{ID: "service-" + service.ID, SourceKind: "service", Content: ServiceCard(service, ServiceCardIcon(service))})
	}
	return page
}

func brandPreviewIconURL(id string) string {
	if id != "" {
		return IconImageURL(id)
	}
	return "/ui/brand/favicon.svg"
}

func (d ThemesPage) editorActions() templ.Component {
	if !d.Available || d.Creating {
		return nil
	}
	for _, item := range d.Items {
		if item.ID == d.SelectedID && item.Revision != "" {
			return ThemeEditorActions()
		}
	}
	return nil
}
