// Package iconassets contains the immutable, vendored icon catalog.
package iconassets

import (
	"embed"
	"encoding/json"
	"github.com/araihu/goshtoso/components/icon/heroicons"
	"github.com/araihu/goshtoso/iconlibrary"
	"strings"
)

//go:embed selfhst/catalog.json selfhst/LICENSES
var Files embed.FS

var Entries = load()
var byID = index()

func load() []iconlibrary.Icon {
	b, err := Files.ReadFile("selfhst/catalog.json")
	if err != nil {
		panic(err)
	}
	var catalog iconlibrary.Catalog
	if err := json.Unmarshal(b, &catalog); err != nil {
		panic(err)
	}
	for i := range catalog.Icons {
		for j := range catalog.Icons[i].Variants {
			catalog.Icons[i].Variants[j].Path = "/ui/icon-library/selfhst/" + catalog.Icons[i].Variants[j].Path
		}
	}
	for _, g := range heroicons.Glyphs {
		catalog.Icons = append(catalog.Icons, iconlibrary.Icon{ID: "goshtoso:" + g.CanonicalName, Name: strings.ReplaceAll(strings.TrimPrefix(g.CanonicalName, "ui-hi-16-solid-"), "-", " "), Reference: g.CanonicalName, Source: "goshtoso", License: "MIT", Variants: []iconlibrary.Variant{{Appearance: "default", Path: heroicons.SpriteURL + "#" + string(g.Symbol), MIME: "image/svg-symbol"}}})
	}
	return catalog.Icons
}
func index() map[string]iconlibrary.Icon {
	m := map[string]iconlibrary.Icon{}
	for _, i := range Entries {
		m[i.ID] = i
	}
	return m
}
func Has(id string) bool                        { _, ok := byID[id]; return ok }
func Lookup(id string) (iconlibrary.Icon, bool) { i, ok := byID[id]; return i, ok }
