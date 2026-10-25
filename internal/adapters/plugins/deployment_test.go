package plugins

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	app "github.com/araihu/balemoh/internal/application/plugins"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
)

func deploymentFixture(t *testing.T) (string, File) {
	t.Helper()
	root := t.TempDir()
	pkg := filepath.Join(root, "package")
	if err := os.Mkdir(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"ui", "backend"} {
		cmd := exec.Command("go", "build", "-o", filepath.Join(pkg, role), "./runtime/testdata/"+role)
		cmd.Dir = "../../../plugin-sdk"
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build fixture: %v: %s", err, out)
		}
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui", Backend: "backend"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{{ID: "token", Required: true}}, Queries: []manifest.Query{{ID: "fixture.read", InputSchema: "schema.json", OutputSchema: "output.json", Capability: "data.query/v1"}}, Pages: []manifest.Page{{ID: "current", Title: "Current", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{"data.query/v1"}}}, Navigation: []manifest.Navigation{}, Widgets: []manifest.Widget{}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "data.query/v1"}, Optional: []string{}}}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(pkg, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(m)
	write("plugin.json", data)
	write("schema.json", []byte(`{"type":"object","additionalProperties":false}`))
	write("output.json", []byte(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`))
	p, err := manifest.Load(pkg)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := p.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "token"), []byte("declared-only"), 0600); err != nil {
		t.Fatal(err)
	}
	file := File{Installations: []Installation{{Package: pkg, ManifestDigest: p.Digest, PackageDigest: digest, Connections: []Connection{{Key: "one", Name: "One", Config: json.RawMessage(`{}`), Secrets: map[string]string{"token": "token"}, Grants: []string{"fixture.read"}}}}}}
	return filepath.Join(root, "deployment.json"), file
}
func saveDeployment(t *testing.T, path string, file File) {
	t.Helper()
	data, _ := json.Marshal(file)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestDeploymentPinsSecretsAndValidation(t *testing.T) {
	path, file := deploymentFixture(t)
	saveDeployment(t, path, file)
	deployment, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	provisions := deployment.Provisions()
	if len(provisions) != 1 || provisions[0].Digest == "" {
		t.Fatal("missing provision hash")
	}
	for _, mutate := range []func(*File){
		func(f *File) { f.Installations[0].PackageDigest = strings.Repeat("0", 64) },
		func(f *File) { f.Installations[0].ManifestDigest = "" },
		func(f *File) { f.Installations[0].Connections[0].Secrets = nil },
		func(f *File) { f.Installations[0].Connections[0].Secrets = map[string]string{"undeclared": "token"} },
		func(f *File) { f.Installations[0].Connections[0].Grants = []string{"unregistered"} },
		func(f *File) { f.Installations[0].Connections[0].Grants = nil },
		func(f *File) {
			f.Installations[0].Connections[0].Config = json.RawMessage(`{"unexpected":"declared-only"}`)
		},
	} {
		data, _ := json.Marshal(file)
		var copy File
		json.Unmarshal(data, &copy)
		mutate(&copy)
		saveDeployment(t, path, copy)
		rejected, err := Load(path)
		if err != nil || len(rejected.Problems()) == 0 || len(rejected.Provisions()) != 0 {
			t.Fatal("invalid installation was not isolated", err)
		}
		public, _ := json.Marshal(rejected.Problems())
		if strings.Contains(string(public), "declared-only") {
			t.Fatal("rejected installation leaked secret")
		}

	}
	for _, data := range []string{"", `null`, `{}`, `{"installations":null}`, `{"installations":[],"unknown":"declared-only"}`, strings.Repeat("x", (256<<10)+1)} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || strings.Contains(err.Error(), "declared-only") {
			t.Fatal("invalid deployment accepted", err)
		}
	}
	saveDeployment(t, path, file)
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "token"), []byte("changed-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	rotated, err := Load(path)
	if err != nil || rotated.Provisions()[0].Digest == provisions[0].Digest {
		t.Fatal("secret rotation did not advance config hash", err)
	}
}
func TestBackendRuntimeGrantsAndCachedMetadata(t *testing.T) {
	path, file := deploymentFixture(t)
	saveDeployment(t, path, file)
	deployment, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	records := []app.Connection{{ID: "connection", PluginID: "example.fixture", Key: "one", Revision: "1", Enabled: true}}
	runtime := NewRuntime(t.Context(), deployment, records)
	t.Cleanup(runtime.Close)
	deadline := time.Now().Add(10 * time.Second)
	for {
		result, err := runtime.Query(t.Context(), "example.fixture", "connection", "fixture.read", json.RawMessage(`{}`))
		if err == nil {
			if !strings.Contains(string(result.Data), "safe-data") {
				t.Fatal("invalid data")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("backend did not become available", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	state, grants := runtime.CachedStatus("example.fixture", "connection")
	if state.AppliedRevision == nil || *state.AppliedRevision != "1" || len(grants) != 1 {
		t.Fatal("cache did not report configured metadata")
	}
	if _, err := runtime.Query(t.Context(), "example.fixture", "connection", "not-granted", json.RawMessage(`{}`)); app.ErrorCode(err) != "not_found" {
		t.Fatal("unknown operation accepted")
	}
	if _, err := runtime.Query(t.Context(), "example.fixture", "connection", "fixture.read", json.RawMessage(`{"extra":true}`)); app.ErrorCode(err) != "invalid_argument" {
		t.Fatal("input schema not validated")
	}
	runtime.mu.Lock()
	runtime.plugins["example.fixture"].connections["connection"].config.grants = nil
	runtime.mu.Unlock()
	if _, err := runtime.Query(t.Context(), "example.fixture", "connection", "fixture.read", json.RawMessage(`{}`)); app.ErrorCode(err) != "permission_denied" {
		t.Fatal("direct call bypassed grant")
	}
	runtime.Close()
	start := time.Now()
	for range 100 {
		state, _ := runtime.CachedStatus("example.fixture", "connection")
		if state.State != "unavailable" {
			t.Fatal("dead child appears available")
		}
	}
	if time.Since(start) > time.Second {
		t.Fatal("cached metadata invokes child")
	}
}

func TestRejectedConfigureHasNoAppliedRevisionOrGrants(t *testing.T) {
	path, file := deploymentFixture(t)
	tokenPath := filepath.Join(filepath.Dir(path), "token")
	if err := os.WriteFile(tokenPath, []byte("wrong-token"), 0600); err != nil {
		t.Fatal(err)
	}
	saveDeployment(t, path, file)
	deployment, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(t.Context(), deployment, []app.Connection{{ID: "connection", PluginID: "example.fixture", Key: "one", Revision: "7", Enabled: true}})
	defer runtime.Close()
	deadline := time.Now().Add(10 * time.Second)
	for {
		state, grants := runtime.CachedStatus("example.fixture", "connection")
		if state.AppliedRevision != nil || state.Ready || len(grants) != 0 {
			t.Fatal("rejected/starting configuration claimed readiness or applied revision")
		}
		if state.ConfigurationError != nil {
			if state.ConfigurationError.Code != "invalid_argument" || state.LastConfigureAttemptAt == nil || state.ObservedAt != nil || state.LastAttemptAt != nil || state.SourceError != nil {
				t.Fatal("Configure failure metadata is missing or impersonates source collection")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Configure failure was discarded")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func TestCredentialLoaderRejectsUnusableBearerTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	for _, value := range []string{strings.Repeat("x", 32) + " space", strings.Repeat("x", 32) + "\ttab", strings.Repeat("x", 32) + "\x01control", strings.Repeat("x", 32) + "\x7fcontrol"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCredential(path); err == nil {
			t.Fatal("API accepted unusable Bearer credential")
		}
	}
}

func TestInvalidInstallationAndConnectionLeaveValidEntriesUsable(t *testing.T) {
	path, file := deploymentFixture(t)
	badConnection := file.Installations[0].Connections[0]
	badConnection.Key = "bad-source"
	badConnection.Secrets = nil
	file.Installations[0].Connections = append(file.Installations[0].Connections, badConnection)
	badInstallation := file.Installations[0]
	badInstallation.PackageDigest = strings.Repeat("0", 64)
	file.Installations = append(file.Installations, badInstallation)
	saveDeployment(t, path, file)
	deployment, err := Load(path)
	if err != nil || len(deployment.Provisions()) != 1 || len(deployment.Problems()) != 2 {
		t.Fatal("invalid optional entries stopped valid installation", err)
	}
	if deployment.Provisions()[0].Key != "one" {
		t.Fatal("valid connection was replaced")
	}
	public, _ := json.Marshal(deployment.Problems())
	for _, private := range []string{file.Installations[0].Package, "declared-only", "token"} {
		if strings.Contains(string(public), private) {
			t.Fatal("problem exposes private deployment values")
		}
	}
}
