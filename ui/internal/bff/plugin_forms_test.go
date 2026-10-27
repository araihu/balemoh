package bff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/plugins"
	"net/url"
	"strings"
	"testing"

	"github.com/araihu/balemoh/ui/internal/view"
)

func TestPluginNumericOptionsRoundTripWithoutFloatConversion(t *testing.T) {
	for _, tc := range []struct{ kind, value string }{{"integer", "9007199254740993"}, {"number", "9007199254740993.125"}} {
		t.Run(tc.kind, func(t *testing.T) {
			original := []byte(`{"n":` + tc.value + `}`)
			values := view.PluginValues(original)
			form := view.PluginForm{Values: values}
			shown := form.Value("n")
			if shown != tc.value {
				t.Fatalf("form rounded %s to %s", tc.value, shown)
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"option.n": {shown}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_ = r.ParseForm()
			raw, err := pluginFormOptions([]byte(`{"type":"object","properties":{"n":{"type":"`+tc.kind+`","minimum":0,"maximum":1000000000000000000}},"additionalProperties":false}`), r)
			if err != nil || string(raw) != string(original) {
				t.Fatalf("save rounded value: %s %v", raw, err)
			}
		})
	}
}

// The API fixture owns records and revisions independently of rendered forms.
type widgetManagementFixture struct {
	mu          sync.Mutex
	catalog     plugins.WidgetCatalog
	unavailable bool
	writes      int
}

func widgetManagementHandler(t *testing.T) (http.Handler, *widgetManagementFixture) {
	t.Helper()
	preview := previewRegistry(t, manifest.UIProfile)
	pkg := preview.Entries()[0].Package
	preview.Close()
	schema := json.RawMessage(`{"type":"object","properties":{"search":{"type":"string","title":"Search","maxLength":256},"maxRows":{"type":"integer","title":"Maximum rows","minimum":1,"maximum":20}} ,"additionalProperties":false}`)
	const connection = "00000000-0000-4000-8000-000000000001"
	f := &widgetManagementFixture{catalog: plugins.WidgetCatalog{Definitions: []plugins.Definition{{PluginID: pkg.Manifest.ID, ContributionID: "summary", Title: "Fixture widget", OptionsSchema: schema}}, Placements: []plugins.PlacementRecord{
		{PluginID: pkg.Manifest.ID, ConnectionID: connection, ContributionID: "summary", PlacementID: "00000000-0000-4000-8000-000000000002", Revision: "2", Options: json.RawMessage(`{"search":"first","maxRows":5}`), Enabled: true},
		{PluginID: pkg.Manifest.ID, ConnectionID: connection, ContributionID: "summary", PlacementID: "00000000-0000-4000-8000-000000000003", Revision: "1", Options: json.RawMessage(`{"search":"second"}`)},
	}}}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.URL.Path == "/api/v1/plugins" {
			_ = json.NewEncoder(w).Encode(map[string]any{"plugins": []any{map[string]any{"id": pkg.Manifest.ID, "manifestDigest": pkg.Digest, "connections": []any{map[string]any{"id": connection, "key": "one", "name": "Primary", "enabled": true}}}}, "problems": []any{}})
			return
		}
		if f.unavailable {
			w.WriteHeader(503)
			return
		}
		if r.URL.Path == "/api/v1/plugin-widgets" && r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(f.catalog)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/plugin-widgets/")
		index := -1
		for i, p := range f.catalog.Placements {
			if p.PlacementID == id {
				index = i
				break
			}
		}
		if r.Method == "DELETE" {
			if index < 0 {
				w.WriteHeader(404)
				return
			}
			if r.Header.Get("If-Match") != `"`+f.catalog.Placements[index].Revision+`"` {
				w.WriteHeader(412)
				return
			}
			f.catalog.Placements = append(f.catalog.Placements[:index], f.catalog.Placements[index+1:]...)
			f.writes++
			w.WriteHeader(204)
			return
		}
		if r.Method == "PUT" {
			var p plugins.PlacementRecord
			if json.NewDecoder(r.Body).Decode(&p) != nil {
				w.WriteHeader(400)
				return
			}
			p.PlacementID = id
			if index >= 0 {
				if r.Header.Get("If-Match") != `"`+f.catalog.Placements[index].Revision+`"` {
					w.WriteHeader(412)
					return
				}
				n, _ := strconv.Atoi(f.catalog.Placements[index].Revision)
				p.Revision = strconv.Itoa(n + 1)
				f.catalog.Placements[index] = p
			} else {
				p.Revision = "1"
				f.catalog.Placements = append(f.catalog.Placements, p)
				w.WriteHeader(201)
			}
			f.writes++
			_ = json.NewEncoder(w).Encode(p)
			return
		}
		t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(404)
	}))
	t.Cleanup(api.Close)
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte(strings.Repeat("t", 32)), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := plugins.NewSource(api.URL, token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := pkg.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	config := plugins.Config{Installations: []plugins.Installation{{Package: pkg.Root, ManifestDigest: pkg.Digest, PackageDigest: digest, ConnectionKey: "one"}}}
	data, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "plugins.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := plugins.OpenData(t.Context(), path, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(registry.Close)
	if err := registry.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler, err := NewWithPlugins(&widgetCatalog{}, time.Second, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	return handler, f
}
func widgetRequest(h http.Handler, method, path string, values url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestWidgetManagementListAndIndividualEditors(t *testing.T) {
	h, f := widgetManagementHandler(t)
	list := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if list.Code != 200 || strings.Count(list.Body.String(), "<table ") != 1 || !strings.Contains(list.Body.String(), `id="plugin-widgets-table"`) || strings.Contains(list.Body.String(), `name="option.search"`) || strings.Count(list.Body.String(), ">Plugins/Widgets</span>") != 1 {
		t.Fatalf("list mixed editors or duplicated sidebar: %d", list.Code)
	}
	for _, id := range []string{"00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"} {
		if !strings.Contains(list.Body.String(), `href="/plugin-widgets/`+id+`/edit"`) {
			t.Fatal("missing native detail link", id)
		}
		edit := widgetRequest(h, "GET", widgetEditURL(id), nil)
		if edit.Code != 200 || strings.Count(edit.Body.String(), `name="option.search"`) != 1 || !strings.Contains(edit.Body.String(), `id="placement-form-`+id+`"`) || strings.Contains(edit.Body.String(), ">Add Fixture widget<") {
			t.Fatalf("editor not scoped: %d", edit.Code)
		}
	}
	newURL := widgetNewURL("example.fixture", "00000000-0000-4000-8000-000000000001", "summary")
	created := widgetRequest(h, "GET", newURL, nil)
	if created.Code != 200 || strings.Count(created.Body.String(), `name="option.search"`) != 1 || strings.Contains(created.Body.String(), "Delete widget") {
		t.Fatal("new form mixed saved controls")
	}
	legacy := widgetRequest(h, "GET", widgetManagementURL("example.fixture", "00000000-0000-4000-8000-000000000001"), nil)
	if legacy.Code != 302 || legacy.Header().Get("Location") != "/plugins/widgets" {
		t.Fatal("legacy list bookmark broken")
	}
	missing := widgetRequest(h, "GET", "/plugin-widgets/00000000-0000-4000-8000-000000000099/edit", nil)
	if missing.Code != 404 || !strings.Contains(missing.Body.String(), "Back to Plugins/Widgets") || !strings.Contains(missing.Body.String(), "<h1") {
		t.Fatalf("missing detail: status %d recovery %v body tail %s", missing.Code, strings.Contains(missing.Body.String(), "Back to Plugins/Widgets"), missing.Body.String()[max(0, missing.Body.Len()-1800):])
	}
	f.mu.Lock()
	f.unavailable = true
	f.mu.Unlock()
	outage := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if outage.Code != 503 || !strings.Contains(outage.Body.String(), "Back to Plugins/Widgets") || strings.Contains(outage.Body.String(), "No widgets available") {
		t.Fatal("outage confused with empty list")
	}
	for _, path := range []string{"/", "/healthz", "/staging"} {
		if w := widgetRequest(h, "GET", path, nil); w.Code != 200 {
			t.Fatal("management outage broke base", path)
		}
	}
}
func TestWidgetEditorWritesStayScopedAndRetainConflictDraft(t *testing.T) {
	h, f := widgetManagementHandler(t)
	const id = "00000000-0000-4000-8000-000000000002"
	options := url.Values{"revision": {"2"}, "enabled": {"true"}, "option.search": {"changed"}, "option.maxRows": {"7"}}
	saved := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/options", options)
	if saved.Code != 303 || saved.Header().Get("Location") != widgetEditURL(id) {
		t.Fatalf("save lost individual destination: %d", saved.Code)
	}
	options.Set("option.search", "retained draft")
	conflict := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/options", options)
	if conflict.Code != 409 || strings.Count(conflict.Body.String(), `name="option.search"`) != 1 || !strings.Contains(conflict.Body.String(), `value="retained draft"`) || !strings.Contains(conflict.Body.String(), `href="`+widgetEditURL(id)+`"`) {
		t.Fatal("conflict lost draft or recovery")
	}
	invalid := url.Values{"revision": {"3"}, "enabled": {"true"}, "option.maxRows": {"invalid"}, "option.search": {"numeric draft"}}
	if w := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/options", invalid); w.Code != 409 || !strings.Contains(w.Body.String(), `value="numeric draft"`) {
		t.Fatal("invalid options lost draft")
	}
	denied := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/delete", url.Values{"revision": {"3"}})
	if denied.Code != 400 || !strings.Contains(denied.Body.String(), "Confirm deletion") {
		t.Fatal("deletion confirmation bypassed")
	}
	f.mu.Lock()
	if f.writes != 1 || string(f.catalog.Placements[1].Options) != `{"search":"second"}` {
		t.Error("conflict or denial wrote records", f.writes)
	}
	f.mu.Unlock()
	selected := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/selection", url.Values{"revision": {"3"}, "enabled": {"false"}})
	if selected.Code != 303 || selected.Header().Get("Location") != widgetEditURL(id) {
		t.Fatal("selection lost individual destination")
	}
	deleted := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/delete", url.Values{"revision": {"4"}, "confirm": {"true"}})
	if deleted.Code != 303 || deleted.Header().Get("Location") != "/plugins/widgets" {
		t.Fatal("delete did not return to list")
	}
}
func TestWidgetListRetainsUnavailableAndDisabledRecords(t *testing.T) {
	h, f := widgetManagementHandler(t)
	f.mu.Lock()
	f.catalog.Definitions = nil
	f.mu.Unlock()
	list := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if list.Code != 200 || !strings.Contains(list.Body.String(), "Unavailable widget summary") || !strings.Contains(list.Body.String(), `id="plugin-selection-action-00000000-0000-4000-8000-000000000003"`) {
		t.Fatal("unavailable saved configuration disappeared")
	}
	edit := widgetRequest(h, "GET", "/plugin-widgets/00000000-0000-4000-8000-000000000003/edit", nil)
	if edit.Code != 200 || strings.Contains(edit.Body.String(), "Save options") || !strings.Contains(edit.Body.String(), "Delete widget") {
		t.Fatal("unavailable editor offered destructive option replacement")
	}
	f.mu.Lock()
	f.catalog.Placements = nil
	f.mu.Unlock()
	empty := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), "No widgets available") {
		t.Fatal("empty list lacks guidance")
	}
}

