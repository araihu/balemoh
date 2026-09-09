package view

import (
	"github.com/a-h/templ"
	"github.com/araihu/goshtoso-app-shells/consoleshell"
	"github.com/araihu/goshtoso/components/head"
	"github.com/araihu/goshtoso/components/sidebar"
)

func ConsolePage(data PageData) templ.Component {
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
				SectionsTitle: "Balemoh",
				Sections: []sidebar.Section{{Items: []sidebar.Item{
					{ID: "nav-home", Label: "Home", Href: "/"},
					{ID: "nav-staging", Label: "Staging", Href: "/staging"},
					{ID: "nav-icons", Label: "Icons", Href: "/icons"},
				}}},
			},
			Appearance: consoleshell.AppearanceConfig{
				DefaultTheme:       "goshtoso",
				InitialColorScheme: consoleshell.ColorSchemeSystem,
				PersistPreferences: true,
				ThemeStylesheets:   []string{"/ui/balemoh.css"},
			},
			Interactions: consoleshell.InteractionConfig{EnableHTMX: false, LocalRuntime: true},
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
