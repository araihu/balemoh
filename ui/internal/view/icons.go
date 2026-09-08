package view

import (
	"fmt"
	"github.com/araihu/goshtoso/iconlibrary"
	"net/url"
	"strings"
)

type LibraryIcon struct {
	ID, Name, Source, Tags, URL, LightURL, DarkURL, Symbol, SpriteURL, License, Digest string
	UsedBy                                                                             []string
}
type IconPage struct {
	Icons                        []LibraryIcon
	Query, Source, Error, Notice string
	Page, Total                  int
	Picker                       bool
	Editing                      *LibraryIcon
}

func LibraryEntry(i iconlibrary.Icon) LibraryIcon {
	v := LibraryIcon{ID: i.ID, Name: i.Name, Source: i.Source, Tags: strings.Join(i.Tags, ", "), License: i.License}
	for _, a := range i.Variants {
		switch a.Appearance {
		case "default":
			v.URL = a.Path
		case "light":
			v.LightURL = a.Path
		case "dark":
			v.DarkURL = a.Path
		}
		if a.MIME == "image/svg-symbol" {
			v.SpriteURL, v.Symbol, _ = strings.Cut(a.Path, "#")
			v.URL = ""
		}
	}
	return v
}
func IconImageURL(id string) string { return "/icons/" + url.PathEscape(id) + "/image" }
func IconEditURL(id string) string  { return "/icons/" + url.PathEscape(id) + "/edit" }
func IconPageURL(data IconPage, page int) string {
	path := "/icons"
	if data.Picker {
		path += "/picker"
	}
	return fmt.Sprintf("%s?q=%s&source=%s&page=%d", path, url.QueryEscape(data.Query), url.QueryEscape(data.Source), page)
}

func iconUsageLabel(count int) string {
	if count == 1 {
		return "Used by 1 service"
	}
	return fmt.Sprintf("Used by %d services", count)
}
