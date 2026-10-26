package bff

import (
	"context"
	"errors"
	api "github.com/araihu/balemoh/client"
	chartassets "github.com/araihu/goshtoso-charts/assets"
	"golang.org/x/net/html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (f *fakeCatalog) Hosts(context.Context) ([]api.Host, error) { return []api.Host{}, nil }
func (f *fakeCatalog) Host(context.Context, string, string) (api.HostDetail, error) {
	return api.HostDetail{}, &upstreamError{operation: "host", status: 404}
}

type hostCatalog struct {
	fakeCatalog
	hosts  []api.Host
	detail api.HostDetail
	err    error
}

func (f *hostCatalog) Hosts(context.Context) ([]api.Host, error) { return f.hosts, f.err }
func (f *hostCatalog) Host(context.Context, string, string) (api.HostDetail, error) {
	return f.detail, f.err
}
func TestHostsIndexLinksAndStates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		catalog *hostCatalog
		status  int
		want    string
	}{
		{"empty", &hostCatalog{}, 200, "No hosts discovered"},
		{"error", &hostCatalog{err: errors.New("private detail")}, 503, "Unable to load hosts"},
		{"host", &hostCatalog{hosts: []api.Host{{Id: "docker-id", Name: "raspi", Kind: "docker", Status: "stale"}}}, 200, `href="/hosts/docker-id"`},
		{"escape", &hostCatalog{hosts: []api.Host{{Id: "id", Name: "<script>bad</script>", Kind: "unknown"}}}, 200, "&lt;script&gt;bad&lt;/script&gt;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := New(tc.catalog, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/hosts", nil))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.want) || strings.Contains(w.Body.String(), "private detail") {
				t.Fatalf("response: %d", w.Code)
			}
		})
	}
}
func TestHostDetailRangesMetadataAndMissingData(t *testing.T) {
	now := time.Now().UTC()
	cpu := 0.0
	catalog := &hostCatalog{detail: api.HostDetail{Host: api.Host{Id: "host", Name: "raspi", Kind: "docker", Status: "stale", CpuPercent: &cpu, ObservedAt: &now}, Range: "1h", Samples: []api.HostSample{{At: now.Add(-time.Minute), CpuPercent: &cpu}, {At: now}}}}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/hosts/host?range=24h", nil))
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatalf("render failed: %d", w.Code)
	}
	for _, want := range []string{"raspi", "0.0%", "Unavailable", "Reports are more than", "goshtoso-charts-interactive", `href="https://balemoh.decastro.me/hosts/host"`, `aria-current="true"`, "Workload inventory is unavailable"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, tc := range []struct {
		path   string
		err    error
		status int
	}{{"/hosts/host?range=week", nil, 400}, {"/hosts/missing", &upstreamError{status: 404}, 404}, {"/hosts/host", errors.New("private upstream detail"), 503}} {
		catalog.err = tc.err
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "private upstream detail") {
			t.Errorf("%s = %d", tc.path, w.Code)
		}
	}
}

