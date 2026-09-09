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
	for _, want := range []string{`placeholder="https://discovered.example/"`, `value="https://custom.example/"`, `aria-label="Homepage card preview"`, `balemoh-service-card`, `Team workspace`, `href="https://custom.example/"`, `x-bind:src="iconPreview"`} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("editor missing %s", want)
		}
	}
	if service.Address != "https://custom.example/" {
		t.Fatal("placeholder changed address override")
	}
	if discoveredAddress(Service{}) != "" {
		t.Fatal("undiscovered address should have empty placeholder")
	}
}
