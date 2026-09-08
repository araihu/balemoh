package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHomepageCardAddresses(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		service := Service{ID: "app", DisplayName: "Application", Description: "Team workspace", Namespace: "apps"}
		for _, address := range []string{"https://app.example/", "https://admin.example/"}[:count] {
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
			items := addressMenuItems([]string{"https://admin.example/"})
			if len(items) != 1 || items[0].Href != "https://admin.example/" || items[0].Target != "_blank" {
				t.Fatalf("extra address: %#v", items)
			}
		}
	}
}
