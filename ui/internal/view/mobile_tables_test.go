package view

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

func TestMobileTablesReuseCellsAndPreserveHeaderAssociations(t *testing.T) {
	service := Service{ID: "app", DisplayName: "Application", Status: "live", Endpoints: []Endpoint{{URL: "https://app.example/"}}}
	var full, fragment bytes.Buffer
	if err := ServiceTable(PageData{Status: "live", Staging: true, Services: []Service{service}}).Render(context.Background(), &full); err != nil {
		t.Fatal(err)
	}
	if err := StagingRow(service).Render(context.Background(), &fragment); err != nil {
		t.Fatal(err)
	}
	for _, match := range regexp.MustCompile(`headers="([^"]+)"`).FindAllStringSubmatch(fragment.String(), -1) {
		if !strings.Contains(full.String(), `id="`+match[1]+`"`) {
			t.Errorf("updated row references absent header %q", match[1])
		}
	}
	if strings.Count(full.String(), `id="select-app"`) != 1 || strings.Count(fragment.String(), `id="select-app"`) != 1 {
		t.Fatal("pinning control must be shared by desktop and mobile")
	}
	if !strings.Contains(full.String(), `onchange="this.form.requestSubmit()"`) {
		t.Fatal("the templ upgrade must retain the trusted pinning handler")
	}
}

func TestMobileTableConfigurationCoversProductLists(t *testing.T) {
	for name, component := range map[string]templ.Component{
		"hosts":        HostTable([]Host{{ID: "host", Name: "Host", Architecture: "arm64"}}, &HostsPage{Selected: map[string]bool{}}),
		"widgets":      PluginWidgetsIndex(PluginWidgetList{Rows: []PluginWidgetRow{{ID: "widget01", Title: "Widget", Plugin: "Example", Connection: "Lab"}}}),
		"attributions": AttributionsContent(),
		"endpoints":    ResourceData(Service{Endpoints: []Endpoint{{URL: "https://example.internal/"}}}, 0),
		"workloads":    HostApplicationWorkloads(HostApplication{Service: Service{ID: "app", DisplayName: "Application"}, Workloads: []HostWorkload{{Name: "pod", Namespace: "apps", State: "Running", Node: "node"}}}, "Kubernetes cluster"),
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := component.Render(context.Background(), &output); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "goshtoso-table-responsive") {
				t.Fatal("list does not use the library's responsive layout")
			}
			if name == "workloads" && strings.Count(output.String(), `>pod</span>`) != 1 {
				t.Fatal("workload evidence must have one shared representation")
			}
		})
	}
}

func TestServiceNameOwnsNavigation(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		var output bytes.Buffer
		service := Service{ID: "app", DisplayName: "Application", Status: "live"}
		if err := ServiceTable(PageData{Status: "live", Staging: true, ReadOnly: readonly, Services: []Service{service}}).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		document, err := html.Parse(strings.NewReader(output.String()))
		if err != nil {
			t.Fatal(err)
		}
		links := 0
		var visit func(*html.Node)
		visit = func(node *html.Node) {
			for _, attr := range node.Attr {
				if node.Data == "a" && attr.Key == "href" && attr.Val == EditURL(service.ID) {
					links++
					var name string
					var text func(*html.Node)
					text = func(child *html.Node) {
						if child.Type == html.TextNode {
							name += child.Data
						}
						for next := child.FirstChild; next != nil; next = next.NextSibling {
							text(next)
						}
					}
					text(node)
					if strings.TrimSpace(name) != service.DisplayName {
						t.Errorf("edit link must name the service, got %q", name)
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(document)
		want := 1
		if readonly {
			want = 0
		}
		if links != want {
			t.Errorf("readonly=%t: got %d edit links, want %d", readonly, links, want)
		}
	}
}
