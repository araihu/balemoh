package config

import (
	"fmt"
	"net/url"
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
	// FederationGatewayURL is the remote Balemoh gateway receiving local snapshots.
	FederationGatewayURL string `env:"BALEMOH_FEDERATION_GATEWAY_URL"`
	// FederationToken authenticates this instance when publishing snapshots.
	FederationToken string `env:"BALEMOH_FEDERATION_TOKEN"`
	// FederationIngestToken authenticates remote agents sending snapshots here.
	FederationIngestToken string `env:"BALEMOH_FEDERATION_INGEST_TOKEN"`
	// FederationAllowedSources registers source identities accepted by this gateway, formatted as kind/id entries.
	FederationAllowedSources []string `env:"BALEMOH_FEDERATION_ALLOWED_SOURCES" envSeparator:","`
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

	gatewayURLSet := strings.TrimSpace(o.FederationGatewayURL) != ""
	if gatewayURLSet != (strings.TrimSpace(o.FederationToken) != "") {
		return fmt.Errorf("federation gateway URL and federation token must be configured together")
	}
	if gatewayURLSet {
		parsed, err := url.Parse(strings.TrimSpace(o.FederationGatewayURL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("federation gateway URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
		}
	}

	ingestTokenSet := strings.TrimSpace(o.FederationIngestToken) != ""
	if ingestTokenSet != (len(o.FederationAllowedSources) > 0) {
		return fmt.Errorf("federation ingest token and allowed sources must be configured together")
	}
	for _, rawSource := range o.FederationAllowedSources {
		source := strings.TrimSpace(rawSource)
		parts := strings.SplitN(source, "/", 2)
		normalizedSource := ""
		if len(parts) == 2 {
			normalizedSource = strings.TrimSpace(parts[0]) + "/" + strings.TrimSpace(parts[1])
		}
		if source == "" || len(parts) != 2 || normalizedSource != source || strings.ContainsRune(source, '\x00') {
			return fmt.Errorf("federation allowed source %q must use kind/id format", source)
		}
	}
	return nil
}
