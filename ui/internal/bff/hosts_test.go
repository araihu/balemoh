package bff

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
	"golang.org/x/net/html"
)

func TestHostsListsDistinctDiscoverySources(t *testing.T) {
	catalog := &fakeCatalog{staging: []api.ServiceCandidate{
		{Source: api.SourceRef{Kind: "container", Id: "raspi"}},
		{Source: api.SourceRef{Kind: "container", Id: "bastion"}},
		{Source: api.SourceRef{Kind: "container", Id: "raspi"}, Pinned: true},
		{Source: api.SourceRef{Kind: "kubernetes", Id: "devspace-local"}, Status: "hidden"},
		{Source: api.SourceRef{Kind: "kubernetes", Id: "devspace-local"}},
		{Source: api.SourceRef{Kind: "other", Id: "unsupported-source"}},
	}}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hosts", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	for _, name := range []string{"raspi", "bastion", "devspace-local", "unsupported-source"} {
		if strings.Count(body, ">"+name+"<") != 1 {
			t.Errorf("expected one row for %s", name)
		}
	}
	if !strings.Contains(body, "Unknown host") || !strings.Contains(body, "Kubernetes cluster") || !strings.Contains(body, "Docker host") {
		t.Error("incorrect host types")
	}
	for symbol, count := range map[string]int{"kubernetes-kubernetes": 1, "docker-docker": 2, "heroicons-server": 1} {
		if strings.Count(body, "/ui/icons/sprite.svg#"+symbol+"\"") != count {
			t.Errorf("expected %d host avatars using %s", count, symbol)
		}
	}
	if strings.Index(body, ">devspace-local<") > strings.Index(body, ">bastion<") || strings.Index(body, ">bastion<") > strings.Index(body, ">raspi<") {
		t.Error("expected clusters first, then hosts sorted by name")
	}
	if catalog.syncCalls != 0 || len(catalog.pinnedIDs) != 0 || len(catalog.unpinnedIDs) != 0 {
		t.Fatal("listing hosts mutated the catalog")
	}
}

func TestHostsEmptyErrorAndSourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		catalog *fakeCatalog
		status  int
		want    string
		absent  string
	}{
		{"empty", &fakeCatalog{}, 200, "No hosts discovered", "hosts-table"},
		{"unavailable", &fakeCatalog{stagingErr: errors.New("private upstream detail")}, 503, "Unable to load hosts", "No hosts discovered"},
		{"escaped", &fakeCatalog{staging: []api.ServiceCandidate{{Source: api.SourceRef{Kind: "container", Id: "<script>bad</script>"}}}}, 200, "&lt;script&gt;bad&lt;/script&gt;", "<script>bad</script>"},
		{"same name across kinds", &fakeCatalog{staging: []api.ServiceCandidate{
			{Source: api.SourceRef{Kind: "container", Id: "shared"}},
			{Source: api.SourceRef{Kind: "kubernetes", Id: "shared"}},
		}}, 200, "Kubernetes cluster", "No hosts discovered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := New(tc.catalog, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hosts", nil))
			body := w.Body.String()
			if w.Code != tc.status || !strings.Contains(body, tc.want) || strings.Contains(body, tc.absent) || strings.Contains(body, "private upstream detail") {
				t.Fatalf("unexpected host response: status %d", w.Code)
			}
			if tc.name == "same name across kinds" && strings.Count(body, ">shared<") != 2 {
				t.Error("source kind must be part of host identity")
			}
		})
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
		for key, want := range map[string]string{"canonical": "https://balemoh.decastro.me" + route, "og:url": "https://balemoh.decastro.me" + route, "og:image": "https://balemoh.decastro.me/ui/social-v1.png", "twitter:card": "summary_large_image"} {
			if len(values[key]) != 1 || values[key][0] != want {
				t.Errorf("%s: metadata %s = %v, want %s", route, key, values[key], want)
			}
		}
		if route == "/hosts" && (values["title"][0] != "Hosts · Balemoh" || !strings.Contains(values["description"][0], "Kubernetes clusters and Docker hosts")) {
			t.Error("Hosts metadata is not route-specific")
		}
	}
}
