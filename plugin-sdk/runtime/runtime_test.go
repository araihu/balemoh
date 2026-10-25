package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func packageFixture(t *testing.T) *manifest.Package {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(root, "ui")
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/ui")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: "example.fixture", Version: "0.0.1", Name: "Fixture", Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Pages: []manifest.Page{{ID: "current", Title: "Fixture page", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{}}}, Navigation: []manifest.Navigation{{ID: "fixture", Title: "Fixture", Page: "current"}}, Widgets: []manifest.Widget{{ID: "summary", Title: "Fixture", OptionsSchema: "schema.json", Capabilities: []string{}}}, Assets: []manifest.Asset{}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "home.widget/v1", "sidebar.navigation/v1"}, Optional: []string{}}}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema.json"), []byte(`{"type":"object","additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRealChildRenderDeadlineAndEnvironment(t *testing.T) {
	p := packageFixture(t)
	t.Setenv("BALEMOH_TEST_SECRET", "must-not-be-inherited")
	c, err := pluginruntime.Open(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	r := ui.Request{ContributionType: "page", ContributionID: "current", Mode: "page", Method: "GET", ConnectionID: "fixture", DOMNamespace: "first"}
	response, err := c.Handle(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(response.HTML), "Synthetic plugin content") || strings.Contains(string(response.HTML), "must-not-be-inherited") {
		t.Fatalf("unexpected child output: %q", response.HTML)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	r.Query = map[string][]string{"wait": {"true"}}
	if _, err := c.Handle(ctx, r); err == nil {
		t.Fatal("expected deadline failure")
	}
	r.Query = nil
	if _, err := c.Handle(t.Context(), r); err != nil {
		t.Fatalf("child should remain usable after one timeout: %v", err)
	}
	r.Query = map[string][]string{"oversized": {"true"}}
	if _, err := c.Handle(t.Context(), r); err == nil {
		t.Fatal("expected oversized response failure")
	}
	pid := c.PID()
	c.Close()
	c.Close()
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatal("child survived Close")
	}
	if _, err := c.Handle(t.Context(), r); err == nil {
		t.Fatal("closed child should be unavailable")
	}
}

func TestRejectsChildIdentityMismatch(t *testing.T) {
	p := packageFixture(t)
	if err := os.WriteFile(filepath.Join(p.Root, "mismatch"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if c, err := pluginruntime.Open(t.Context(), p); err == nil {
		c.Close()
		t.Fatal("expected identity mismatch")
	}
}

func TestSupervisorRestartsOwnedCrashedChild(t *testing.T) {
	p := packageFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	ready := make(chan *pluginruntime.Client, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		pluginruntime.Supervise(ctx, func(ctx context.Context) (*pluginruntime.Client, error) { return pluginruntime.Open(ctx, p) }, func(c *pluginruntime.Client) { ready <- c }, func(*pluginruntime.Client) {}, nil)
	}()
	var first *pluginruntime.Client
	select {
	case first = <-ready:
	case <-ctx.Done():
		t.Fatal("first child not ready")
	}
	if e := syscall.Kill(first.PID(), syscall.SIGKILL); e != nil {
		t.Fatal(e)
	}
	var second *pluginruntime.Client
	select {
	case second = <-ready:
	case <-ctx.Done():
		t.Fatal("child did not restart")
	}
	if first.PID() == second.PID() {
		t.Fatal("child identity reused")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown scheduled or retained a child")
	}
	if e := syscall.Kill(second.PID(), 0); e == nil {
		t.Fatal("owned child survived cancellation")
	}
}
