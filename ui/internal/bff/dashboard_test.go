package bff

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a-h/templ"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

type widgetCatalog struct {
	fakeCatalog
	mu            sync.Mutex
	snapshot      api.DashboardSnapshot
	err, mutation error
	calls         []bool
}

func (c *widgetCatalog) Dashboard(context.Context) (api.DashboardSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return api.DashboardSnapshot{Items: append([]api.DashboardWidget(nil), c.snapshot.Items...), Unavailable: append([]string(nil), c.snapshot.Unavailable...)}, c.err
}
func (c *widgetCatalog) SelectWidget(_ context.Context, kind, id string, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, enabled)
	if c.mutation != nil {
		return c.mutation
	}
	c.snapshot.Items = nil
	if enabled {
		c.snapshot.Items = []api.DashboardWidget{hostWidget(id, 0)}
	}
	return nil
}
func hostWidget(id string, cpu float64) api.DashboardWidget {
	return api.DashboardWidget{Id: "host-" + id, Kind: "host-metrics", Presentation: "card", SourceKind: "host", SourceId: id, Title: id, Data: map[string]interface{}{"cpuPercent": cpu, "kind": "docker", "status": "reporting"}}
}
func TestDashboardRenderers(t *testing.T) {
	items := []api.DashboardWidget{hostWidget("zero", 0), {Id: "host-gone", SourceKind: "host", SourceId: "gone", Kind: "host-metrics", Presentation: "card", Title: "Missing host", Error: "Host no longer reports telemetry."}, {Id: "other", Kind: "future", Presentation: "unknown", Title: "Future item"}, {Id: "note", Kind: "note", Presentation: "inline"}}
	renderers := widgetRenderers()
	renderers["note/inline"] = func(item api.DashboardWidget) (view.Widget, error) {
		return view.Widget{ID: item.Id, Content: templ.Raw(`<aside>Plain note</aside>`)}, nil
	}
	page := renderWidgets(api.DashboardSnapshot{Items: items}, renderers)
	var b bytes.Buffer
	if err := view.WidgetGrid(page.Widgets).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "Remove from home") || strings.Contains(b.String(), `type="checkbox"`) {
		t.Fatal("homepage includes selection control")
	}
	for _, want := range []string{"0.0%", "Unavailable", `href="/hosts/gone"`, "Host no longer reports telemetry.", "Future item", `<div id="widget-note" data-dashboard-widget><aside>Plain note</aside></div>`} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
