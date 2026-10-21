package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestInteractiveHostHistoryPreservesMissingAndZero(t *testing.T) {
	zero, memory := 0.0, 42.5
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	page := HostDetail{Samples: []HostSample{{At: at, CPU: &zero}, {At: at.Add(time.Minute), Memory: &memory}, {At: at.Add(2 * time.Minute), CPU: &zero, Memory: &memory}}}
	for _, cpu := range []bool{true, false} {
		cfg := historyChart(page, cpu)
		for i, sample := range page.Samples {
			value := sample.Memory
			if cpu {
				value = sample.CPU
			}
			point := cfg.Series[0].Data[i]
			if point.Missing != (value == nil) || value != nil && point.Value != *value {
				t.Fatalf("cpu=%t point %d lost missing/zero distinction", cpu, i)
			}
		}
		var output bytes.Buffer
		if err := HostChartContent(page, cpu).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		for _, want := range []string{`goshtoso-charts-interactive`, `"value":null`, `"trigger":"axis"`} {
			if !strings.Contains(body, want) {
				t.Errorf("missing interactive output %s", want)
			}
		}
		if cpu && !strings.Contains(body, `"value":0`) {
			t.Error("zero reading missing")
		}
	}
}

func TestHostTabsKeepLiveUpdatesScoped(t *testing.T) {
	for _, kind := range []string{"Kubernetes cluster", "Kubernetes node", "Docker host"} {
		page := HostDetail{Host: Host{ID: "host", Name: "Host", Kind: kind, Architecture: "arm64"}, Window: "1h"}
		if kind == "Kubernetes node" {
			page.Host.ParentID = "cluster"
		}
		var output bytes.Buffer
		if err := HostContent(page).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		for _, tab := range []string{"overview", "workloads"} {
			if !strings.Contains(body, `aria-controls="tabpanelhost-tabs`+tab+`"`) {
				t.Errorf("%s missing %s tab", kind, tab)
			}
		}
		if strings.Contains(body, `id="host-nodes"`) != (kind == "Kubernetes cluster") {
			t.Errorf("wrong nodes panel for %s", kind)
		}
		if strings.Contains(body, "Applications discovered on this source") != (kind != "Kubernetes node") {
			t.Errorf("wrong services panel for %s", kind)
		}
		output.Reset()
		if err := HostLiveSnapshot(page).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body = output.String()
		if strings.Contains(body, `role="tab"`) || strings.Contains(body, `id="host-overview"`) {
			t.Fatal("SSE replaces tab state or overview container")
		}
	}
}
