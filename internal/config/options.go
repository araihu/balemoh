package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Options contains Balemoh's runtime environment configuration.
type Options struct {
	// HTTPAddr is the address used by the HTTP server.
	HTTPAddr string `env:"BALEMOH_HTTP_ADDR" envDefault:":8080"`
	// DatabasePath is the path to Balemoh's SQLite database.
	DatabasePath string `env:"BALEMOH_DATABASE_PATH" envDefault:"./data/balemoh.db"`
	// KubernetesEnabled enables the in-cluster Kubernetes discovery source.
	KubernetesEnabled bool `env:"BALEMOH_KUBERNETES_ENABLED" envDefault:"false"`
	// KubernetesSourceID is the stable identity used to scope Kubernetes candidates.
	KubernetesSourceID string `env:"BALEMOH_KUBERNETES_SOURCE_ID"`
	// KubernetesNamespace limits Kubernetes discovery to one namespace.
	KubernetesNamespace string `env:"BALEMOH_KUBERNETES_NAMESPACE"`
	// ContainerEnabled enables the Docker-compatible Docker or Podman container discovery source.
	ContainerEnabled bool `env:"BALEMOH_CONTAINER_ENABLED" envDefault:"false"`
	// ContainerSourceID is the stable identity used to scope container candidates.
	ContainerSourceID string `env:"BALEMOH_CONTAINER_SOURCE_ID"`
	// ContainerHost is the Docker-compatible API socket or endpoint.
	ContainerHost string `env:"BALEMOH_CONTAINER_HOST" envDefault:"unix:///var/run/docker.sock"`
}

// Load parses the process environment into runtime configuration.
func Load() (Options, error) {
	options, err := env.ParseAs[Options]()
	if err != nil {
		return Options{}, err
	}
	return options, options.Validate()
}

// ParseEnvironment parses the supplied environment into runtime configuration.
func ParseEnvironment(environment map[string]string) (Options, error) {
	options, err := env.ParseAsWithOptions[Options](env.Options{Environment: environment})
	if err != nil {
		return Options{}, err
	}
	return options, options.Validate()
}

// Validate checks that required configuration values contain non-whitespace content.
func (o Options) Validate() error {
	if strings.TrimSpace(o.HTTPAddr) == "" {
		return fmt.Errorf("HTTP address must not be empty")
	}
	if strings.TrimSpace(o.DatabasePath) == "" {
		return fmt.Errorf("database path must not be empty")
	}
	if o.KubernetesEnabled {
		if strings.TrimSpace(o.KubernetesSourceID) == "" {
			return fmt.Errorf("Kubernetes source ID must not be empty when discovery is enabled")
		}
		if strings.TrimSpace(o.KubernetesNamespace) == "" {
			return fmt.Errorf("Kubernetes namespace must not be empty when discovery is enabled")
		}
	}
	if o.ContainerEnabled {
		if strings.TrimSpace(o.ContainerSourceID) == "" {
			return fmt.Errorf("container source ID must not be empty when discovery is enabled")
		}
		if strings.TrimSpace(o.ContainerHost) == "" {
			return fmt.Errorf("container host must not be empty when discovery is enabled")
		}
	}
	return nil
}
