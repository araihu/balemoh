package bff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	html "golang.org/x/net/html"
)

func pinRequest(h http.Handler, path string, values url.Values, partial bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if partial {
		r.Header.Set("HX-Request-Type", "partial")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func widgetCheckboxState(t *testing.T, body, id string) (checked, disabled bool) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if attrs["id"] == "plugin-selection-action-"+id {
				found = true
				_, checked = attrs["checked"]
				_, disabled = attrs["disabled"]
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if !found {
		t.Fatal("checkbox missing", id)
	}
	return
}
func TestWidgetCheckboxPreservesOptionsAndRejectsStaleRevision(t *testing.T) {
	h, f := widgetManagementHandler(t)
	const id = "00000000-0000-4000-8000-000000000002"
	path := "/plugin-widgets/" + id + "/selection"
	values := url.Values{"selection_form": {"plugin-widget"}, "selection_context": {"widgets"}, "previous": {"true"}, "revision": {"2"}}
	w := pinRequest(h, path, values, true)
	if w.Code != 200 || strings.Contains(w.Body.String(), "<html") || !strings.Contains(w.Body.String(), `id="plugin-selection-`+id+`"`) {
		t.Fatalf("unchecked partial: %d %s", w.Code, w.Body.String())
	}
	checked, disabled := widgetCheckboxState(t, w.Body.String(), id)
	if checked || disabled {
		t.Fatal("unpin did not confirm unchecked state")
	}
	f.mu.Lock()
	if f.writes != 1 || f.catalog.Placements[0].Enabled || string(f.catalog.Placements[0].Options) != `{"search":"first","maxRows":5}` || f.catalog.Placements[1].Enabled {
		t.Error("pin changed options or neighbor")
	}
	// Another editor changes the revision and options after this checkbox rendered.
	f.catalog.Placements[0].Revision = "4"
	f.catalog.Placements[0].Options = json.RawMessage(`{"search":"other editor","maxRows":9}`)
	f.mu.Unlock()
	values.Set("enabled", "true") // Still revision 2.
	stale := pinRequest(h, path, values, true)
	if stale.Code != 409 || !strings.Contains(stale.Body.String(), `name="revision" value="4"`) || strings.Contains(stale.Body.String(), "<table") {
		t.Fatalf("stale checkbox did not recover server truth: %d", stale.Code)
	}
	checked, _ = widgetCheckboxState(t, stale.Body.String(), id)
	if checked {
		t.Fatal("conflict displayed requested rather than current state")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writes != 1 || string(f.catalog.Placements[0].Options) != `{"search":"other editor","maxRows":9}` {
		t.Fatal("stale pin overwrote options")
	}
}
func TestUnconfiguredWidgetPinIsStableAcrossTabsAndReplays(t *testing.T) {
	h, f := widgetManagementHandler(t)
	f.mu.Lock()
	f.catalog.Placements = nil
	f.mu.Unlock()
	id := defaultWidgetID("example.fixture", "00000000-0000-4000-8000-000000000001", "summary")
	path := widgetManagementURL("example.fixture", "00000000-0000-4000-8000-000000000001") + "/summary/selection"
	list := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if strings.Count(list.Body.String(), "<table ") != 1 || strings.Count(list.Body.String(), `type="checkbox"`) != 1 || !strings.Contains(list.Body.String(), `id="plugin-selection-action-`+id+`"`) || strings.Contains(list.Body.String(), `data-table-row-link`) {
		t.Fatal("unconfigured widget not represented once with native checkbox/link")
	}
	noop := pinRequest(h, path, url.Values{"selection_form": {"plugin-widget"}, "selection_context": {"widgets"}}, true)
	if noop.Code != 200 {
		t.Fatal("unchecked unconfigured row failed")
	}
	f.mu.Lock()
	if f.writes != 0 {
		t.Error("unchecked unconfigured row wrote a placement")
	}
	f.mu.Unlock()
	values := url.Values{"selection_form": {"plugin-widget"}, "selection_context": {"widgets"}, "previous": {"false"}, "enabled": {"true"}}
	for range 2 {
		w := pinRequest(h, path, values, true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `href="`+widgetEditURL(id)+`"`) || !strings.Contains(w.Body.String(), `hx-swap-oob="innerHTML"`) {
			t.Fatalf("create/replay did not update individual link: %d", w.Code)
		}
		checked, _ := widgetCheckboxState(t, w.Body.String(), id)
		if !checked {
			t.Fatal("created widget not checked")
		}
	}
	f.mu.Lock()
	if f.writes != 1 || len(f.catalog.Placements) != 1 || !f.catalog.Placements[0].Enabled {
		t.Error("repeated pin duplicated record")
	}
	f.mu.Unlock()
	fresh := widgetRequest(h, "GET", "/plugins/widgets", nil)
	if strings.Count(fresh.Body.String(), `type="checkbox"`) != 1 || !strings.Contains(fresh.Body.String(), `href="`+widgetEditURL(id)+`"`) {
		t.Fatal("saved widget duplicated available type")
	}
	values.Del("enabled")
	values.Set("revision", "1")
	unpinned := pinRequest(h, "/plugin-widgets/"+id+"/selection", values, false)
	if unpinned.Code != 303 || unpinned.Header().Get("Location") != "/plugins/widgets" {
		t.Fatal("native checkbox lost list fallback")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.catalog.Placements) != 1 || f.catalog.Placements[0].Enabled || string(f.catalog.Placements[0].Options) != "{}" {
		t.Fatal("unpin deleted saved configuration")
	}
}
func TestWidgetCheckboxFailureAndStaleAvailableRow(t *testing.T) {
	h, f := widgetManagementHandler(t)
	const id = "00000000-0000-4000-8000-000000000002"
	values := url.Values{"selection_form": {"plugin-widget"}, "selection_context": {"widgets"}, "previous": {"true"}, "revision": {"2"}}
	f.mu.Lock()
	f.unavailable = true
	f.mu.Unlock()
	outage := pinRequest(h, "/plugin-widgets/"+id+"/selection", values, true)
	if outage.Code != 503 || strings.Contains(outage.Body.String(), "<html") || !strings.Contains(outage.Body.String(), "Refresh") {
		t.Fatalf("outage must retain scoped unconfirmed state: %d", outage.Code)
	}
	checked, disabled := widgetCheckboxState(t, outage.Body.String(), id)
	if !checked || !disabled {
		t.Fatal("outage must retain confirmed value and disable unknown state")
	}
	f.mu.Lock()
	f.unavailable = false
	f.mu.Unlock()
	missing := pinRequest(h, "/plugin-widgets/"+id+"/selection", url.Values{"selection_form": {"plugin-widget"}, "enabled": {"false"}}, true)
	if missing.Code != 428 {
		t.Fatal("saved revision bypassed")
	}
	path := widgetManagementURL("example.fixture", "00000000-0000-4000-8000-000000000001") + "/summary/selection"
	changed := pinRequest(h, path, url.Values{"selection_form": {"plugin-widget"}, "enabled": {"true"}}, true)
	if changed.Code != 409 || !strings.Contains(changed.Body.String(), "Widget list changed") {
		t.Fatal("stale available row created duplicate after editor saved a sibling")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writes != 0 || len(f.catalog.Placements) != 2 {
		t.Fatal("denied pins mutated records")
	}
}
func TestWidgetDefaultOptionsUseDeclaredDefaults(t *testing.T) {
	raw, err := defaultWidgetOptions([]byte(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":20,"default":5},"search":{"type":"string","maxLength":100}},"additionalProperties":false}`))
	if err != nil || string(raw) != `{"limit":5}` {
		t.Fatalf("defaults %s: %v", raw, err)
	}
}

func TestWidgetEditorCheckboxSynchronizesOnlyItsWriteMetadata(t *testing.T) {
	h, _ := widgetManagementHandler(t)
	const id = "00000000-0000-4000-8000-000000000002"
	values := url.Values{"selection_form": {"plugin-widget"}, "selection_context": {"edit"}, "revision": {"2"}, "previous": {"true"}}
	pinned := pinRequest(h, "/plugin-widgets/"+id+"/selection", values, true)
	if pinned.Code != 200 || !strings.Contains(pinned.Body.String(), `id="plugin-options-revision-`+id+`"`) || !strings.Contains(pinned.Body.String(), `value="3"`) || !strings.Contains(pinned.Body.String(), `id="plugin-options-enabled-`+id+`"`) || strings.Contains(pinned.Body.String(), `name="option.search"`) {
		t.Fatal("editor pin must sync write metadata without replacing drafts")
	}
	// A stale idempotent pin can confirm selection, but cannot bless stale option drafts.
	stale := pinRequest(h, "/plugin-widgets/"+id+"/selection", values, true)
	if stale.Code != 200 || strings.Contains(stale.Body.String(), `id="plugin-options-revision-`) {
		t.Fatal("stale options revision was advanced")
	}
}
