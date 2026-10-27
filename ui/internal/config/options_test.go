package config

import (
	"strings"
	"testing"
	"time"
)

func TestPluginPreviewsAreOptIn(t *testing.T) {
	o, err := ParseEnvironment(map[string]string{})
	if err != nil || o.PluginPreviewConfig != "" {
		t.Fatalf("previews enabled by default: %v", err)
	}
	o, err = ParseEnvironment(map[string]string{"BALEMOH_UI_PLUGIN_PREVIEW_CONFIG": "/tmp/preview.json"})
	if err != nil || o.PluginPreviewConfig != "/tmp/preview.json" {
		t.Fatalf("preview config missing: %v", err)
	}
}

func TestParseEnvironmentUsesOperationalDefaults(t *testing.T) {
	t.Parallel()

	got, err := ParseEnvironment(nil)
	if err != nil {
		t.Fatalf("ParseEnvironment() error = %v", err)
	}

	if got.HTTPAddr != ":8081" {
		t.Errorf("HTTPAddr = %q, want %q", got.HTTPAddr, ":8081")
	}
	if got.APIBaseURL != "http://127.0.0.1:8080" {
		t.Errorf("APIBaseURL = %q, want %q", got.APIBaseURL, "http://127.0.0.1:8080")
	}
	if got.RequestTimeout != 5*time.Second {
		t.Errorf("RequestTimeout = %s, want 5s", got.RequestTimeout)
	}
	if got.ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 5s", got.ShutdownTimeout)
	}
}

func TestParseEnvironmentAcceptsExplicitValues(t *testing.T) {
	t.Parallel()

	got, err := ParseEnvironment(map[string]string{
		"BALEMOH_UI_HTTP_ADDR":        "127.0.0.1:18081",
		"BALEMOH_UI_API_BASE_URL":     "https://balemoh-api.example.test/",
		"BALEMOH_UI_REQUEST_TIMEOUT":  "2s",
		"BALEMOH_UI_SHUTDOWN_TIMEOUT": "9s",
	})
	if err != nil {
		t.Fatalf("ParseEnvironment() error = %v", err)
	}

	if got.HTTPAddr != "127.0.0.1:18081" || got.APIBaseURL != "https://balemoh-api.example.test/" {
		t.Fatalf("parsed endpoints = %#v", got)
	}
	if got.RequestTimeout != 2*time.Second || got.ShutdownTimeout != 9*time.Second {
		t.Fatalf("parsed timeouts = %#v", got)
	}
}

func TestParseEnvironmentRejectsUnsafeOrInvalidValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "empty cache", env: map[string]string{"BALEMOH_UI_ICON_CACHE_DIR": " "}, want: "icon cache"},
		{name: "invalid icon timeout", env: map[string]string{"BALEMOH_UI_ICON_STARTUP_TIMEOUT": "0s"}, want: "icon startup timeout"},
		{name: "missing scheme", env: map[string]string{"BALEMOH_UI_API_BASE_URL": "127.0.0.1:8080"}, want: "absolute"},
		{name: "credentials", env: map[string]string{"BALEMOH_UI_API_BASE_URL": "https://user:password@example.test"}, want: "credentials"},
		{name: "query", env: map[string]string{"BALEMOH_UI_API_BASE_URL": "https://example.test?token=secret"}, want: "query"},
		{name: "request timeout", env: map[string]string{"BALEMOH_UI_REQUEST_TIMEOUT": "0s"}, want: "request timeout"},
		{name: "shutdown timeout", env: map[string]string{"BALEMOH_UI_SHUTDOWN_TIMEOUT": "-1s"}, want: "shutdown timeout"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseEnvironment(tc.env)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("ParseEnvironment() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestPluginConfigurationRequiresCredentialAndExcludesPreview(t *testing.T) {
	for _, env := range []map[string]string{{"BALEMOH_UI_PLUGIN_CONFIG": "plugins.json"}, {"BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE": "token"}, {"BALEMOH_UI_PLUGIN_CONFIG": "plugins.json", "BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE": "token", "BALEMOH_UI_PLUGIN_PREVIEW_CONFIG": "preview.json"}} {
		if _, err := ParseEnvironment(env); err == nil {
			t.Fatal("invalid plugin environment accepted")
		}
	}
	got, err := ParseEnvironment(map[string]string{"BALEMOH_UI_PLUGIN_CONFIG": "plugins.json", "BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE": "token"})
	if err != nil || got.PluginConfig == "" {
		t.Fatal("valid data plugin environment rejected", err)
	}
}

func TestAppearanceEnvironment(t *testing.T) {
	for _, tt := range []struct {
		theme, mode string
		valid       bool
	}{{"goshtoso", "system", true}, {"minimal", "light", true}, {"minimal", "dark", true}, {"unknown", "light", false}, {"goshtoso", "unknown", false}, {"", "light", true}} {
		options, err := ParseEnvironment(map[string]string{"BALEMOH_UI_THEME": tt.theme, "BALEMOH_UI_COLOR_SCHEME": tt.mode})
		if (err == nil) != tt.valid {
			t.Fatalf("appearance %q/%q: %+v %v", tt.theme, tt.mode, options, err)
		}
	}
}
