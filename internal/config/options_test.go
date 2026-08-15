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
}

func TestParseEnvironmentUsesExplicitOverrides(t *testing.T) {
	got, err := ParseEnvironment(map[string]string{
		"BALEMOH_HTTP_ADDR":     "127.0.0.1:9090",
		"BALEMOH_DATABASE_PATH": "/tmp/balemoh.db",
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
