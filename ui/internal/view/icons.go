package view

import (
	"fmt"
	"github.com/araihu/goshtoso/iconlibrary"
	"net/url"
	"strconv"
	"strings"
)

type LibraryIcon struct {
	ID, Name, Source, Tags, URL, LightURL, DarkURL, Symbol, SpriteURL, License, Digest string
	UsedBy                                                                             []string
}
type IconPage struct {
	Icons                []LibraryIcon
	Query, Error, Notice string
	Sources              []string
	Page, Total          int
	Picker               bool
	Editing              *LibraryIcon
	Details              *LibraryIcon
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
func IconImageURL(id string) string   { return "/icons/" + url.PathEscape(id) + "/image" }
func IconEditURL(id string) string    { return "/icons/" + url.PathEscape(id) + "/edit" }
func IconDetailsURL(id string) string { return "/icons/" + url.PathEscape(id) + "/details" }
func IconPageURL(data IconPage, page int) string {
	path := "/icons"
	if data.Picker {
		path += "/picker"
	}
	query := url.Values{"q": {data.Query}, "page": {strconv.Itoa(page)}}
	for _, source := range data.Sources {
		query.Add("source", source)
	}
	return path + "?" + query.Encode()
}

func iconUsageLabel(count int) string {
	if count == 1 {
		return "Used by 1 service"
	}
	return fmt.Sprintf("Used by %d services", count)
}