func TestHostsNavigationAndServerRenderedMetadata(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/", "/hosts"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route, nil))
		doc, err := html.Parse(strings.NewReader(w.Body.String()))
		if err != nil {
			t.Fatal(err)
		}
		values := map[string][]string{}
		hostsLink := false
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			switch n.Data {
			case "title":
				if n.FirstChild != nil {
					values["title"] = append(values["title"], n.FirstChild.Data)
				}
			case "meta":
				key := attrs["name"]
				if key == "" {
					key = attrs["property"]
				}
				values[key] = append(values[key], attrs["content"])
			case "link":
				if attrs["rel"] == "canonical" {
					values["canonical"] = append(values["canonical"], attrs["href"])
				}
			case "a":
				if attrs["href"] == "/hosts" {
					hostsLink = true
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		if !hostsLink {
			t.Errorf("%s: missing Hosts navigation", route)
		}
		for _, key := range []string{"title", "description", "canonical", "og:url", "og:type", "og:title", "og:description", "og:site_name", "og:image", "og:image:type", "og:image:width", "og:image:height", "og:image:alt", "twitter:card", "twitter:title", "twitter:description", "twitter:image", "twitter:image:alt"} {
			if len(values[key]) != 1 || values[key][0] == "" {
				t.Errorf("%s: metadata %s = %v", route, key, values[key])
			}
		}
		for key, want := range map[string]string{"canonical": "https://balemoh.decastro.me" + route, "og:url": "https://balemoh.decastro.me" + route, "og:image": "https://balemoh.decastro.me/ui/social-v2.png", "twitter:card": "summary_large_image"} {
			if len(values[key]) != 1 || values[key][0] != want {
				t.Errorf("%s: metadata %s = %v, want %s", route, key, values[key], want)
			}
		}
		if route == "/hosts" && (values["title"][0] != "Hosts · Balemoh" || !strings.Contains(values["description"][0], "Kubernetes clusters and Docker hosts")) {
			t.Error("Hosts metadata is not route-specific")
		}
	}
}

func TestHostChartAssetsAreServed(t *testing.T) {
	now := time.Now().UTC()
	value := 10.0
	catalog := &hostCatalog{detail: api.HostDetail{Host: api.Host{Id: "host", Name: "host", Kind: "docker"}, Samples: []api.HostSample{{At: now, CpuPercent: &value}}}}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/hosts/host", nil))
	doc, err := html.Parse(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	runtimeIndex, chartIndex := strings.Index(body, chartassets.RuntimeURL), strings.Index(body, "goshtoso-charts-interactive")
	if runtimeIndex < 0 || chartIndex < runtimeIndex {
		t.Fatal("interactive runtime must load before chart initialization")
	}
	found := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Data == "script" {
			for _, a := range n.Attr {
				if a.Key == "src" && strings.HasPrefix(a.Val, "/charts/assets/") {
					found = true
					r := httptest.NewRecorder()
					handler.ServeHTTP(r, httptest.NewRequest("GET", a.Val, nil))
					if r.Code != 200 || !strings.Contains(r.Header().Get("Content-Type"), "javascript") {
						t.Errorf("chart asset %s = %d", a.Val, r.Code)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if !found {
		t.Fatal("chart control script missing")
	}
}

func TestHostPartialNavigation(t *testing.T) {
	catalog := &hostCatalog{detail: api.HostDetail{Host: api.Host{Id: "host", Name: "raspi", Kind: "docker"}}}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"partial", "full"} {
		r := httptest.NewRequest("GET", "/hosts/host?range=24h", nil)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Request-Type", kind)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, `/hosts/host/events?range=24h`) {
			t.Fatal("wrong range snapshot")
		}
		if strings.Count(body, `hx-target="#host-overview"`) != 3 {
			t.Fatal("all history actions must use partial navigation")
		}
		if kind == "partial" && (strings.Contains(body, `role="tab"`) || strings.Contains(body, `id="host-applications"`) || strings.Contains(body, `id="host-selection-action-host"`)) {
			t.Fatal("history filter replaced unrelated host content")
		}
		if strings.Contains(body, chartassets.RuntimeURL) != (kind == "full") {
			t.Fatalf("%s: chart runtime should load only with full document", kind)
		}
		if strings.Contains(body, "<html") != (kind == "full") {
			t.Fatalf("%s: wrong document boundary", kind)
		}
		for _, want := range []string{`hx-target="#host-overview"`, `hx-swap="innerHTML"`, `hx-sync="#host-overview:replace"`, `hx-push-url="true"`} {
			if !strings.Contains(body, want) {
				t.Errorf("missing %s", want)
			}
		}
	}
	for _, tc := range []struct {
		path string
		err  error
		code int
	}{
		{"/hosts/host?range=week", nil, 400},
		{"/hosts/host", &upstreamError{status: 404}, 404},
		{"/hosts/host", errors.New("private upstream detail"), 503},
	} {
		catalog.err = tc.err
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("HX-Request-Type", "partial")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != tc.code || w.Header().Get("HX-Push-Url") != "false" || !strings.Contains(body, `id="host-history-error"`) || strings.Contains(body, "<html") || strings.Contains(body, "private upstream detail") {
			t.Fatalf("invalid recovery: %d %s", w.Code, body)
		}
	}
}

type selectableHostCatalog struct {
	widgetCatalog
	dashboardCalls int
}

func (c *selectableHostCatalog) Hosts(context.Context) ([]api.Host, error) {
	return []api.Host{{Id: "selected", Name: "Pinned host", Kind: "docker"}, {Id: "other", Name: "Other host", Kind: "docker"}}, nil
}
func (c *selectableHostCatalog) Dashboard(ctx context.Context) (api.DashboardSnapshot, error) {
	c.dashboardCalls++
	return c.widgetCatalog.Dashboard(ctx)
}
func TestHostsIndexSelectionSnapshot(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		c := &selectableHostCatalog{}
		c.snapshot.Items = []api.DashboardWidget{hostWidget("selected", 0)}
		if unavailable {
			c.snapshot.Unavailable = []string{"host"}
		}
		h, err := New(c, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/hosts", nil))
		if w.Code != 200 || c.dashboardCalls != 1 {
			t.Fatalf("status=%d dashboard calls=%d", w.Code, c.dashboardCalls)
		}
		doc, err := html.Parse(strings.NewReader(w.Body.String()))
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "input" && attrs["type"] == "checkbox" {
				found++
				_, checked := attrs["checked"]
				_, disabled := attrs["disabled"]
				if disabled != unavailable || checked != (!unavailable && attrs["id"] == "host-selection-action-selected") {
					t.Errorf("wrong selection: %v", attrs)
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
		if found != 2 {
			t.Fatalf("checkboxes=%d", found)
		}
	}
}
func TestHostsListSelectionResponseAndRedirect(t *testing.T) {
	c := &widgetCatalog{}
	h, err := New(c, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, partial := range []bool{true, false} {
		r := httptest.NewRequest("POST", "/hosts/test/selection", strings.NewReader("selection_form=host&selection_context=hosts&enabled=true"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if partial {
			r.Header.Set("HX-Request-Type", "partial")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if partial {
			if w.Code != 200 || !strings.Contains(w.Body.String(), `name="selection_context" value="hosts"`) || strings.Contains(w.Body.String(), `>Show on home<`) {
				t.Fatalf("bad list fragment: %d %s", w.Code, w.Body.String())
			}
		} else if w.Code != 303 || w.Header().Get("Location") != "/hosts" {
			t.Fatalf("bad redirect: %d %s", w.Code, w.Header().Get("Location"))
		}
	}
}
