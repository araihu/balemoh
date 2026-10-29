package view

import (
	"github.com/a-h/templ"
	"github.com/araihu/balemoh/ui/internal/appicons"
	"github.com/araihu/goshtoso-app-shells/consoleshell"
	"github.com/araihu/goshtoso/assets"
	"github.com/araihu/goshtoso/components/head"
	"github.com/araihu/goshtoso/components/icon"
	"github.com/araihu/goshtoso/components/icon/heroicons"
	"github.com/araihu/goshtoso/components/sidebar"
)

func ConsolePage(data PageData) templ.Component {
	if data.Appearance.Theme == "" {
		data.Appearance.Theme = data.Theme
		if data.Appearance.Theme == "" {
			data.Appearance.Theme = "goshtoso"
		}
	}
	if data.Appearance.Mode == "" {
		data.Appearance.Mode = data.ColorScheme
		if data.Appearance.Mode == "" {
			data.Appearance.Mode = "system"
		}
	}
	runtimeScripts := []string{"/ui/plugin-runtime.js", "/ui/notifications.js"}
	if data.IconPage != nil {
		runtimeScripts = append(runtimeScripts, "/ui/theme-selection.js", "/ui/presentation-fonts.js")
	}
	if data.ThemeEditor != nil {
		runtimeScripts = append(runtimeScripts, "/ui/presentation.js")
	}
	if (data.HostDetail != nil || data.Dashboard != nil) && data.Error == "" {
		runtimeScripts = append(runtimeScripts, assets.HTMXExtSSEURL, "/ui/host-live.js")
	}
	if data.Dashboard != nil && data.Error == "" {
		runtimeScripts = append(runtimeScripts, "/ui/dashboard-layout.js")
	}
	navigation := []sidebar.Item{
		{ID: "nav-home", Label: "Home", Href: "/", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidHome, Decorative: true})},
		{ID: "nav-staging", Label: "Staging", Href: "/staging", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidQueueList, Decorative: true})},
		{ID: "nav-hosts", Label: "Hosts", Href: "/hosts", Icon: appicons.Icon(appicons.Config{Symbol: appicons.IconHeroiconsServer, Decorative: true})},
	}
	config := sidebar.Item{
		ID:    "nav-config",
		Label: "Config",
		Href:  "/presentation",
		Icon:  icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidCog6Tooth, Decorative: true}),
		Items: []sidebar.Item{
			{ID: "nav-presentation", Label: "Appearance", Href: "/presentation", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidPaintBrush, Decorative: true})},
		},
	}
	for _, item := range data.PluginNavigation {
		if item.ID == "nav-plugin-widgets" {
			config.Items = append(config.Items, item)
		} else {
			navigation = append(navigation, item)
		}
	}
	navigation = append(navigation, config)
	return consoleshell.Layout(
		consoleshell.Config{
			Brand: consoleshell.Brand{
				Name:       "Balemoh",
				HomeURL:    "/",
				Logo:       SiteLogo(data.Appearance.BrandLogoID),
				HideName:   true,
				FaviconURL: data.Appearance.iconURL(),
			},
			Navigation: consoleshell.Navigation{
				Drawer:        true,
				IconOnlyMenu:  true,
				DisableSearch: true,
				Items:         navigation,
			},
			Appearance: consoleshell.AppearanceConfig{
				DefaultTheme:       data.Appearance.Theme,
				InitialColorScheme: consoleshell.ColorScheme(data.Appearance.Mode),
				PersistPreferences: false,
				ThemeStylesheets:   []string{"/ui/balemoh.css"},
			},
			HeaderActions: HeaderActions(false),
			Interactions:  consoleshell.InteractionConfig{EnableHTMX: false, LocalRuntime: true, RuntimeScripts: runtimeScripts},
			MainID:        "main-content",
			ContentID:     "balemoh-content",
			Footer:        ProductFooter(),
		},
		consoleshell.Page{
			Title:         data.Title,
			DocumentTitle: data.Title + " · Balemoh",
			Description:   data.Description,
			CanonicalURL:  canonicalURL(data),
			Active:        data.Active,
			Content:       PageContent(data),
			Head:          AppearanceHead(data.Appearance),
			Metadata:      &head.MetadataConfig{Image: head.SocialImage{URL: "https://balemoh.decastro.me/ui/social-v2.png", MIMEType: "image/png", Width: 1280, Height: 640, Alt: "Balemoh, discover services and pin them to your homelab homepage."}},
		},
	)
}

func pageURL(staging bool) string {
	if staging {
		return "https://balemoh.decastro.me/staging"
	}
	return "https://balemoh.decastro.me/"
}

func canonicalURL(data PageData) string {
	if data.Path != "" {
		return "https://balemoh.decastro.me" + data.Path
	}
	return pageURL(data.Staging)
}
