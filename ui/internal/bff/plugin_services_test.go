package bff

import (
	"errors"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"strings"
	"testing"
)

func TestPluginServicesFollowEffectiveCatalogPresentation(t *testing.T) {
	id := strings.Repeat("a", 64)
	hidden := strings.Repeat("b", 64)
	missing := strings.Repeat("c", 64)
	catalog := &fakeCatalog{staging: []api.ServiceCandidate{
		{Id: id, DisplayName: "Pi-hole", Status: "live", Resource: api.ResourceRef{Name: "pi-hole"}, Source: api.SourceRef{Kind: "container"}},
		{Id: hidden, DisplayName: "Hidden", Status: "hidden"},
		{Id: missing, DisplayName: "Missing", Status: "live", Missing: true},
	}}
	resolver := ServiceCatalog{Catalog: catalog}
	result, err := resolver.ResolveServices(t.Context(), []string{id, hidden, missing, strings.Repeat("d", 64)})
	if err != nil || len(result) != 1 || result[0].Name != "Pi-hole" || result[0].IconURL == "" {
		t.Fatal(result, err)
	}
	original := result[0].IconURL
	upload := "upload:test"
	catalog.staging[0].Icon = &upload
	catalog.staging[0].DisplayName = "DNS"
	result, err = resolver.ResolveServices(t.Context(), []string{id})
	if err != nil || len(result) != 1 || result[0].Name != "DNS" || result[0].IconURL != "/icons/upload:test/image" || result[0].IconURL == original {
		t.Fatal("override did not update", result, err)
	}
	catalog.stagingErr = errors.New("private upstream failure")
	if _, err = resolver.ResolveServices(t.Context(), []string{id}); backend.Code(err) != "unavailable" {
		t.Fatal(err)
	}
}
