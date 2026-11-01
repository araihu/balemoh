package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
)

func TestActionCallbackRequiresPostGrantAndBoundConnection(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/v1/plugins/example.fixture/connections/current/actions/fixture.request" {
			t.Error("action escaped scoped connection")
		}
		_, _ = w.Write([]byte(`{"data":{"accepted":true}}`))
	}))
	defer server.Close()
	source, e := NewSource(server.URL, tokenFixture(t), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	if e = os.WriteFile(filepath.Join(root, "input.json"), []byte(`{"type":"object","additionalProperties":false}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "output.json"), []byte(`{"type":"object","required":["accepted"],"properties":{"accepted":{"type":"boolean"}},"additionalProperties":false}`), 0600); e != nil {
		t.Fatal(e)
	}
	entry := &Entry{Installation: Installation{ConnectionID: "current"}, Package: &manifest.Package{Root: root, Manifest: manifest.Manifest{ID: "example.fixture", Actions: []manifest.Query{{ID: "fixture.request", InputSchema: "input.json", OutputSchema: "output.json", Capability: "data.action/v1"}}}}}
	host := scopedServices{source: source, entry: entry, grants: map[string]bool{"fixture.request": true}}
	if _, e = host.Action(t.Context(), "fixture.request", json.RawMessage(`{}`)); backend.Code(e) != "permission_denied" || calls.Load() != 0 {
		t.Fatal("GET callback performed an action")
	}
	host.allowActions = true
	host.grants = map[string]bool{}
	if _, e = host.Action(t.Context(), "fixture.request", json.RawMessage(`{}`)); backend.Code(e) != "permission_denied" {
		t.Fatal("ungranted action reached API")
	}
	host.grants["fixture.request"] = true
	if _, e = host.Action(t.Context(), "fixture.request", json.RawMessage(`{"foreign":true}`)); backend.Code(e) != "invalid_argument" {
		t.Fatal("malformed action reached API")
	}
	result, e := host.Action(context.Background(), "fixture.request", json.RawMessage(`{}`))
	if e != nil || !strings.Contains(string(result.Data), "accepted") || calls.Load() != 1 {
		t.Fatal("scoped action failed", e, calls.Load())
	}
}
