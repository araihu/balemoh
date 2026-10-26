package bff

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
)

func TestHostApplicationsUseExactResourceEvidence(t *testing.T) {
	source := api.SourceRef{Kind: "kubernetes", Id: "cluster"}
	pod := func(namespace, name string) api.ResourceRef {
		return api.ResourceRef{Kind: "pod", Namespace: &namespace, Name: name}
	}
	service := api.ServiceCandidate{Id: "jellyfin", Source: source, DisplayName: "Jellyfin", Resource: api.ResourceRef{Kind: "service", Namespace: new("media"), Name: "jellyfin"}, Resources: &[]api.ResourceObservation{{Resource: pod("media", "jellyfin-abc")}}}
	missing := api.ServiceCandidate{Id: "old-app", Source: source, DisplayName: "Old application", Missing: true, Status: "missing", Resource: api.ResourceRef{Kind: "service", Name: "old"}}
	oldPod := api.ServiceCandidate{Id: "old-pod", Source: source, Resource: pod("media", "jellyfin-old"), Missing: true, Status: "missing"}
	foreign := service
	foreign.Id, foreign.Source.Id = "foreign", "another-cluster"
	detail := api.HostDetail{Host: api.Host{Source: source, Kind: "cluster", Workloads: []api.HostWorkload{
		{Name: "jellyfin-abc", Namespace: "media", State: "Running", Ready: new(true), Node: new("raspi")},
		{Name: "jellyfin-abc", Namespace: "other", State: "Running"},
		{Name: "jellyfin-similar-name", Namespace: "media", State: "Pending"},
	}}}
	apps := hostApplications(detail, []api.ServiceCandidate{service, missing, oldPod, foreign})
	if len(apps) != 3 || apps[0].Service.ID != "jellyfin" || len(apps[0].Workloads) != 1 || apps[0].Workloads[0].Node != "raspi" || !apps[1].Missing || !apps[2].Unassociated || len(apps[2].Workloads) != 2 {
		t.Fatalf("incorrect associations: %+v", apps)
	}
	// Node views show only applications with workloads actually reported there.
	detail.Host.ParentId, detail.Host.Name = "cluster-host", "raspi"
	detail.Host.Workloads = detail.Host.Workloads[:1]
	apps = hostApplications(detail, []api.ServiceCandidate{service, missing})
	if len(apps) != 1 || apps[0].Service.ID != "jellyfin" {
		t.Fatalf("node contains unrelated applications: %+v", apps)
	}
}

func TestHostApplicationsComposeAndStoppedContainers(t *testing.T) {
	source := api.SourceRef{Kind: "container", Id: "raspi"}
	metadata := map[string]string{"compose.project": "media", "compose.service": "jellyfin"}
	compose := api.ServiceCandidate{Id: "compose", DisplayName: "Jellyfin", Source: source, Resource: api.ResourceRef{Kind: "compose-service", Namespace: new("media"), Name: "jellyfin"}, Metadata: metadata, Missing: true}
	container := api.ServiceCandidate{Id: "container", Source: source, Resource: api.ResourceRef{Kind: "container", Name: "media-jellyfin-1"}, Metadata: metadata, Missing: true}
	detail := api.HostDetail{Host: api.Host{Source: source, Kind: "docker", Name: "raspi", Workloads: []api.HostWorkload{{Name: "media-jellyfin-1", State: "exited"}}}}
	apps := hostApplications(detail, []api.ServiceCandidate{compose, container})
	if len(apps) != 1 || apps[0].Service.ID != "compose" || apps[0].Missing || len(apps[0].Workloads) != 1 || !apps[0].Attention() || !apps[0].Workloads[0].Docker {
		t.Fatalf("stopped Compose application lost or duplicated: %+v", apps)
	}
}

func TestMissingKubernetesEvidenceDoesNotOwnCurrentPods(t *testing.T) {
	for _, node := range []bool{false, true} {
		for _, missingStatus := range []bool{false, true} {
			t.Run(fmt.Sprintf("node=%t/statusOnly=%t", node, missingStatus), func(t *testing.T) {
				source := api.SourceRef{Kind: "kubernetes", Id: "cluster"}
				pod := api.ResourceRef{Kind: "pod", Namespace: new("media"), Name: "app-1"}
				historical := api.ServiceCandidate{Id: "historical", DisplayName: "Former app", Source: source, Missing: !missingStatus, Resource: api.ResourceRef{Kind: "service", Namespace: new("media"), Name: "former-app"}, Resources: &[]api.ResourceObservation{{Resource: pod}}}
				if missingStatus {
					historical.Status = "missing"
				}
				detail := api.HostDetail{Host: api.Host{Source: source, Kind: "cluster", Workloads: []api.HostWorkload{{Name: "app-1", Namespace: "media", State: "Running", Ready: new(true)}}}}
				if node {
					detail.Host.Kind, detail.Host.ParentId, detail.Host.Name = "node", "cluster-host", "raspi"
				}
				apps := hostApplications(detail, []api.ServiceCandidate{historical})
				historyFound, unassociatedFound := false, false
				for _, app := range apps {
					if app.Service.ID == historical.Id {
						historyFound = true
						if !app.Missing || app.Scope() != "missing" || len(app.Workloads) != 0 {
							t.Fatal("historical Kubernetes references claimed a current pod")
						}
					}
					if app.Unassociated && len(app.Workloads) == 1 && app.Workloads[0].Name == "app-1" {
						unassociatedFound = true
					}
				}
				if historyFound == node || !unassociatedFound {
					t.Fatalf("incorrect history or node placement: %+v", apps)
				}
				// A current group may own the pod without historical evidence
				// joining it or consuming its inventory membership.
				current := historical
				current.Id, current.DisplayName, current.Missing, current.Status = "current", "Current app", false, "live"
				apps = hostApplications(detail, []api.ServiceCandidate{historical, current})
				currentFound := false
				for _, app := range apps {
					if app.Unassociated || app.Service.ID == historical.Id && len(app.Workloads) != 0 {
						t.Fatal("historical evidence competed with current ownership")
					}
					if app.Service.ID == current.Id && len(app.Workloads) == 1 && !app.Missing {
						currentFound = true
					}
				}
				if !currentFound {
					t.Fatal("current discovery lost its pod association")
				}
			})
		}
	}
}

func TestHostApplicationsCatalogFailureRetainsReadOnlyInventory(t *testing.T) {
	catalog := &hostCatalog{fakeCatalog: fakeCatalog{stagingErr: errors.New("private catalog error")}, detail: api.HostDetail{Host: api.Host{Id: "host", Name: "Cluster", Kind: "cluster", Status: "fresh", InventoryAvailable: true, Workloads: []api.HostWorkload{{Name: "orphan", State: "Pending"}}}}}
	handler, err := New(catalog, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/hosts/host", nil))
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatalf("response %d: %s", w.Code, body)
	}
	for _, want := range []string{"Unassociated workloads", "orphan", "Application catalog unavailable", "Needs attention"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if w.Code != 200 || strings.Contains(body, "private catalog error") || strings.Contains(body, "/staging/services/") {
		t.Fatal("inventory is unavailable or includes editing actions")
	}
}
