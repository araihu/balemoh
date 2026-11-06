package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type actionHost struct {
	ui.NoDataServices
	calls int
}

func (h *actionHost) Action(_ context.Context, _ string, _ json.RawMessage) (backend.ActionResult, error) {
	h.calls++
	return backend.ActionResult{Data: json.RawMessage(`{"accepted":true}`)}, nil
}

func TestRealUICallbackActionsRequirePostPage(t *testing.T) {
	p := backendPackage(t)
	p.Manifest.Pages[0].Methods = append(p.Manifest.Pages[0].Methods, "POST")
	p.Manifest.Actions = []manifest.Query{{ID: "fixture.request", InputSchema: "schema.json", OutputSchema: "action-output.json", Capability: "data.action/v1"}}
	p.Manifest.Capabilities.Optional = append(p.Manifest.Capabilities.Optional, "data.action/v1")
	if err := os.WriteFile(filepath.Join(p.Root, "action-output.json"), []byte(`{"type":"object","properties":{"accepted":{"type":"boolean"}},"required":["accepted"],"additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	p = saveActionPackage(t, p)
	child, err := pluginruntime.Open(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	host := &actionHost{}
	r := ui.Request{ContributionType: "page", ContributionID: "current", Mode: "page", Method: "GET", Query: map[string][]string{"action": {"fixture.request"}}}
	if _, err = child.HandleWithServices(t.Context(), r, host); err == nil || host.calls != 0 {
		t.Fatal("GET action reached host", err)
	}
	r.Method = "POST"
	response, err := child.HandleWithServices(t.Context(), r, host)
	if err != nil || !strings.Contains(string(response.HTML), `"accepted":true`) || host.calls != 1 {
		t.Fatal("POST action callback failed", err)
	}
	r.ContributionType = "widget"
	r.Mode = "widget"
	r.ContributionID = "summary"
	if _, err = child.HandleWithServices(t.Context(), r, host); err == nil || host.calls != 1 {
		t.Fatal("widget action reached host", err)
	}
}

func TestRealBackendActionIsSeparateFromQuery(t *testing.T) {
	p := backendPackage(t)
	p.Manifest.Actions = []manifest.Query{{ID: "fixture.request", InputSchema: "schema.json", OutputSchema: "output.json", Capability: "data.action/v1"}}
	p.Manifest.Capabilities.Optional = append(p.Manifest.Capabilities.Optional, "data.action/v1")
	p = saveActionPackage(t, p)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	_, err = child.Configure(t.Context(), backend.Config{ConnectionID: "connection", Revision: "1", Config: json.RawMessage(`{}`), Secrets: map[string]string{"token": "declared-only"}})
	if err != nil {
		t.Fatal(err)
	}
	r := backend.Request{ConnectionID: "connection", OperationID: "fixture.request", Input: json.RawMessage(`{}`)}
	result, err := child.Action(t.Context(), r)
	if err != nil || !strings.Contains(string(result.Data), "safe-data") {
		t.Fatal("action failed", err)
	}
	if _, err = child.Query(t.Context(), r); err == nil {
		t.Fatal("action was exposed as a query")
	}
	r.Input = json.RawMessage(`null`)
	if _, err = child.Action(t.Context(), r); backend.Code(err) != "invalid_argument" {
		t.Fatal("malformed action input accepted", err)
	}
}

func saveActionPackage(t *testing.T, p *manifest.Package) *manifest.Package {
	t.Helper()
	data, _ := json.Marshal(p.Manifest)
	if e := os.WriteFile(filepath.Join(p.Root, "plugin.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
	out, e := manifest.Load(p.Root)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
