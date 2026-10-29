package plugins

import (
	"context"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	plugintesting "github.com/araihu/balemoh/plugin-sdk/testing"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"strings"
	"testing"
)

func TestServiceCatalogEnforcesInstallationGrants(t *testing.T) {
	id := strings.Repeat("a", 64)
	calls := 0
	source := &Source{Services: plugintesting.Services{ResolveServicesFunc: func(_ context.Context, ids []string) ([]ui.ServiceIdentity, error) {
		calls++
		return []ui.ServiceIdentity{{ID: ids[0], Name: "Pi-hole"}}, nil
	}}}
	entry := &Entry{Installation: Installation{ServiceIDs: []string{id}}, Package: &manifest.Package{Manifest: manifest.Manifest{Capabilities: manifest.Capabilities{Optional: []string{ui.ServiceCatalogCapability}}}}}
	host := scopedServices{source: source, entry: entry}
	if _, err := host.ResolveServices(t.Context(), []string{strings.Repeat("b", 64)}); backend.Code(err) != "permission_denied" || calls != 0 {
		t.Fatal("ungranted ID reached catalog", err)
	}
	result, err := host.ResolveServices(t.Context(), []string{id})
	if err != nil || len(result) != 1 || calls != 1 {
		t.Fatal(result, err)
	}
	entry.Installation.ServiceIDs = nil
	if _, err := host.ResolveServices(t.Context(), []string{id}); backend.Code(err) != "permission_denied" || calls != 1 {
		t.Fatal("other connection inherited grant", err)
	}
	entry.Installation.ServiceIDs = []string{id}
	entry.Package.Manifest.Capabilities.Optional = nil
	if _, err := host.ResolveServices(t.Context(), []string{id}); backend.Code(err) != "permission_denied" || calls != 1 {
		t.Fatal("undeclared capability accepted", err)
	}
}
