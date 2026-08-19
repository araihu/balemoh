package view

import (
	"github.com/a-h/templ"
	"github.com/araihu/goshtoso-app-shells/consoleshell"
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
				DisableSearch: true,
				SectionsTitle: "Balemoh",
				Sections: []sidebar.Section{{Items: []sidebar.Item{
					{ID: "nav-home", Label: "Homepage", Href: "/"},
					{ID: "nav-staging", Label: "Staging", Href: "/staging"},
				}}},
			},
			Appearance: consoleshell.AppearanceConfig{
				DefaultTheme:       "goshtoso",
				InitialColorScheme: consoleshell.ColorSchemeSystem,
				PersistPreferences: true,
				ThemeStylesheets:   []string{"/ui/balemoh.css"},
			},
			Interactions: consoleshell.InteractionConfig{EnableHTMX: false},
			MainID:       "main-content",
			ContentID:    "balemoh-content",
		},
		consoleshell.Page{
			Title:         data.Title,
			DocumentTitle: data.Title + " · Balemoh",
			Description:   data.Description,
			CanonicalURL:  pageURL(data.Staging),
			Active:        data.Active,
			Content:       PageContent(data),
		},
	)
}

func pageURL(staging bool) string {
	if staging {
		return "/staging"
	}
	return "/"
}
