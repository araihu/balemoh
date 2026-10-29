package ui

import (
	"embed"
	"io/fs"
)

//go:embed internal/appicons/sprite.svg
var IconSprite []byte

//go:embed internal/appicons/LICENSES
var iconLicenses embed.FS

// IconLicenses contains the retained notices for Balemoh's application icons.
func IconLicenses() fs.FS {
	files, err := fs.Sub(iconLicenses, "internal/appicons/LICENSES")
	if err != nil {
		panic(err)
	}
	return files
}
