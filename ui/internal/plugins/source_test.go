package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func tokenFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(strings.Repeat("private", 6)), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestSourceScopesGrantAndRejectsRedirectsAndOversizedData(t *testing.T) {
	var calls atomic.Int32
	var mode atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(100) }))
	defer target.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("private", 6) {
			t.Error("credential missing")
		}
		if r.URL.Path != "/api/v1/plugins/example.fixture/connections/current/queries/fixture.read" {
			t.Error("callback escaped scope", r.URL.Path)
		}
		switch mode.Load() {
		case 1:
			http.Redirect(w, r, target.URL, 302)
		case 2:
			fmt.Fprint(w, strings.Repeat("x", backend.MaxResult+(65<<10)))
		default:
			fmt.Fprint(w, `{"data":{"value":"safe"},"state":"unavailable","observationRevision":"1","observedAt":null,"lastSuccessAt":null}`)
		}
	}))
	defer api.Close()
	source, err := NewSource(api.URL, tokenFixture(t), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "schema.json"), []byte(`{"type":"object"}`), 0600); err != nil {
		t.Fatal(err)
	}
	entry := &Entry{Installation: Installation{ConnectionID: "current"}, Package: &manifest.Package{Root: root, Manifest: manifest.Manifest{ID: "example.fixture", Queries: []manifest.Query{{ID: "fixture.read", InputSchema: "schema.json", OutputSchema: "schema.json"}}}}}
	h := scopedServices{source: source, entry: entry, grants: map[string]bool{"fixture.read": true}, placement: ui.Placement{ID: "placement", Options: json.RawMessage(`{}`)}}
	if _, err := h.Query(t.Context(), "undeclared", json.RawMessage(`{}`)); backend.Code(err) != "permission_denied" || calls.Load() != 0 {
		t.Fatal("ungranted query reached API")
	}
	if _, err := h.Query(t.Context(), "fixture.read", json.RawMessage(`null`)); backend.Code(err) != "invalid_argument" || calls.Load() != 0 {
		t.Fatal("invalid input reached API")
	}
	if _, err := h.Query(t.Context(), "fixture.read", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	placement, err := h.Placement(context.Background())
	if err != nil || placement.ID != "placement" {
		t.Fatal("placement scope lost")
	}
	mode.Store(1)
	if _, err := h.Query(t.Context(), "fixture.read", json.RawMessage(`{}`)); err == nil || calls.Load() != 2 {
		t.Fatal("credential followed redirect")
	}
	mode.Store(2)
	if _, err := h.Query(t.Context(), "fixture.read", json.RawMessage(`{}`)); backend.Code(err) != "invalid_response" {
		t.Fatal("oversized API data accepted")
	}
}
func TestDataDiscoveryFailureLeavesCoreRegistryUsable(t *testing.T) {
	// A valid pinned package remains unmounted on API outage, without stopping BFF.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer api.Close()
	source, err := NewSource(api.URL, tokenFixture(t), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	pkg := filepath.Join(root, "package")
	if err := os.Mkdir(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "ui"), []byte("fixture never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "schema.json"), []byte(`{"type":"object"}`), 0600); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Pages: []manifest.Page{{ID: "current", Title: "Current", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{}}}, Navigation: []manifest.Navigation{{ID: "current", Title: "Current", Page: "current"}}, Widgets: []manifest.Widget{}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "sidebar.navigation/v1"}, Optional: []string{}}}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(pkg, "plugin.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := manifest.Load(pkg)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := p.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(Config{Installations: []Installation{{Package: pkg, ManifestDigest: p.Digest, PackageDigest: digest, ConnectionKey: "one"}}})
	path := filepath.Join(root, "plugins.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	registry, err := OpenData(t.Context(), path, source)
	if err != nil {
		t.Fatal("optional plugin stopped BFF startup", err)
	}
	defer registry.Close()
	if err := registry.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	if registry.Problems() != 1 || len(registry.Navigation()) != 0 || len(registry.Widgets()) != 0 {
		t.Fatal("failed discovery mounted invalid identity")
	}
}

func TestCredentialLoaderRejectsUnusableBearerTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	for _, value := range []string{strings.Repeat("x", 32) + " space", strings.Repeat("x", 32) + "\ttab", strings.Repeat("x", 32) + "\x01control", strings.Repeat("x", 32) + "\x7fcontrol"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewSource("http://127.0.0.1", path, time.Second); err == nil {
			t.Fatal("BFF accepted unusable Bearer credential")
		}
	}
}

func TestNavigationNamesEachConnectionAndKeepsScopedLinks(t *testing.T) {
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Widgets: []manifest.Widget{}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "sidebar.navigation/v1"}, Optional: []string{}}, Pages: []manifest.Page{{ID: "current", Title: "Current", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{}}}, Navigation: []manifest.Navigation{{ID: "alerts", Title: "Alerts", Page: "current"}}}
	first := "00000000-0000-4000-8000-000000000001"
	second := "00000000-0000-4000-8000-000000000002"
	registry := &Registry{entries: []*Entry{{Installation: Installation{ConnectionID: first}, Package: &manifest.Package{Manifest: m}, Compatible: true, ConnectionName: "Primary Alertmanager"}, {Installation: Installation{ConnectionID: second}, Package: &manifest.Package{Manifest: m}, Compatible: true, ConnectionName: "Secondary Alertmanager"}}}
	navigation := registry.Navigation()
	if len(navigation) != 2 || navigation[0].Title != "Primary Alertmanager" || navigation[1].Title != "Secondary Alertmanager" || len(navigation[0].Children) != 1 || len(navigation[1].Children) != 1 {
		t.Fatal("connection groups lost identity")
	}
	if navigation[0].ID == navigation[1].ID || !strings.Contains(navigation[0].Children[0].Href, first) || !strings.Contains(navigation[1].Children[0].Href, second) {
		t.Fatal("connection deep links collide")
	}
	registry.source = &Source{}
	navigation = registry.Navigation()
	for _, connection := range navigation {
		settings := connection.Children[1]
		if settings.Title != "Configure widgets" || settings.Icon != "hi-16-solid-cog-6-tooth" {
			t.Fatal("widget settings lost its host-owned icon")
		}
	}
}
