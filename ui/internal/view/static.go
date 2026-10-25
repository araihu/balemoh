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
