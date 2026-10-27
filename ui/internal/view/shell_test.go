package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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

func TestColorModeControlHonorsAppearanceOverrides(t *testing.T) {
	for _, tt := range []struct {
		name, theme, mode string
		wantControl       bool
	}{
		{name: "default", wantControl: true},
		{name: "system", theme: "goshtoso", mode: "system", wantControl: true},
		{name: "forced light", theme: "goshtoso", mode: "light"},
		{name: "forced dark", theme: "goshtoso", mode: "dark"},
		{name: "forced theme", theme: "minimal", mode: "system"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if err := ConsolePage(PageData{Title: "Home", Description: "Homepage services", Theme: tt.theme, ColorScheme: tt.mode}).Render(context.Background(), &body); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(body.String(), `id="balemoh-color-mode"`); got != tt.wantControl {
				t.Fatalf("theme control present = %v, want %v", got, tt.wantControl)
			}
		})
	}
}
