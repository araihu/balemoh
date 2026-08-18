package config

import "testing"

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
	if got.ContainerEnabled {
		t.Fatal("ContainerEnabled = true, want false by default")
	}
	if got.ContainerHost != "unix:///var/run/docker.sock" {
		t.Fatalf("ContainerHost = %q, want default Docker socket", got.ContainerHost)
	}
}

func TestParseEnvironmentUsesExplicitOverrides(t *testing.T) {
	got, err := ParseEnvironment(map[string]string{
		"BALEMOH_HTTP_ADDR":            "127.0.0.1:9090",
		"BALEMOH_DATABASE_PATH":        "/tmp/balemoh.db",
		"BALEMOH_KUBERNETES_ENABLED":   "true",
		"BALEMOH_KUBERNETES_SOURCE_ID": "kind-balemoh",
		"BALEMOH_KUBERNETES_NAMESPACE": "balemoh-dev",
		"BALEMOH_CONTAINER_ENABLED":    "true",
		"BALEMOH_CONTAINER_SOURCE_ID":  "docker-desktop",
		"BALEMOH_CONTAINER_HOST":       "unix:///Users/test/.docker/run/docker.sock",
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
	if !got.KubernetesEnabled || got.KubernetesSourceID != "kind-balemoh" || got.KubernetesNamespace != "balemoh-dev" {
		t.Fatalf("Kubernetes options = %#v, want enabled source and namespace", got)
	}
	if !got.ContainerEnabled || got.ContainerSourceID != "docker-desktop" || got.ContainerHost != "unix:///Users/test/.docker/run/docker.sock" {
		t.Fatalf("Container options = %#v, want enabled source and host", got)
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

func TestParseEnvironmentRejectsEnabledKubernetesWithoutIdentityOrNamespace(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "source ID",
			env: map[string]string{
				"BALEMOH_KUBERNETES_ENABLED":   "true",
				"BALEMOH_KUBERNETES_NAMESPACE": "balemoh-dev",
			},
		},
		{
			name: "namespace",
			env: map[string]string{
				"BALEMOH_KUBERNETES_ENABLED":   "true",
				"BALEMOH_KUBERNETES_SOURCE_ID": "kind-balemoh",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseEnvironment(test.env); err == nil {
				t.Fatal("ParseEnvironment() error = nil, want Kubernetes validation error")
			}
		})
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
