package config

import (
	"testing"
	"time"
)

func TestParseEnvironmentUsesDefaults(t *testing.T) {
	got, err := ParseEnvironment(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", got.HTTPAddr)
	}
	if got.DatabasePath != "./data/balemoh.db" {
		t.Fatalf("DatabasePath = %q", got.DatabasePath)
	}
	if got.DiscoverySyncInterval != 5*time.Minute {
		t.Fatalf("DiscoverySyncInterval = %s, want 5m", got.DiscoverySyncInterval)
	}
	if got.ContainerEnabled {
		t.Fatal("ContainerEnabled = true, want false by default")
	}
	if got.ContainerHost != "unix:///var/run/docker.sock" {
		t.Fatalf("ContainerHost = %q, want default Docker socket", got.ContainerHost)
	}
	if got.FederationGatewayURL != "" || got.FederationToken != "" || got.FederationAllowInsecureHTTP || len(got.FederationAllowedSources) != 0 || len(got.FederationSourceTokens) != 0 {
		t.Fatalf("federation options = %#v, want disabled by default", got)
	}
}

func TestParseEnvironmentUsesExplicitOverrides(t *testing.T) {
	got, err := ParseEnvironment(map[string]string{
		"BALEMOH_HTTP_ADDR":                  "127.0.0.1:9090",
		"BALEMOH_DATABASE_PATH":              "/tmp/balemoh.db",
		"BALEMOH_DISCOVERY_SYNC_INTERVAL":    "45s",
		"BALEMOH_KUBERNETES_ENABLED":         "true",
		"BALEMOH_KUBERNETES_SOURCE_ID":       "kind-balemoh",
		"BALEMOH_KUBERNETES_NAMESPACE":       "balemoh-dev",
		"BALEMOH_CONTAINER_ENABLED":          "true",
		"BALEMOH_CONTAINER_SOURCE_ID":        "docker-desktop",
		"BALEMOH_CONTAINER_HOST":             "unix:///Users/test/.docker/run/docker.sock",
		"BALEMOH_FEDERATION_GATEWAY_URL":     "https://gateway.example.test/balemoh",
		"BALEMOH_FEDERATION_TOKEN":           "agent-secret",
		"BALEMOH_FEDERATION_ALLOWED_SOURCES": "kubernetes/cluster-1,container/docker-local",
		"BALEMOH_FEDERATION_SOURCE_TOKENS":   "kubernetes/cluster-1=kubernetes-secret,container/docker-local=container-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPAddr != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr = %q", got.HTTPAddr)
	}
	if got.DatabasePath != "/tmp/balemoh.db" {
		t.Fatalf("DatabasePath = %q", got.DatabasePath)
	}
	if got.DiscoverySyncInterval != 45*time.Second {
		t.Fatalf("DiscoverySyncInterval = %s, want 45s", got.DiscoverySyncInterval)
	}
	if !got.KubernetesEnabled || got.KubernetesSourceID != "kind-balemoh" || got.KubernetesNamespace != "balemoh-dev" {
		t.Fatalf("Kubernetes options = %#v, want enabled source and namespace", got)
	}
	if !got.ContainerEnabled || got.ContainerSourceID != "docker-desktop" || got.ContainerHost != "unix:///Users/test/.docker/run/docker.sock" {
		t.Fatalf("Container options = %#v, want enabled source and host", got)
	}
	if got.FederationGatewayURL != "https://gateway.example.test/balemoh" || got.FederationToken != "agent-secret" {
		t.Fatalf("federation publisher options = %#v", got)
	}
	if len(got.FederationAllowedSources) != 2 || got.FederationAllowedSources[1] != "container/docker-local" || len(got.FederationSourceTokens) != 2 {
		t.Fatalf("federation gateway options = %#v", got)
	}
}

