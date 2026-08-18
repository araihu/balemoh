package container

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/araihu/balemoh/internal/application/catalog"
	dockertypes "github.com/docker/docker/api/types/container"
)

type fakeClient struct {
	containers []dockertypes.Summary
	err        error
	options    dockertypes.ListOptions
}

func (f *fakeClient) ContainerList(_ context.Context, options dockertypes.ListOptions) ([]dockertypes.Summary, error) {
	f.options = options
	return f.containers, f.err
}

func TestDiscovererStagesContainersAndComposeServices(t *testing.T) {
	client := &fakeClient{containers: []dockertypes.Summary{
		{
			ID:     "web-container-1",
			Names:  []string{"/homelab-web-1"},
			Image:  "ghcr.io/example/web:1.2",
			State:  dockertypes.StateRunning,
			Labels: dockerComposeLabels("homelab", "web"),
			Ports: []dockertypes.Port{
				{IP: "127.0.0.1", PrivatePort: 8080, PublicPort: 18080, Type: "tcp"},
				{PrivatePort: 8443, PublicPort: 18443, Type: "tcp"},
				{IP: "127.0.0.1", PrivatePort: 53, Type: "udp"},
			},
		},
		{
			ID:     "web-container-2",
			Names:  []string{"/homelab-web-2"},
			Image:  "ghcr.io/example/web:1.2",
			State:  dockertypes.StateRunning,
			Labels: podmanComposeLabels("homelab", "web"),
			Ports: []dockertypes.Port{
				{IP: "::1", PrivatePort: 8080, PublicPort: 28080, Type: "tcp"},
			},
		},
		{
			ID:     "standalone-container",
			Names:  []string{"/standalone"},
			Image:  "docker.io/library/redis:7",
			State:  dockertypes.StateRunning,
			Labels: map[string]string{"com.example.role": "cache"},
		},
	}}
	discoverer, err := NewDiscoverer(client, "docker-desktop")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}

	candidates, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if client.options.All {
		t.Fatal("ContainerList() All = true, want running containers only")
	}
	if len(candidates) != 4 {
		t.Fatalf("Discover() candidates = %d, want three containers plus one Compose service: %#v", len(candidates), candidates)
	}

	byIdentity := make(map[string]catalog.Candidate, len(candidates))
	for _, candidate := range candidates {
		if err := candidate.Validate(); err != nil {
			t.Fatalf("candidate %s/%s invalid: %v", candidate.Resource.Kind, candidate.Resource.Name, err)
		}
		if candidate.Source.Kind != sourceKind || candidate.Source.ID != "docker-desktop" {
			t.Fatalf("candidate source = %#v, want container/docker-desktop", candidate.Source)
		}
		key := candidate.Resource.Kind + "/" + candidate.Resource.Namespace + "/" + candidate.Resource.Name
		byIdentity[key] = candidate
	}

	containerCandidate := byIdentity["container//homelab-web-1"]
	if containerCandidate.Metadata["container.id"] != "web-container-1" || containerCandidate.Metadata["container.state"] != "running" {
		t.Fatalf("container metadata = %#v, want identity and running state", containerCandidate.Metadata)
	}
	if containerCandidate.Metadata["container.publishedIPs"] != "0.0.0.0,127.0.0.1" {
		t.Fatalf("container published IPs = %q, want sorted host bindings", containerCandidate.Metadata["container.publishedIPs"])
	}
	if !reflect.DeepEqual(containerCandidate.Images, []string{"ghcr.io/example/web:1.2"}) {
		t.Fatalf("container images = %#v, want image reference", containerCandidate.Images)
	}
	if len(containerCandidate.Endpoints) != 2 {
		t.Fatalf("container endpoints = %#v, want only published ports", containerCandidate.Endpoints)
	}
	if containerCandidate.Endpoints[0].URL != "tcp://0.0.0.0:18443" || containerCandidate.Endpoints[1].URL != "tcp://127.0.0.1:18080" {
		t.Fatalf("container endpoints = %#v, want host IP and public port URLs", containerCandidate.Endpoints)
	}
	if containerCandidate.Metadata["compose.project"] != "homelab" || containerCandidate.Metadata["compose.service"] != "web" {
		t.Fatalf("container Compose metadata = %#v, want project/service", containerCandidate.Metadata)
	}

	composeCandidate := byIdentity["compose-service/homelab/web"]
	if composeCandidate.Metadata["compose.containers"] != "2" {
		t.Fatalf("Compose container count = %q, want 2", composeCandidate.Metadata["compose.containers"])
	}
	if !reflect.DeepEqual(composeCandidate.Images, []string{"ghcr.io/example/web:1.2"}) {
		t.Fatalf("Compose images = %#v, want deduplicated image", composeCandidate.Images)
	}
	if composeCandidate.Metadata["container.publishedIPs"] != "0.0.0.0,127.0.0.1,::1" {
		t.Fatalf("Compose published IPs = %q, want aggregated host bindings", composeCandidate.Metadata["container.publishedIPs"])
	}
	if len(composeCandidate.Endpoints) != 3 {
		t.Fatalf("Compose endpoints = %#v, want one endpoint per published binding", composeCandidate.Endpoints)
	}

	standalone := byIdentity["container//standalone"]
	if len(standalone.Endpoints) != 0 || standalone.Metadata["compose.service"] != "" {
		t.Fatalf("standalone candidate = %#v, want no Compose or published-port metadata", standalone)
	}
}

func TestDiscovererPropagatesContainerListErrors(t *testing.T) {
	wantErr := errors.New("container daemon unavailable")
	discoverer, err := NewDiscoverer(&fakeClient{err: wantErr}, "docker-local")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}

	_, err = discoverer.Discover(context.Background())
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "list running containers") {
		t.Fatalf("Discover() error = %v, want wrapped list error", err)
	}
}

func TestNewDiscovererRejectsMissingDependencies(t *testing.T) {
	if _, err := NewDiscoverer(nil, "docker-local"); err == nil {
		t.Fatal("NewDiscoverer(nil) error = nil, want dependency error")
	}
	if _, err := NewDiscoverer(&fakeClient{}, " "); err == nil {
		t.Fatal("NewDiscoverer(blank source) error = nil, want source ID error")
	}
	if _, err := NewDiscoverer(&fakeClient{}, "bad\x00source"); err == nil {
		t.Fatal("NewDiscoverer(NUL source) error = nil, want source ID error")
	}
}

func dockerComposeLabels(project, service string) map[string]string {
	return map[string]string{
		composeProjectLabel:       project,
		composeServiceLabel:       service,
		composeWorkingDirLabel:    "/srv/homelab",
		composeConfigFilesLabel:   "/srv/homelab/compose.yaml",
		composeContainerNumberKey: "1",
	}
}

func podmanComposeLabels(project, service string) map[string]string {
	return map[string]string{
		podmanProjectLabel:       project,
		podmanServiceLabel:       service,
		podmanWorkingDirLabel:    "/srv/homelab",
		podmanConfigFilesLabel:   "/srv/homelab/compose.yaml",
		podmanContainerNumberKey: "2",
	}
}
