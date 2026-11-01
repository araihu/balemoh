package view

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/araihu/goshtoso/components/sidebar"
	"golang.org/x/net/html"
)

func TestConfigIsLastAndContainsPluginManagement(t *testing.T) {
	for _, withPlugins := range []bool{false, true} {
		data := PageData{Title: "Settings", Description: "Application settings", Active: "nav-presentation"}
		want := []string{"nav-home", "nav-staging", "nav-hosts", "nav-config"}
		if withPlugins {
			data.Active = "nav-plugin-widgets"
			data.PluginNavigation = []sidebar.Item{
				{ID: "nav-plugin-widgets", Label: "Plugins/Widgets", Href: "/plugins/widgets"},
				{ID: "nav-alerts", Label: "Alerts", Href: "/plugins/alerts"},
			}
			want = slices.Insert(want, 3, "nav-alerts")
		}
		var body bytes.Buffer
		if err := ConsolePage(data).Render(t.Context(), &body); err != nil {
			t.Fatal(err)
		}
		doc, err := html.Parse(&body)
		if err != nil {
			t.Fatal(err)
		}
		nodes := map[string]*html.Node{}
		var order []string
		for node := range doc.Descendants() {
			for _, attr := range node.Attr {
				if attr.Key == "data-consoleshell-nav-id" {
					if nodes[attr.Val] != nil {
						t.Fatalf("duplicate navigation item %q", attr.Val)
					}
					nodes[attr.Val] = node
					order = append(order, attr.Val)
				}
			}
		}
		var topLevel []string
		for _, id := range order {
			if nodes[id].Parent == nodes["nav-home"].Parent || id == "nav-config" && nodes[id].Parent.Parent == nodes["nav-home"].Parent {
				topLevel = append(topLevel, id)
			}
		}
		if !slices.Equal(topLevel, want) {
			t.Fatalf("withPlugins=%v: top-level items = %v, want %v", withPlugins, topLevel, want)
		}
		settings := nodes["nav-notification-settings"]
		if settings == nil || settings.Parent != nodes["nav-presentation"].Parent {
			t.Fatal("notification settings must be a Config child alongside Appearance")
		}
		if withPlugins {
			widget := nodes["nav-plugin-widgets"]
			if widget == nil || widget.Parent != nodes["nav-presentation"].Parent || widget.Parent.Parent != nodes["nav-config"].Parent {
				t.Fatal("plugin management must be a sibling of Appearance inside Config")
			}
		}
	}
}

func TestConfigDisclosureOpensForActiveChildren(t *testing.T) {
	for _, active := range []string{"nav-home", "nav-hosts", "nav-alerts", "nav-presentation", "nav-notification-settings", "nav-plugin-widgets"} {
		t.Run(active, func(t *testing.T) {
			data := PageData{Title: "Navigation", Description: "Sidebar navigation", Active: active, PluginNavigation: []sidebar.Item{{ID: "nav-plugin-widgets", Label: "Plugins/Widgets", Href: "/plugins/widgets"}, {ID: "nav-alerts", Label: "Alerts", Href: "/plugins/alerts"}}}
			var body bytes.Buffer
			if err := ConsolePage(data).Render(t.Context(), &body); err != nil {
				t.Fatal(err)
			}
			doc, err := html.Parse(&body)
			if err != nil {
				t.Fatal(err)
			}
			var control, children *html.Node
			for node := range doc.Descendants() {
				for _, attr := range node.Attr {
					if attr.Key == "data-consoleshell-nav-id" && attr.Val == "nav-config" {
						control = node
					}
					if attr.Key == "id" && attr.Val == "nav-config-children" {
						children = node
					}
				}
			}
			if control == nil || control.Data != "button" || children == nil {
				t.Fatal("Config must expose a disclosure button and controlled children")
			}
			want := "false"
			if active == "nav-presentation" || active == "nav-notification-settings" || active == "nav-plugin-widgets" {
				want = "true"
			}
			expanded := ""
			for _, attr := range control.Attr {
				if attr.Key == "aria-expanded" {
					expanded = attr.Val
				}
			}
			if expanded != want {
				t.Fatalf("Config expanded=%q, want %q", expanded, want)
			}
			hidden := false
			for _, attr := range children.Attr {
				hidden = hidden || attr.Key == "style" && strings.Contains(attr.Val, "display: none")
			}
			if hidden != (want == "false") {
				t.Fatal("initial child visibility must match the active page")
			}
		})
	}
}

func TestSidebarRendersNavigationIcons(t *testing.T) {
	var b bytes.Buffer
	if err := ConsolePage(PageData{Title: "Hosts", Description: "Host metrics", Path: "/hosts", Active: "nav-hosts", HostsPage: &HostsPage{}}).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"hi-16-solid-home", "hi-16-solid-queue-list", "heroicons-server", "hi-16-solid-paint-brush"} {
		if !strings.Contains(b.String(), "#"+symbol) {
			t.Errorf("sidebar omitted %s", symbol)
		}
	}
}

func TestHeaderDoesNotOverridePersistedAppearance(t *testing.T) {
	for _, tt := range []struct {
		name, theme, mode string
		wantControl       bool
	}{
		{name: "default"},
		{name: "system", theme: "goshtoso", mode: "system"},
		{name: "forced light", theme: "goshtoso", mode: "light"},
		{name: "forced dark", theme: "goshtoso", mode: "dark"},
		{name: "forced theme", theme: "minimal", mode: "system"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if err := ConsolePage(PageData{Title: "Home", Description: "Homepage services", Theme: tt.theme, ColorScheme: tt.mode}).Render(context.Background(), &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.String(), `id="notification-summary"`) {
				t.Fatal("notification header missing")
			}
			if got := strings.Contains(body.String(), `id="balemoh-color-mode"`); got != tt.wantControl {
				t.Fatalf("theme control present = %v, want %v", got, tt.wantControl)
			}
		})
	}
}
