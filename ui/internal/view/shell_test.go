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
