package bff

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/balemoh/ui/internal/plugins"
)

func TestRequiredAndOptionalPluginCapabilities(t *testing.T) {
	for _, required := range []bool{true, false} {
		t.Run(map[bool]string{true: "required incompatible", false: "optional page omitted"}[required], func(t *testing.T) {
			r := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) {
				if required {
					m.Capabilities.Required = append(m.Capabilities.Required, "future.required/v1")
				} else {
					m.Capabilities.Optional = append(m.Capabilities.Optional, "future.optional/v1")
					m.Pages[0].Capabilities = []string{"future.optional/v1"}
				}
			})
			if len(r.Navigation()) != 0 {
				t.Fatal("unavailable page still mounted in navigation")
			}
			if required {
				if r.Problems() != 1 || len(r.Widgets()) != 0 || r.Entries()[0].Client() != nil {
					t.Fatal("incompatible installation mounted or started")
				}
			} else if len(r.Widgets()) != 2 || r.Problems() != 0 {
				t.Fatal("unsupported optional page disabled unrelated widgets")
			}
			if _, _, _, err := r.ResolvePage("example.fixture", "00000000-0000-4000-8000-000000000001", "/current", nil); err != plugins.ErrNotFound {
				t.Fatal("unavailable page still addressable")
			}
		})
	}
}

