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
	var runtimeScripts []string
	if (data.HostDetail != nil || data.Dashboard != nil) && data.Error == "" {
		runtimeScripts = []string{assets.HTMXExtSSEURL, "/ui/host-live.js"}
	}
	if data.Dashboard != nil && data.Error == "" {
		runtimeScripts = append(runtimeScripts, "/ui/dashboard-layout.js")
	}
	return consoleshell.Layout(
		consoleshell.Config{
			Brand: consoleshell.Brand{
				Name:    "Balemoh",
				HomeURL: "/",
			},
			Navigation: consoleshell.Navigation{
				Drawer:        true,
				IconOnlyMenu:  true,
				DisableSearch: true,
				Items: []sidebar.Item{
					{ID: "nav-home", Label: "Home", Href: "/", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidHome, Decorative: true})},
					{ID: "nav-staging", Label: "Staging", Href: "/staging", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidQueueList, Decorative: true})},
					{ID: "nav-hosts", Label: "Hosts", Href: "/hosts", Icon: appicons.Icon(appicons.Config{Symbol: appicons.IconHeroiconsServer, Decorative: true})},
					{ID: "nav-icons", Label: "Icons", Href: "/icons", Icon: icon.Icon(icon.Config{SpriteURL: heroicons.SpriteURL, Symbol: heroicons.Icon16SolidPaintBrush, Decorative: true})},
				},
			},
			Appearance: consoleshell.AppearanceConfig{
				DefaultTheme:       "goshtoso",
				InitialColorScheme: consoleshell.ColorSchemeSystem,
				PersistPreferences: true,
				ThemeStylesheets:   []string{"/ui/balemoh.css"},
			},
			Interactions: consoleshell.InteractionConfig{EnableHTMX: false, LocalRuntime: true, RuntimeScripts: runtimeScripts},
			MainID:       "main-content",
			ContentID:    "balemoh-content",
			Footer:       AttributionLinks(),
		},
		consoleshell.Page{
			Title:         data.Title,
			DocumentTitle: data.Title + " · Balemoh",
			Description:   data.Description,
			CanonicalURL:  canonicalURL(data),
			Active:        data.Active,
			Content:       PageContent(data),
			Metadata:      &head.MetadataConfig{Image: head.SocialImage{URL: "https://balemoh.decastro.me/ui/social-v1.png", MIMEType: "image/png", Width: 1280, Height: 640, Alt: "Balemoh, discover services and pin them to your homelab homepage."}},
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