func TestHostSelectionDesiredStateAndFailures(t *testing.T) {
	c := &widgetCatalog{}
	h, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	send := func(body string, partial bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/hosts/test/selection", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if partial {
			r.Header.Set("HX-Request-Type", "partial")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for range 2 {
		w := send("enabled=true&range=24h", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `type="checkbox"`) || !regexp.MustCompile(`<input[^>]*\schecked(?:\s|=|/?>)`).MatchString(w.Body.String()) {
			t.Fatalf("add %d %s", w.Code, w.Body.String())
		}
	}
	if len(c.snapshot.Items) != 1 || len(c.calls) != 2 || !c.calls[1] {
		t.Fatal("repeat add inverted desired state")
	}
	w := send("enabled=toggle", true)
	if w.Code != 400 || len(c.calls) != 2 {
		t.Fatal("malformed selection mutated state")
	}
	w = send("enabled=false&range=24h", false)
	if w.Code != 303 || w.Header().Get("Location") != "/hosts/test?range=24h" {
		t.Fatal("native context lost")
	}
	w = send("selection_form=host&range=24h", true)
	if w.Code != 200 || len(c.snapshot.Items) != 0 || c.calls[len(c.calls)-1] {
		t.Fatal("unchecked checkbox did not remove selection")
	}
	if regexp.MustCompile(`<input[^>]*\schecked(?:\s|=|/?>)`).MatchString(w.Body.String()) {
		t.Fatal("checkbox remained checked after removal")
	}
	w = send("range=24h", true)
	if w.Code != 400 {
		t.Fatal("missing form marker accepted")
	}
	c.mutation = errors.New("private secret")
	w = send("enabled=true&range=24h", true)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "Could not save selection") || strings.Contains(w.Body.String(), "private secret") {
		t.Fatal("mutation failure response")
	}
	w = send("enabled=true&range=24h", false)
	if w.Code != 503 || !strings.Contains(w.Body.String(), `href="/hosts/test?range=24h"`) {
		t.Fatal("native retry link missing")
	}
}
func TestDashboardStreamRecoveryAndCancellation(t *testing.T) {
	c := &widgetCatalog{snapshot: api.DashboardSnapshot{Items: []api.DashboardWidget{hostWidget("initial", 0)}}}
	s := &server{catalog: c, timeout: time.Second, widgets: widgetRenderers()}
	done := make(chan struct{}, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { done <- struct{}{} }()
		s.streamDashboard(w, r, 10*time.Millisecond)
	}))
	defer ts.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	frame := func() string {
		t.Helper()
		var b strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\n" {
				return b.String()
			}
			b.WriteString(line)
		}
	}
	until := func(want string) string {
		t.Helper()
		for {
			next := frame()
			if strings.Contains(next, want) {
				return next
			}
		}
	}
	until("0.0%")
	if frame() != ": keepalive\n" {
		t.Fatal("unchanged snapshot resent")
	}
	c.mu.Lock()
	c.snapshot.Items = nil
	c.snapshot.Unavailable = []string{"host"}
	c.mu.Unlock()
	if next := until("Some sources unavailable"); strings.Contains(next, `id="dashboard-grid"`) || strings.Contains(next, `id="widget-host-initial"`) {
		t.Fatal("failed provider rewrote retained widgets")
	}
	c.mu.Lock()
	c.err = errors.New("private failure")
	c.mu.Unlock()
	if next := until("Updates unavailable"); strings.Contains(next, "dashboard-grid") || strings.Contains(next, "private failure") {
		t.Fatal("transport failure erased grid or leaked error")
	}
	c.mu.Lock()
	c.err = nil
	c.snapshot = api.DashboardSnapshot{Items: []api.DashboardWidget{hostWidget("initial", 72.3)}}
	c.mu.Unlock()
	if next := until("72.3%"); strings.Contains(next, `id="dashboard-grid"`) || !strings.Contains(next, `id="widget-host-initial" hx-swap-oob="innerMorph"`) {
		t.Fatal("telemetry recovery replaced the grid instead of its changed widget")
	}
	c.mu.Lock()
	c.snapshot.Items = append(c.snapshot.Items, hostWidget("neighbour", 10))
	c.mu.Unlock()
	if next := until("neighbour"); !strings.Contains(next, `id="dashboard-grid"`) {
		t.Fatal("new selection did not reconcile sections")
	}
	c.mu.Lock()
	c.snapshot.Items = []api.DashboardWidget{hostWidget("initial", 80), hostWidget("neighbour", 10)}
	c.mu.Unlock()
	if next := until("80.0%"); strings.Contains(next, "neighbour") || strings.Contains(next, `id="dashboard-grid"`) {
		t.Fatal("telemetry update resent unchanged neighbour or grid")
	}
	c.mu.Lock()
	c.snapshot.Items = []api.DashboardWidget{hostWidget("initial", 72.3)}
	c.mu.Unlock()
	if next := until("72.3%"); !strings.Contains(next, `id="dashboard-grid"`) {
		t.Fatal("removed selection did not reconcile sections")
	}
	response.Body.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream leaked")
	}
	response, err = client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	reader = bufio.NewReader(response.Body)
	until("72.3%")
	response.Body.Close()
	<-done
}

