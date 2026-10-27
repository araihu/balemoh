package view

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/araihu/goshtoso/components/badge"
)

func TestApplicationHealthDoesNotInventReadiness(t *testing.T) {
	for _, tc := range []struct {
		name      string
		workload  HostWorkload
		attention bool
		summary   string
		tone      badge.Tone
	}{
		{"older agent", HostWorkload{State: "Running", Restarts: 100}, false, "readiness unknown", badge.ToneDefault},
		{"ready", HostWorkload{State: "Running", Ready: new(true), Restarts: 100}, false, "1/1 workloads ready", badge.ToneSuccess},
		{"crash loop", HostWorkload{State: "Running", Ready: new(false), Reason: "CrashLoopBackOff"}, true, "0/1 workloads ready", badge.ToneWarning},
		{"pending", HostWorkload{State: "Pending"}, true, "readiness unknown", badge.ToneWarning},
		{"completed", HostWorkload{State: "Succeeded", Ready: new(false)}, false, "1 completed", badge.ToneSuccess},
		{"docker", HostWorkload{State: "running", Docker: true}, false, "1/1 containers running", badge.ToneSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := HostApplication{Workloads: []HostWorkload{tc.workload}}
			if app.Attention() != tc.attention || !strings.Contains(app.Summary(), tc.summary) || app.Tone() != tc.tone {
				t.Fatalf("incorrect state: %s, %s, attention=%t", app.Summary(), app.Tone(), app.Attention())
			}
		})
	}
}

func TestApplicationStreamUpdatesRowsWithoutReplacingFilters(t *testing.T) {
	previous := HostDetail{Applications: []HostApplication{{Service: Service{ID: "app", DisplayName: "Jellyfin"}, Workloads: []HostWorkload{{Name: "pod", State: "Running", Ready: new(true)}}}}}
	current := previous
	current.Applications = append([]HostApplication(nil), previous.Applications...)
	current.Applications[0].Workloads = []HostWorkload{{Name: "pod", State: "Pending", Ready: new(false)}}
	var output bytes.Buffer
	if err := HostInventorySnapshot(current, &previous).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	if !strings.Contains(body, `hx-swap-oob="outerMorph"`) || strings.Contains(body, `id="host-application-results"`) || strings.Contains(body, `data-application-search`) || strings.Contains(body, `role="tab"`) {
		t.Fatal("value update replaced collection or controls")
	}
	output.Reset()
	current.Applications = append(current.Applications, HostApplication{Service: Service{ID: "new", DisplayName: "New"}})
	if err := HostInventorySnapshot(current, &previous).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `id="host-application-results" hx-swap-oob="innerMorph"`) {
		t.Fatal("membership change did not reconcile collection")
	}
}

func TestMixedCompletedWorkloadsDoNotReduceActiveReadiness(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		activeReady, completedReady *bool
		summary                     string
		attention                   bool
		tone                        badge.Tone
	}{
		{"ready and completed", new(true), new(false), "1/1 active workloads ready · 1 completed", false, badge.ToneSuccess},
		{"completed readiness absent", new(true), nil, "1/1 active workloads ready · 1 completed", false, badge.ToneSuccess},
		{"active not ready", new(false), new(false), "0/1 active workloads ready · 1 completed", true, badge.ToneWarning},
		{"active readiness unknown", nil, new(false), "1 active · readiness unknown · 1 completed", false, badge.ToneDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := HostApplication{Workloads: []HostWorkload{{State: "Running", Ready: tc.activeReady}, {State: "Succeeded", Ready: tc.completedReady}}}
			if app.Summary() != tc.summary || app.Attention() != tc.attention || app.Tone() != tc.tone {
				t.Fatalf("misleading mixed state: %q, attention=%t, tone=%s", app.Summary(), app.Attention(), app.Tone())
			}
		})
	}
}

func TestApplicationHistoryAndRuntimeEvidenceAreReadOnly(t *testing.T) {
	app := HostApplication{Service: Service{ID: "missing", DisplayName: "<script>bad</script>"}, Missing: true}
	var output bytes.Buffer
	if err := HostApplicationRow(app, "Kubernetes cluster", false).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, want := range []string{`data-scope="missing"`, `hidden`, `Historical catalog entry`, `&lt;script&gt;bad&lt;/script&gt;`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, "hx-post") || strings.Contains(body, "/edit") || strings.Contains(body, "<script>bad") {
		t.Fatal("history includes mutations or unsafe content")
	}
}

func TestMobileWorkloadEvidencePreservesRuntimeFields(t *testing.T) {
	for _, tc := range []struct {
		name, kind   string
		workload     HostWorkload
		want, absent []string
	}{
		{name: "pod failure", kind: "Kubernetes cluster", workload: HostWorkload{Name: "pod<1>", Namespace: "media", State: "Running", Ready: new(false), Node: "raspi", Reason: "CrashLoopBackOff", Restarts: 42, Images: []string{"registry.example/image:tag&build"}}, want: []string{"pod&lt;1&gt;", "media", "Running", "Not ready", "raspi", "CrashLoopBackOff", "42", "registry.example/image:tag&amp;build"}, absent: []string{"pod<1>"}},
		{name: "older agent", kind: "Kubernetes node", workload: HostWorkload{Name: "old-pod", State: "Running"}, want: []string{"old-pod", "Unknown", "Not reported", "Total restarts"}},
		{name: "stopped docker", kind: "Docker host", workload: HostWorkload{Name: "container", State: "exited", Docker: true}, want: []string{"container", "exited", "Not reported"}, absent: []string{"Readiness", "Total restarts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			app := HostApplication{Service: Service{DisplayName: "Example"}, Workloads: []HostWorkload{tc.workload}}
			if err := HostApplicationWorkloadsMobile(app, tc.kind).Render(context.Background(), &output); err != nil {
				t.Fatal(err)
			}
			body := output.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("mobile evidence lost %q", want)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(body, absent) {
					t.Errorf("mobile evidence contains %q", absent)
				}
			}
		})
	}
}