func TestPluginRenderingNeverBlocksBaseResponses(t *testing.T) {
	r := previewRegistry(t, manifest.UIProfile)
	root := r.Entries()[0].Package.Root
	if err := os.WriteFile(filepath.Join(root, "slow"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	h, err := NewWithPlugins(&widgetCatalog{}, 100*time.Millisecond, nil, r)
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		start := time.Now()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if time.Since(start) > 500*time.Millisecond {
			t.Errorf("base route %s waited for plugin", path)
		}
		return w
	}
	page := "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/pages/current"
	for _, path := range []string{"/", page, "/healthz", "/staging", "/hosts"} {
		if w := get(path); w.Code != 200 {
			t.Fatalf("base route %s status %d", path, w.Code)
		}
		if path == "/" && strings.Contains(get(path).Body.String(), "Configure widget") {
			t.Fatal("Home still exposes widget configuration")
		}
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	response, err := client.Get(ts.URL + "/dashboard/events")
	if err != nil {
		t.Fatal("SSE waited for plugin", err)
	}
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	response.Body.Close()
	if err != nil || !strings.HasPrefix(line, "data:") {
		t.Fatal("missing immediate SSE frame", err)
	}
	calls, _ := filepath.Glob(filepath.Join(root, "called-*"))
	if len(calls) != 0 {
		t.Fatalf("base render invoked plugin: %v", calls)
	}
	for _, path := range []string{"/plugin-widgets/00000000-0000-4000-8000-000000000002", strings.Replace(page, "/pages/", "/page-content/", 1)} {
		start := time.Now()
		w := get(path)
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond || w.Code != 503 || !strings.Contains(w.Body.String(), "Retry plugin") || strings.Contains(w.Body.String(), "<html") {
			t.Fatalf("isolated failure %s status %d elapsed %v", path, w.Code, elapsed)
		}
	}
	calls, _ = filepath.Glob(filepath.Join(root, "called-*"))
	if len(calls) != 2 {
		t.Fatalf("expected page and one placement call; got %v", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "called-widget-00000000-0000-4000-8000-000000000003")); !os.IsNotExist(err) {
		t.Fatal("loading one slot invoked its neighbor")
	}
}

func previewRegistry(t *testing.T, profile string) *plugins.Registry {
	return configuredPreviewRegistry(t, profile, nil)
}

func configuredPreviewRegistry(t *testing.T, profile string, modify func(*manifest.Manifest), extraFiles ...map[string][]byte) *plugins.Registry {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(root, "ui")
	cmd := exec.Command("go", "build", "-o", binary, "./runtime/testdata/ui")
	cmd.Dir = "../../../plugin-sdk"
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	for name, data := range map[string]string{"style.css": "[data-plugin-mount]{color:inherit}", "module.js": "export function mount(){}"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, files := range extraFiles {
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: profile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Pages: []manifest.Page{{ID: "current", Title: "Fixture page", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{}}}, Navigation: []manifest.Navigation{{ID: "fixture", Title: "Fixture navigation", Page: "current"}}, Widgets: []manifest.Widget{{ID: "summary", Title: "Fixture widget", OptionsSchema: "schema.json", Capabilities: []string{}}}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "home.widget/v1", "sidebar.navigation/v1"}, Optional: []string{}}}
	if modify != nil {
		modify(&m)
	}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema.json"), []byte(`{"type":"object","additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hung"), []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := plugins.Config{Installations: []plugins.Installation{{Package: root, ConnectionID: "00000000-0000-4000-8000-000000000001", Widgets: []plugins.Placement{{ID: "00000000-0000-4000-8000-000000000002", Definition: "summary"}, {ID: "00000000-0000-4000-8000-000000000003", Definition: "summary"}}}}}
	data, _ = json.Marshal(c)
	path := filepath.Join(root, "preview.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := plugins.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if m.Targets[runtime.GOOS+"/"+runtime.GOARCH].UI != "hung" {
		if err := r.WaitReady(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestPluginPagesWidgetsAndFailureIsolation(t *testing.T) {
	registry := previewRegistry(t, manifest.UIProfile)
	h, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	page := "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/pages/current"
	w := get(page)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "data-plugin-slot") || strings.Contains(w.Body.String(), "Synthetic plugin content") || !strings.Contains(w.Body.String(), "Fixture navigation") {
		t.Fatalf("page not mounted in shell: status %d", w.Code)
	}
	w = get("/")
	html := w.Body.String()
	if w.Code != 200 || strings.Count(html, "data-plugin-slot") != 2 || strings.Contains(html, "Synthetic plugin content") {
		t.Fatalf("two preview placements not rendered: %d", w.Code)
	}
	contentPage := strings.Replace(page, "/pages/", "/page-content/", 1)
	if w := get(contentPage); w.Code != 200 || !strings.Contains(w.Body.String(), "Synthetic plugin content") || strings.Contains(w.Body.String(), "<html") {
		t.Fatal("isolated page content unavailable")
	}
	first := get("/plugin-widgets/00000000-0000-4000-8000-000000000002")
	second := get("/plugin-widgets/00000000-0000-4000-8000-000000000003")
	html = first.Body.String() + second.Body.String()
	ids := regexp.MustCompile(`id="(plugin-[a-f0-9]{64})"`).FindAllStringSubmatch(html, -1)
	if len(ids) != 2 || ids[0][1] == ids[1][1] {
		t.Fatalf("placement DOM identity collision: %v", ids)
	}
	for _, bad := range []string{strings.Replace(page, "000000000001", "000000000099", 1), page + "/extra", strings.Replace(page, "pages/current", "pages/current%2fextra", 1)} {
		if w := get(bad); w.Code < 400 {
			t.Errorf("unexpected route accepted: %s %d", bad, w.Code)
		}
	}
	if w := get("/staging"); w.Code != 200 {
		t.Fatal("core staging broke with plugin enabled")
	}
	registry.Close()
	w = get(page)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Plugin unavailable") || !strings.Contains(w.Body.String(), "Staging") {
		t.Fatal("child failure removed shell or lacked local fallback")
	}
	if w := get(contentPage); w.Code != 503 || !strings.Contains(w.Body.String(), "Plugin unavailable") {
		t.Fatal("missing local content failure")
	}
	w = get("/")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Plugin unavailable") {
		t.Fatal("home unavailable after child failure")
	}
	if w := get("/healthz"); w.Code != 200 {
		t.Fatal("optional plugin failure failed core health")
	}
	if w := get("/staging"); w.Code != 200 {
		t.Fatal("optional plugin failure failed core staging")
	}
}

func TestIncompatiblePluginDoesNotStartOrBreakCore(t *testing.T) {
	r := previewRegistry(t, "unsupported/profile")
	if r.Problems() != 1 {
		t.Fatal("missing incompatible-profile diagnostic")
	}
	h, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, r)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Plugin unavailable") {
		t.Fatal("incompatible plugin broke home")
	}
}

