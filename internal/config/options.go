package config

import (
	"fmt"
	"net"
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
	// FederationAllowInsecureHTTP permits an explicitly configured loopback HTTP gateway for local development.
	FederationAllowInsecureHTTP bool `env:"BALEMOH_FEDERATION_ALLOW_INSECURE_HTTP" envDefault:"false"`
	// FederationAllowedSources registers source identities accepted by this gateway, formatted as kind/id entries.
	FederationAllowedSources []string `env:"BALEMOH_FEDERATION_ALLOWED_SOURCES" envSeparator:","`
	// FederationSourceTokens maps registered source identities to Bearer credentials, formatted as kind/id=token entries.
	FederationSourceTokens []string `env:"BALEMOH_FEDERATION_SOURCE_TOKENS" envSeparator:","`
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
		if parsed.Scheme == "http" {
			if !o.FederationAllowInsecureHTTP {
				return fmt.Errorf("federation gateway URL must use HTTPS unless insecure HTTP is explicitly enabled")
			}
			if !isLoopbackHost(parsed.Hostname()) {
				return fmt.Errorf("insecure federation HTTP is restricted to a loopback gateway")
			}
		}
	}

	allowed := make(map[string]struct{}, len(o.FederationAllowedSources))
	for _, rawSource := range o.FederationAllowedSources {
		source, err := parseFederationSource(rawSource)
		if err != nil {
			return err
		}
		if _, exists := allowed[source]; exists {
			return fmt.Errorf("federation allowed sources must not contain duplicates")
		}
		allowed[source] = struct{}{}
	}
	if len(o.FederationSourceTokens) != len(allowed) {
		if len(allowed) == 0 {
			if len(o.FederationSourceTokens) > 0 {
				return fmt.Errorf("federation source tokens require allowed sources")
			}
		} else {
			return fmt.Errorf("federation source tokens must provide one credential per allowed source")
		}
	}
	seenTokens := make(map[string]struct{}, len(o.FederationSourceTokens))
	for _, rawToken := range o.FederationSourceTokens {
		parts := strings.SplitN(strings.TrimSpace(rawToken), "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return fmt.Errorf("federation source tokens must use kind/id=token format")
		}
		source, err := parseFederationSource(parts[0])
		if err != nil {
			return fmt.Errorf("federation source token has invalid source: %w", err)
		}
		if _, ok := allowed[source]; !ok {
			return fmt.Errorf("federation source token references an unregistered source")
		}
		if _, exists := seenTokens[source]; exists {
			return fmt.Errorf("federation source tokens must not contain duplicates")
		}
		seenTokens[source] = struct{}{}
	}
	localSources := make(map[string]struct{}, 2)
	if o.KubernetesEnabled {
		localSources["kubernetes/"+strings.TrimSpace(o.KubernetesSourceID)] = struct{}{}
	}
	if o.ContainerEnabled {
		localSources["container/"+strings.TrimSpace(o.ContainerSourceID)] = struct{}{}
	}
	for source := range allowed {
		if _, overlaps := localSources[source]; overlaps {
			return fmt.Errorf("federation source %q cannot be both local and remote", source)
		}
	}
	return nil
}

func parseFederationSource(raw string) (string, error) {
	source := strings.TrimSpace(raw)
	parts := strings.SplitN(source, "/", 2)
	if source == "" || len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[0]) != parts[0] || strings.TrimSpace(parts[1]) != parts[1] || strings.ContainsRune(parts[0], '\x00') || strings.ContainsRune(parts[1], '\x00') || strings.ContainsRune(parts[0], '=') || strings.ContainsRune(parts[1], '=') {
		return "", fmt.Errorf("federation allowed source %q must use kind/id format", source)
	}
	return parts[0] + "/" + parts[1], nil
}

func isLoopbackHost(hostname string) bool {
	if strings.EqualFold(strings.TrimSpace(hostname), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(hostname, "[]"))
	return ip != nil && ip.IsLoopback()
}