func TestParseEnvironmentRejectsWhitespaceOnlyValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "http address", env: map[string]string{"BALEMOH_HTTP_ADDR": "   "}},
		{name: "database path", env: map[string]string{"BALEMOH_DATABASE_PATH": "\t\n"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseEnvironment(tt.env); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseEnvironmentRejectsEnabledKubernetesWithoutIdentity(t *testing.T) {
	if _, err := ParseEnvironment(map[string]string{
		"BALEMOH_KUBERNETES_ENABLED":   "true",
		"BALEMOH_KUBERNETES_NAMESPACE": "balemoh-dev",
	}); err == nil {
		t.Fatal("ParseEnvironment() error = nil, want Kubernetes source ID validation error")
	}
}

func TestParseEnvironmentAllowsClusterWideKubernetesDiscovery(t *testing.T) {
	got, err := ParseEnvironment(map[string]string{
		"BALEMOH_KUBERNETES_ENABLED":   "true",
		"BALEMOH_KUBERNETES_SOURCE_ID": "homelab",
	})
	if err != nil {
		t.Fatalf("ParseEnvironment() error = %v, want cluster-wide discovery to be valid", err)
	}
	if got.KubernetesNamespace != "" {
		t.Fatalf("KubernetesNamespace = %q, want empty cluster-wide namespace", got.KubernetesNamespace)
	}
}

func TestParseEnvironmentRejectsNegativeDiscoverySyncInterval(t *testing.T) {
	if _, err := ParseEnvironment(map[string]string{
		"BALEMOH_DISCOVERY_SYNC_INTERVAL": "-1s",
	}); err == nil {
		t.Fatal("ParseEnvironment() error = nil, want negative sync interval validation error")
	}
}

func TestParseEnvironmentRejectsEnabledContainerWithoutIdentityOrHost(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "source ID",
			env: map[string]string{
				"BALEMOH_CONTAINER_ENABLED": "true",
			},
		},
		{
			name: "host",
			env: map[string]string{
				"BALEMOH_CONTAINER_ENABLED":   "true",
				"BALEMOH_CONTAINER_SOURCE_ID": "docker-local",
				"BALEMOH_CONTAINER_HOST":      " \t",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseEnvironment(test.env); err == nil {
				t.Fatal("ParseEnvironment() error = nil, want container validation error")
			}
		})
	}
}

func TestParseEnvironmentRejectsPartialFederationConfiguration(t *testing.T) {
	tests := []map[string]string{
		{"BALEMOH_FEDERATION_GATEWAY_URL": "https://gateway.example.test"},
		{"BALEMOH_FEDERATION_TOKEN": "agent-secret"},
		{"BALEMOH_FEDERATION_ALLOWED_SOURCES": "container/docker-local"},
		{"BALEMOH_FEDERATION_SOURCE_TOKENS": "container/docker-local=secret"},
		{"BALEMOH_FEDERATION_GATEWAY_URL": "ftp://gateway.example.test", "BALEMOH_FEDERATION_TOKEN": "agent-secret"},
		{"BALEMOH_FEDERATION_GATEWAY_URL": "https://gateway.example.test?token=secret", "BALEMOH_FEDERATION_TOKEN": "agent-secret"},
		{"BALEMOH_FEDERATION_ALLOWED_SOURCES": "container/docker-local"},
		{"BALEMOH_FEDERATION_ALLOWED_SOURCES": "container/docker-local", "BALEMOH_FEDERATION_SOURCE_TOKENS": "container/docker-local=secret,container/docker-local=other"},
		{"BALEMOH_CONTAINER_ENABLED": "true", "BALEMOH_CONTAINER_SOURCE_ID": "docker-local", "BALEMOH_FEDERATION_ALLOWED_SOURCES": "container/docker-local", "BALEMOH_FEDERATION_SOURCE_TOKENS": "container/docker-local=secret"},
		{"BALEMOH_FEDERATION_GATEWAY_URL": "http://gateway.example.test", "BALEMOH_FEDERATION_TOKEN": "agent-secret", "BALEMOH_FEDERATION_ALLOW_INSECURE_HTTP": "true"},
	}
	for index, environment := range tests {
		if _, err := ParseEnvironment(environment); err == nil {
			t.Fatalf("case %d: ParseEnvironment() error = nil, want federation validation error", index)
		}
	}
}

func TestParseEnvironmentAllowsLoopbackHTTPOnlyWithExplicitOptIn(t *testing.T) {
	valid := map[string]string{
		"BALEMOH_FEDERATION_GATEWAY_URL":         "http://127.0.0.1:8080",
		"BALEMOH_FEDERATION_TOKEN":               "agent-secret",
		"BALEMOH_FEDERATION_ALLOW_INSECURE_HTTP": "true",
	}
	if _, err := ParseEnvironment(valid); err != nil {
		t.Fatalf("ParseEnvironment() error = %v, want loopback HTTP to be allowed with opt-in", err)
	}
}
