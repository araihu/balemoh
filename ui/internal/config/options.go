// Package config owns the UI process configuration and its documented
// environment contract.
package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Options configures the Balemoh server-rendered UI BFF.
//
type Options struct {
	// HTTPAddr is the address where the UI BFF listens.
	HTTPAddr string `env:"BALEMOH_UI_HTTP_ADDR" envDefault:":8081"`
	// APIBaseURL is the absolute URL of the Balemoh API upstream.
	APIBaseURL string `env:"BALEMOH_UI_API_BASE_URL" envDefault:"http://127.0.0.1:8080"`
	// RequestTimeout bounds one UI-to-API request.
	RequestTimeout time.Duration `env:"BALEMOH_UI_REQUEST_TIMEOUT" envDefault:"5s"`
	// ShutdownTimeout bounds graceful UI shutdown.
	ShutdownTimeout time.Duration `env:"BALEMOH_UI_SHUTDOWN_TIMEOUT" envDefault:"5s"`
}

// Load parses the process environment and validates the resulting options.
func Load() (Options, error) {
	options, err := env.ParseAs[Options]()
	if err != nil {
		return Options{}, fmt.Errorf("parse UI environment: %w", err)
	}
	options = normalize(options)
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

// ParseEnvironment parses a supplied environment map. It keeps configuration
// tests independent from the machine's process environment.
func ParseEnvironment(values map[string]string) (Options, error) {
	options, err := env.ParseAsWithOptions[Options](env.Options{Environment: values})
	if err != nil {
		return Options{}, fmt.Errorf("parse UI environment: %w", err)
	}
	options = normalize(options)
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

func normalize(options Options) Options {
	options.HTTPAddr = strings.TrimSpace(options.HTTPAddr)
	options.APIBaseURL = strings.TrimSpace(options.APIBaseURL)
	return options
}

// Validate rejects configuration that would make the UI unusable or send
// requests to an ambiguous upstream URL.
func (o Options) Validate() error {
	if strings.TrimSpace(o.HTTPAddr) == "" {
		return fmt.Errorf("UI HTTP address must not be empty")
	}
	if o.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if o.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}

	baseURL := strings.TrimSpace(o.APIBaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("API base URL must be absolute with a scheme and host")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("API base URL scheme must be http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("API base URL must not include credentials")
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("API base URL must not include a query")
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("API base URL must not include a fragment")
	}
	return nil
}
