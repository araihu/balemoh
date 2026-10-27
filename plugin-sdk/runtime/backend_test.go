package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func backendPackage(t *testing.T) *manifest.Package {
	t.Helper()
	p := packageFixture(t)
	cmd := exec.Command("go", "build", "-o", filepath.Join(p.Root, "backend"), "./testdata/backend")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build backend: %v: %s", err, out)
	}
	p.Manifest.Targets[runtime.GOOS+"/"+runtime.GOARCH] = manifest.Target{UI: "ui", Backend: "backend"}
	p.Manifest.Secrets = []manifest.Secret{{ID: "token", Required: true}}
	p.Manifest.Capabilities.Required = append(p.Manifest.Capabilities.Required, "data.query/v1")
	for _, id := range []string{"fixture.read", "fixture.wait", "fixture.bad"} {
		p.Manifest.Queries = append(p.Manifest.Queries, manifest.Query{ID: id, InputSchema: "schema.json", OutputSchema: "output.json", Capability: "data.query/v1"})
	}
	data, _ := json.Marshal(p.Manifest)
	if err := os.WriteFile(filepath.Join(p.Root, "plugin.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "output.json"), []byte(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := manifest.Load(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestBackendRPCValidationDeadlineAndScopedCallback(t *testing.T) {
	p := backendPackage(t)
	t.Setenv("BALEMOH_TEST_SECRET", "must-not-inherit")
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	config := backend.Config{ConnectionID: "connection", Revision: "1", Config: json.RawMessage(`{}`), Secrets: map[string]string{"token": "declared-only"}}
	if _, err = child.Configure(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	config.Secrets["undeclared"] = "not-allowed"
	if _, err = child.Configure(t.Context(), config); backend.Code(err) != "invalid_argument" {
		t.Fatal("undeclared secret accepted")
	}
	delete(config.Secrets, "undeclared")
	result, err := child.Query(t.Context(), backend.Request{ConnectionID: "connection", OperationID: "fixture.read", Input: json.RawMessage(`{}`)})
	if err != nil || !strings.Contains(string(result.Data), "safe-data") {
		t.Fatal("real query failed", err)
	}
	for operation, code := range map[string]string{"fixture.bad": "invalid_response", "undeclared": "invalid_argument"} {
		if _, err = child.Query(t.Context(), backend.Request{ConnectionID: "connection", OperationID: operation, Input: json.RawMessage(`{}`)}); backend.Code(err) != code {
			t.Fatalf("operation %s: %v", operation, err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if _, err = child.Query(ctx, backend.Request{ConnectionID: "connection", OperationID: "fixture.wait", Input: json.RawMessage(`{}`)}); backend.Code(err) != "deadline_exceeded" {
		t.Fatal("query ignored parent deadline", err)
	}
	uiChild, err := pluginruntime.Open(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(uiChild.Close)
	services := &callbackFixture{child: child, connection: "connection"}
	request := ui.Request{ConnectionID: "connection", ContributionType: "page", ContributionID: "current", Mode: "page", Method: "GET", Query: map[string][]string{"data": {"fixture.read"}}}
	for range 8 {
		response, err := uiChild.HandleWithServices(t.Context(), request, services)
		if err != nil || !strings.Contains(string(response.HTML), "safe-data") {
			t.Fatal("UI callback failed", err)
		}
	}
	request.Query.Set("data", "fixture.wait")
	ctx2, cancel2 := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel2()
	start := time.Now()
	if _, err = uiChild.HandleWithServices(ctx2, request, services); err == nil || time.Since(start) > time.Second {
		t.Fatal("UI callback ignored deadline", err)
	}
	request.Query.Set("data", "denied")
	if _, err = uiChild.HandleWithServices(t.Context(), request, services); err == nil {
		t.Fatal("callback bypassed grant")
	}
	if err = child.CloseConnection(t.Context(), "connection"); err != nil {
		t.Fatal(err)
	}
	if _, err = child.Status(t.Context(), "connection"); backend.Code(err) != "not_found" {
		t.Fatal("close did not remove connection")
	}
}

type callbackFixture struct {
	child      *pluginruntime.BackendClient
	connection string
}

func (h *callbackFixture) Query(ctx context.Context, q string, input json.RawMessage) (ui.QueryResult, error) {
	if q != "fixture.read" && q != "fixture.wait" {
		return ui.QueryResult{}, backend.Fail("permission_denied")
	}
	return h.child.Query(ctx, backend.Request{ConnectionID: h.connection, OperationID: q, Input: input})
}
func (*callbackFixture) Placement(context.Context) (ui.Placement, error) {
	return ui.Placement{}, backend.Fail("not_found")
}