func TestPluginStartupNeverDelaysCoreListener(t *testing.T) {
	registry := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) {
		target := m.Targets[runtime.GOOS+"/"+runtime.GOARCH]
		target.UI = "hung"
		m.Targets[runtime.GOOS+"/"+runtime.GOARCH] = target
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if err := registry.WaitReady(ctx); err == nil {
		t.Fatal("fixture child did not hang during startup")
	}
	handler, err := NewWithPlugins(&widgetCatalog{}, 100*time.Millisecond, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/healthz", "/staging", "/", "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/pages/current"} {
		start := time.Now()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || time.Since(start) > 500*time.Millisecond {
			t.Fatalf("hung startup blocked core %s: %d", path, w.Code)
		}
	}
	start := time.Now()
	registry.Close()
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancelled startup did not stop owned child promptly")
	}
}

func TestPluginNativeFallbackRemainsSeparateFromShell(t *testing.T) {
	registry := previewRegistry(t, manifest.UIProfile)
	h, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	page := "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/pages/current"
	shell := httptest.NewRecorder()
	h.ServeHTTP(shell, httptest.NewRequest("GET", page, nil))
	if !strings.Contains(shell.Body.String(), "<noscript><iframe") || !strings.Contains(shell.Body.String(), "__native=1") {
		t.Fatal("native asynchronous fallback missing")
	}
	content := httptest.NewRecorder()
	h.ServeHTTP(content, httptest.NewRequest("GET", strings.Replace(page, "/pages/", "/page-content/", 1)+"?__native=1", nil))
	if content.Code != 200 || !strings.Contains(content.Body.String(), "<base target=\"_top\"") || !strings.Contains(content.Body.String(), "Synthetic plugin content") || strings.Contains(content.Body.String(), "consoleshell-menu") {
		t.Fatal("native content nested shell or lost top-level navigation")
	}
}

func TestDeclaredAssetsInNativeDocumentAndBeforeStatus(t *testing.T) {
	registry := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) {
		m.Assets = []manifest.Asset{{ID: "style", File: "style.css", Kind: "style", MediaType: "text/css"}, {ID: "module", File: "module.js", Kind: "module", MediaType: "text/javascript"}}
	})
	server := &server{plugins: registry}
	request := httptest.NewRequest("GET", "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/page-content/current?__native=1", nil)
	request.SetPathValue("pluginID", "example.fixture")
	request.SetPathValue("connectionID", "00000000-0000-4000-8000-000000000001")
	response := pluginui.Response{Kind: "content", Status: 200, HTML: []byte("<p>Native fixture</p>"), AssetIDs: []string{"style", "module"}}
	w := httptest.NewRecorder()
	server.pluginContent(w, request, response, nil)
	href := "/plugins/assets/example.fixture/" + registry.Entries()[0].Package.Digest + "/style.css"
	if w.Code != 200 || !strings.Contains(w.Body.String(), `href="`+href+`"`) || strings.Contains(w.Body.String(), "module.js") {
		t.Fatal("native document lost declared styles or included JavaScript", w.Code)
	}
	handler, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest("GET", href, nil))
	if asset.Code != 200 || asset.Body.String() != "[data-plugin-mount]{color:inherit}" {
		t.Fatal("declared asset unavailable")
	}
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest("GET", href+"%3Fbad", nil))
	if invalid.Code != 404 {
		t.Fatal("URL metacharacter changed asset identity")
	}
	response.AssetIDs = []string{"missing"}
	w = httptest.NewRecorder()
	server.pluginContent(w, request, response, nil)
	if w.Code != 502 {
		t.Fatal("asset resolution committed success status", w.Code)
	}
}

func TestPluginSidebarUsesDeclaredIconsAndSafeFallback(t *testing.T) {
	for _, tc := range []struct{ name, symbol, want string }{
		{"declared", "hi-16-solid-bell", "hi-16-solid-bell"},
		{"omitted", "", "hi-16-solid-squares-2x2"},
		{"unknown", "future-icon", "hi-16-solid-squares-2x2"},
		{"URL is not loaded", "https://example.invalid/icon.svg", "hi-16-solid-squares-2x2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) { m.Navigation[0].Icon = tc.symbol })
			h, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, registry)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "#"+tc.want) {
				t.Fatal("plugin sidebar icon missing")
			}
			if tc.symbol != "" && tc.symbol != tc.want && strings.Contains(w.Body.String(), tc.symbol) {
				t.Fatal("unknown icon reached HTML")
			}
		})
	}
}
