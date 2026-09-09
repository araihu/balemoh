package view

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHomepageCardAddresses(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3} {
		service := Service{ID: "app", DisplayName: "Application", Description: "Team workspace", Namespace: "apps"}
		for _, address := range []string{"https://app.example/", "https://admin.example/", "https://status.example/"}[:count] {
			service.Endpoints = append(service.Endpoints, Endpoint{URL: address})
		}
		var html bytes.Buffer
		if err := HomepageContent(PageData{Services: []Service{service}}).Render(context.Background(), &html); err != nil {
			t.Fatal(err)
		}
		body := html.String()
		if !strings.Contains(body, "Team workspace") || strings.Contains(body, "<details") || strings.Contains(body, "<table") {
			t.Fatalf("unexpected card contents: %s", body)
		}
		if got := strings.Contains(body, "More addresses for Application"); got != (count > 1) {
			t.Fatalf("%d addresses: dropdown = %v", count, got)
		}
		if count == 0 && !strings.Contains(body, "No public address") {
			t.Fatal("missing empty address state")
		}
		if count > 0 && !strings.Contains(body, `href="https://app.example/" target="_blank"`) {
			t.Fatal("first address missing or not a new-tab link")
		}
		if count > 1 {
			if !strings.Contains(body, fmt.Sprintf("+%d", count-1)) {
				t.Fatalf("%d addresses: wrong remaining count", count)
			}
			items := addressMenuItems([]string{"https://admin.example/"})
			if len(items) != 1 || items[0].Href != "https://admin.example/" || items[0].Target != "_blank" {
				t.Fatalf("extra address: %#v", items)
			}
		}
	}
}

func TestServiceEditorCardAndDiscoveredPlaceholder(t *testing.T) {
	service := Service{ID: "app", DisplayName: "Application", Description: "Team workspace", Address: "https://custom.example/", Endpoints: []Endpoint{{URL: "https://discovered.example/"}}}
	var html bytes.Buffer
	if err := ServiceEditor(service).Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`placeholder="https://discovered.example/"`, `value="https://custom.example/"`, `aria-label="Homepage card preview"`, `balemoh-service-card`, `Team workspace`, `href="https://custom.example/"`, `x-bind:src="iconURL"`} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("editor missing %s", want)
		}
	}
	if strings.Index(html.String(), "</form>") > strings.Index(html.String(), "<dialog") {
		t.Fatal("picker controls must not belong to the service save form")
	}
	if !strings.Contains(html.String(), `<dialog id="service-icon-picker"`) {
		t.Fatal("editor must mount its icon picker dialog")
	}
	if service.Address != "https://custom.example/" {
		t.Fatal("placeholder changed address override")
	}
	if discoveredAddress(Service{}) != "" {
		t.Fatal("undiscovered address should have empty placeholder")
	}
}

func TestDiscoveredAddressScheme(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"//app.example/", "https://app.example/"},
		{"http://app.example/", "http://app.example/"},
		{"https://app.example/", "https://app.example/"},
		{"", ""},
	} {
		service := Service{Address: "https://custom.example/", Endpoints: []Endpoint{{URL: tc.url}}}
		if got := discoveredAddress(service); got != tc.want {
			t.Errorf("discoveredAddress(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestHomepageSearchIsScopedToServices(t *testing.T) {
	for _, services := range [][]Service{nil, {{DisplayName: `Team "A"`, Description: "Workspace", Endpoints: []Endpoint{{URL: "https://app.example/"}}}}} {
		var html bytes.Buffer
		if err := HomepageContent(PageData{Services: services}).Render(context.Background(), &html); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(html.String(), `id="home-search"`) != (len(services) > 0) {
			t.Fatal("search requires service cards")
		}
		if len(services) > 0 && (!strings.Contains(html.String(), "app.example") || !strings.Contains(html.String(), `x-on:keydown.window`)) {
			t.Fatal("missing search data or shortcut")
		}
	}
}
