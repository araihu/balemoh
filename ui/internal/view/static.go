package view

import (
	"embed"
	"io/fs"
)

//go:embed static/balemoh.css
var balemohCSS []byte

func BalemohCSS() []byte { return balemohCSS }

//go:embed static/social-v2.png
var socialPreview []byte

func SocialPreview() []byte { return socialPreview }

//go:embed static/social-v1.png
var socialPreviewV1 []byte

// SocialPreviewV1 preserves the immutable URL used by previously shared cards.
func SocialPreviewV1() []byte { return socialPreviewV1 }

//go:embed static/host-live.js
var hostLiveJS []byte

func HostLiveJS() []byte { return hostLiveJS }

//go:embed static/dashboard-layout.js
var dashboardLayoutJS []byte

func DashboardLayoutJS() []byte { return dashboardLayoutJS }

//go:embed static/brand/*
var brandFiles embed.FS

// BrandAssets contains the verified, release-pinned Arai Hu brand assets.
func BrandAssets() fs.FS {
	files, err := fs.Sub(brandFiles, "static/brand")
	if err != nil {
		panic(err)
	}
	return files
}

//go:embed static/plugin-runtime.js
var pluginRuntimeJS []byte

func PluginRuntimeJS() []byte { return pluginRuntimeJS }

//go:embed static/notifications.js
var notificationsJS []byte

func NotificationsJS() []byte { return notificationsJS }

//go:embed static/browser-notifications.js
var browserNotificationsJS []byte

func BrowserNotificationsJS() []byte { return browserNotificationsJS }

//go:embed static/notifications-worker.js
var notificationsWorkerJS []byte

func NotificationsWorkerJS() []byte { return notificationsWorkerJS }

//go:embed static/presentation.js
var presentationJS []byte

func PresentationJS() []byte { return presentationJS }

//go:embed static/theme-selection.js
var themeSelectionJS []byte

func ThemeSelectionJS() []byte { return themeSelectionJS }

//go:embed static/appearance.js
var appearanceJS []byte

func AppearanceJS() []byte { return appearanceJS }

//go:embed static/theme-preview.js
var themePreviewJS []byte

func ThemePreviewJS() []byte { return themePreviewJS }

//go:embed static/presentation-fonts.js
var presentationFontsJS []byte

func PresentationFontsJS() []byte { return presentationFontsJS }