func TestWidgetCreationReplayAndDeletedRecovery(t *testing.T) {
	h, f := widgetManagementHandler(t)
	base := widgetManagementURL("example.fixture", "00000000-0000-4000-8000-000000000001") + "/summary"
	form := widgetRequest(h, "GET", base+"/new", nil)
	match := regexp.MustCompile(`name="placementId" value="([^"]+)"`).FindStringSubmatch(form.Body.String())
	if len(match) != 2 {
		t.Fatal("missing stable create identity")
	}
	id := match[1]
	values := url.Values{"placementId": {id}, "enabled": {"false"}, "option.search": {"new widget"}}
	for range 2 {
		w := widgetRequest(h, "POST", base, values)
		if w.Code != 303 || w.Header().Get("Location") != widgetEditURL(id) {
			t.Fatalf("create replay lost identity: %d", w.Code)
		}
	}
	f.mu.Lock()
	if f.writes != 1 || len(f.catalog.Placements) != 3 {
		t.Error("create replay duplicated placement")
	}
	f.mu.Unlock()
	for i := range 2 {
		w := widgetRequest(h, "POST", "/plugin-widgets/"+id+"/delete", url.Values{"revision": {"1"}, "confirm": {"true"}})
		expected := 303
		if i == 1 {
			expected = 404
		}
		if w.Code != expected {
			t.Fatalf("delete/replay status %d", w.Code)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writes != 2 || len(f.catalog.Placements) != 2 {
		t.Fatal("delete replay mutated another placement")
	}
}
