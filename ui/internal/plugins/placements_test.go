package plugins

import (
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStaleWriteReconcilesExactNumericDesiredState(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	current := PlacementRecord{PluginID: "example.fixture", ConnectionID: id, ContributionID: "summary", PlacementID: id, Revision: "2", Options: json.RawMessage(`{"a":1.0,"n":9007199254740993.0}`)}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			w.WriteHeader(412)
			return
		}
		_ = json.NewEncoder(w).Encode(WidgetCatalog{Placements: []PlacementRecord{current}})
	}))
	defer api.Close()
	source, err := NewSource(api.URL, tokenFixture(t), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	registry := &Registry{source: source}
	desired := current
	desired.Revision = "1"
	desired.Options = json.RawMessage(`{"n":9007199254740993,"a":1}`)
	got, err := registry.SavePlacement(t.Context(), desired, false)
	if err != nil || got.Revision != "2" {
		t.Fatal("equivalent stale desired state failed", err)
	}
	desired.Options = json.RawMessage(`{"n":9007199254740994,"a":1}`)
	if _, err = registry.SavePlacement(t.Context(), desired, false); err == nil {
		t.Fatal("rounded a different desired value into success")
	}
}

func TestRetainedWidgetIgnoresPollTimesButRejectsChangedDataAndGrants(t *testing.T) {
	const connection = "00000000-0000-4000-8000-000000000001"
	const placement = "00000000-0000-4000-8000-000000000002"
	root := t.TempDir()
	binary := filepath.Join(root, "ui")
	command := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./runtime/testdata/ui")
	command.Dir = "../../../plugin-sdk"
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture %v %s", err, output)
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Pages: []manifest.Page{}, Navigation: []manifest.Navigation{}, Widgets: []manifest.Widget{{ID: "summary", Title: "Summary", OptionsSchema: "schema.json", Capabilities: []string{}}}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"home.widget/v1"}, Optional: []string{}}}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema.json"), []byte(`{"type":"object","additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	pkg, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	child, err := pluginruntime.Open(t.Context(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	one := "1"
	observed := time.Now().UTC()
	metadata := api.PluginConnection{Id: connection, Enabled: true, Ready: true, Revision: "1", AppliedRevision: &one, ObservationRevision: &one, State: "fresh", Queries: []string{}, ObservedAt: &observed, LastSuccessAt: &observed, LastAttemptAt: &observed}
	current := PlacementRecord{PluginID: m.ID, ConnectionID: connection, ContributionID: "summary", PlacementID: placement, Revision: "1", Options: json.RawMessage(`{}`), Enabled: true}
	var mu sync.Mutex
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/plugin-widgets":
			_ = json.NewEncoder(w).Encode(WidgetCatalog{Placements: []PlacementRecord{current}})
		case "/api/v1/plugins":
			_ = json.NewEncoder(w).Encode(api.PluginCatalog{Problems: []api.PluginInstallationProblem{}, Plugins: []api.PluginDescriptor{{Id: m.ID, ManifestDigest: pkg.Digest, Connections: []api.PluginConnection{metadata}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer endpoint.Close()
	source, err := NewSource(endpoint.URL, tokenFixture(t), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	registry := &Registry{source: source, entries: []*Entry{{Installation: Installation{ConnectionID: connection}, Package: pkg, Compatible: true, process: &uiProcess{client: child}}}}
	if _, e := registry.CurrentPlacement(t.Context(), placement); e != nil {
		t.Fatal("placement metadata", e)
	}
	if _, e := source.descriptors(t.Context()); e != nil {
		t.Fatal("descriptor metadata", e)
	}
	first, err := registry.PersistentWidget(t.Context(), placement)
	if err != nil || first.Status != 200 || !strings.Contains(string(first.HTML), "Synthetic plugin content") {
		t.Fatal("initial render", err)
	}
	child.Close()
	mu.Lock()
	later := observed.Add(time.Minute)
	metadata.ObservedAt = &later
	metadata.LastSuccessAt = &later
	metadata.LastAttemptAt = &later
	metadata.LastConfigureAttemptAt = &later
	mu.Unlock()
	retained, err := registry.PersistentWidget(t.Context(), placement)
	if err != nil || retained.Status != 503 || !strings.Contains(string(retained.HTML), "Showing last rendered content") || !strings.Contains(string(retained.HTML), "Synthetic plugin content") {
		t.Fatal("diagnostic timestamps invalidated retained render", err)
	}
	mu.Lock()
	two := "2"
	metadata.ObservationRevision = &two
	mu.Unlock()
	if response, err := registry.PersistentWidget(t.Context(), placement); err == nil || len(response.HTML) != 0 {
		t.Fatal("changed observation reused old content")
	}
	mu.Lock()
	metadata.ObservationRevision = &one
	metadata.Revision = "2"
	metadata.AppliedRevision = &two
	mu.Unlock()
	if response, err := registry.PersistentWidget(t.Context(), placement); err == nil || len(response.HTML) != 0 {
		t.Fatal("changed configuration reused old content")
	}
	mu.Lock()
	metadata.Revision = "1"
	metadata.AppliedRevision = &one
	metadata.Queries = []string{"example.read"}
	mu.Unlock()
	if response, err := registry.PersistentWidget(t.Context(), placement); err == nil || len(response.HTML) != 0 {
		t.Fatal("changed grants reused old content")
	}
}