func TestDashboardReconnectFailurePreservesExistingDOM(t *testing.T) {
	c := &widgetCatalog{snapshot: api.DashboardSnapshot{Unavailable: []string{"host"}}}
	s := &server{catalog: c, timeout: time.Second, widgets: widgetRenderers()}
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(done); s.streamDashboard(w, r, time.Second) }))
	defer ts.Close()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	<-done
	if strings.Contains(line, `id="dashboard-grid"`) || !strings.Contains(line, "Some sources unavailable") {
		t.Fatal("reconnect discarded existing provider data")
	}
}

func TestMissingPinnedHostStillExposesSelection(t *testing.T) {
	c := &widgetCatalog{snapshot: api.DashboardSnapshot{Items: []api.DashboardWidget{hostWidget("gone", 0)}}}
	h, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, h, http.MethodGet, "/hosts/gone", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Show on home") || !strings.Contains(w.Body.String(), "Host no longer available") {
		t.Fatal("missing selected host settings inaccessible")
	}
	if strings.Contains(w.Body.String(), "hx-sse:connect") {
		t.Fatal("missing host starts unusable stream")
	}
}

func TestWidgetRefreshIsScopedAndPreservesContentOnFailure(t *testing.T) {
	c := &widgetCatalog{snapshot: api.DashboardSnapshot{Items: []api.DashboardWidget{hostWidget("target", 73.4), hostWidget("neighbour", 9)}}}
	h, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	refresh := func(partial bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/dashboard/widgets/host/target", nil)
		if partial {
			r.Header.Set("HX-Request-Type", "partial")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := refresh(true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "73.4%") || strings.Contains(w.Body.String(), "neighbour") || strings.Contains(w.Body.String(), "hx-sse:connect") {
		t.Fatal("refresh did not isolate target widget")
	}
	c.err = errors.New("private upstream error")
	w = refresh(true)
	if w.Code != 503 || w.Header().Get("HX-Retarget") != "#widget-refresh-feedback-host-target" || strings.Contains(w.Body.String(), "private upstream") || strings.Contains(w.Body.String(), "article") {
		t.Fatal("failure must update only feedback")
	}
	c.err = nil
	c.snapshot.Unavailable = []string{"host"}
	if w = refresh(true); w.Code != 503 {
		t.Fatal("unavailable source accepted")
	}
	c.snapshot = api.DashboardSnapshot{}
	if w = refresh(true); w.Code != 404 {
		t.Fatal("unselected widget accepted")
	}
	if w = refresh(false); w.Code != 303 || w.Header().Get("Location") != "/" {
		t.Fatal("native fallback missing")
	}
	if len(c.calls) != 0 {
		t.Fatal("refresh changed selections")
	}
}

func TestDashboardSectionsKeepHostsFirstAcrossSnapshots(t *testing.T) {
	snapshot := api.DashboardSnapshot{Items: []api.DashboardWidget{
		{Id: "service-one", SourceKind: "service", Title: "Service"},
		{Id: "future-one", SourceKind: "future", Title: "Future"},
		hostWidget("first", 20), hostWidget("second", 30),
	}}
	page := renderWidgets(snapshot, widgetRenderers())
	for _, component := range []templ.Component{view.DashboardGrid(*page), view.DashboardLiveSnapshot(*page, "Connected")} {
		var b bytes.Buffer
		if err := component.Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		html := b.String()
		last := -1
		for _, id := range []string{"dashboard-section-hosts", "widget-host-first", "widget-host-second", "dashboard-section-services", "widget-service-one", "dashboard-section-other", "widget-future-one"} {
			position := strings.Index(html, `id="`+id+`"`)
			if position <= last {
				t.Fatalf("missing or out-of-order item %s", id)
			}
			last = position
		}
	}
	var b bytes.Buffer
	if err := view.DashboardGrid(view.Dashboard{}).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "dashboard-section-") || !strings.Contains(b.String(), "No pinned widgets") {
		t.Fatal("empty dashboard has empty sections or loses empty state")
	}
}
