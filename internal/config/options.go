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
	return nil
}
