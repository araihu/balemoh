package view

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
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
