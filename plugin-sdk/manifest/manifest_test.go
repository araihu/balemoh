package manifest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
)

const valid = `{"manifestVersion":1,"id":"example.fixture","version":"0.0.1","name":"Fixture","protocol":"balemoh.plugins/v1alpha1","uiProfile":"balemoh.goshtoso/v1alpha2","targets":{"linux/amd64":{"ui":"bin/ui"}},"connectionSchema":"schema.json","secrets":[],"queries":[],"pages":[{"id":"current","title":"Fixture page","path":"/current/{id}","methods":["GET"],"capabilities":[]}],"navigation":[{"id":"fixture","title":"Fixture","page":"current"}],"widgets":[],"assets":[],"capabilities":{"required":["page.content/v1","sidebar.navigation/v1"],"optional":[]}}`

func TestDecodeClosedManifestAndRoutes(t *testing.T) {
	for name, modify := range map[string]func(map[string]any){
		"unknown field":           func(m map[string]any) { m["executeShell"] = "oops" },
		"previous visual profile": func(m map[string]any) { m["uiProfile"] = "balemoh.goshtoso/v1alpha1" },
		"incompatible profile":    func(m map[string]any) { m["uiProfile"] = "other/v1" },
		"missing required field":  func(m map[string]any) { delete(m, "queries") },
		"missing page title":      func(m map[string]any) { delete(m["pages"].([]any)[0].(map[string]any), "title") },
		"undeclared page": func(m map[string]any) {
			m["capabilities"].(map[string]any)["required"] = []string{"sidebar.navigation/v1"}
		},
		"undeclared sidebar": func(m map[string]any) { m["capabilities"].(map[string]any)["required"] = []string{"page.content/v1"} },
		"undeclared widget": func(m map[string]any) {
			m["widgets"] = []any{map[string]any{"id": "summary", "title": "Summary", "optionsSchema": "schema.json", "capabilities": []string{}}}
		},
		"nested unknown field": func(m map[string]any) {
			m["targets"].(map[string]any)["linux/amd64"].(map[string]any)["args"] = []string{"sh"}
		},
		"ambiguous routes": func(m map[string]any) {
			m["pages"] = append(m["pages"].([]any), map[string]any{"id": "other", "title": "Other", "path": "/current/{other}", "methods": []string{"GET"}, "capabilities": []string{}})
		},
		"unknown navigation":       func(m map[string]any) { m["navigation"].([]any)[0].(map[string]any)["page"] = "unknown" },
		"package escape":           func(m map[string]any) { m["connectionSchema"] = "../outside.json" },
		"invalid prerelease":       func(m map[string]any) { m["version"] = "1.0.0-01" },
		"empty version identifier": func(m map[string]any) { m["version"] = "1.0.0-a..b" },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			_ = json.Unmarshal([]byte(valid), &m)
			modify(m)
			data, _ := json.Marshal(m)
			if _, _, err := manifest.Decode(strings.NewReader(string(data))); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
	m, d, err := manifest.Decode(strings.NewReader(valid))
	if err != nil || m.ID != "example.fixture" || len(d) != 64 {
		t.Fatalf("valid manifest: %v", err)
	}
	if _, _, err := manifest.Decode(strings.NewReader(valid + ` {}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, _, err := manifest.Decode(strings.NewReader(strings.Repeat(" ", manifest.MaxBytes+1))); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

func TestPackageSchemasAndSymlinkContainment(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		valid        bool
	}{
		{"closed local schema", `{"type":"object","additionalProperties":false}`, true},
		{"local reference", `{"$ref":"common.json"}`, true},
		{"remote reference", `{"$ref":"https://example.com/schema.json"}`, false},
		{"outside reference", `{"$ref":"../outside.json"}`, false},
		{"invalid schema", `{"type":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string]string{"plugin.json": valid, "schema.json": tc.schema, "common.json": `{"type":"object"}`, "bin/ui": "fixture"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0700); err != nil {
					t.Fatal(err)
				}
			}
			_, err := manifest.Load(root)
			if (err == nil) != tc.valid {
				t.Fatalf("package validity = %v, error %v", tc.valid, err)
			}
			if !tc.valid {
				return
			}
			outside := filepath.Join(t.TempDir(), "outside.json")
			if err := os.WriteFile(outside, []byte(`{"type":"object"}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "schema.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "schema.json")); err != nil {
				t.Fatal(err)
			}
			if _, err := manifest.Load(root); err == nil {
				t.Fatal("package accepted schema symlink outside its root")
			}
		})
	}
}

func TestAssetURLMetacharactersRejectedAtDecodeAndPackageLoad(t *testing.T) {
	for _, file := range []string{"assets/a?b.js", "assets/a#b.js"} {
		var value map[string]any
		_ = json.Unmarshal([]byte(valid), &value)
		value["assets"] = []any{map[string]any{"id": "module", "file": file, "kind": "module", "mediaType": "text/javascript"}}
		data, _ := json.Marshal(value)
		if _, _, err := manifest.Decode(strings.NewReader(string(data))); err == nil {
			t.Fatal("accepted URL-breaking asset", file)
		}
		root := t.TempDir()
		_ = os.WriteFile(filepath.Join(root, "plugin.json"), data, 0600)
		if _, err := manifest.Load(root); err == nil {
			t.Fatal("package accepted URL-breaking asset", file)
		}
	}
}

func TestNavigationIconMetadataIsOptional(t *testing.T) {
	for _, symbol := range []string{"", "hi-16-solid-bell", "future-icon"} {
		t.Run(symbol, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal([]byte(valid), &input); err != nil {
				t.Fatal(err)
			}
			if symbol != "" {
				input["navigation"].([]any)[0].(map[string]any)["icon"] = symbol
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			m, _, err := manifest.Decode(strings.NewReader(string(encoded)))
			if err != nil {
				t.Fatal(err)
			}
			if m.Navigation[0].Icon != symbol {
				t.Fatal("navigation icon metadata lost")
			}
			encoded, err = json.Marshal(m.Navigation[0])
			if err != nil {
				t.Fatal(err)
			}
			if symbol == "" && strings.Contains(string(encoded), `"icon"`) {
				t.Fatal("optional icon changed legacy manifest encoding")
			}
		})
	}
}
