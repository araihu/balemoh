package config

import (
	"strings"
	"testing"
	"time"
)

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
