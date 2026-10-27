package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "github.com/araihu/balemoh/internal/application/plugins"
)

type pluginStoreFixture struct{}

func (pluginStoreFixture) Reconcile(context.Context, []app.Provision) ([]app.Connection, error) {
	return nil, nil
}
func (pluginStoreFixture) List(context.Context) ([]app.Connection, error) {
	return []app.Connection{{ID: "connection", PluginID: "example.plugin", Key: "lab", Name: "Lab", Revision: "1", Enabled: true, ManifestDigest: "public-digest", Version: "0.0.1", Digest: "private-config-hash"}}, nil
}
func (pluginStoreFixture) Get(_ context.Context, p, id string) (app.Connection, error) {
	if p != "example.plugin" || id != "connection" {
		return app.Connection{}, app.Fail("not_found")
	}
	return app.Connection{Enabled: true}, nil
}
func (pluginStoreFixture) Delete(context.Context, string) error { return nil }

type pluginRuntimeFixture struct{}

func (pluginRuntimeFixture) InstallationProblems() []app.InstallationProblem {
	return []app.InstallationProblem{}
}
func (pluginRuntimeFixture) CachedStatus(string, string) (app.Status, []string) {
	revision := "1"
	return app.Status{State: "unavailable", AppliedRevision: &revision, ObservationRevision: &revision}, []string{"allowed"}
}
func (pluginRuntimeFixture) Query(_ context.Context, _, _, q string, input json.RawMessage) (app.Result, error) {
	if q != "allowed" {
		return app.Result{}, app.Fail("permission_denied")
	}
	if string(input) != "{}" {
		return app.Result{}, app.Fail("invalid_argument")
	}
	return app.Result{Data: json.RawMessage(`{"items":[]}`), State: "unavailable", ObservationRevision: "1"}, nil
}
func TestPluginAPIAuthenticationGrantsAndBounds(t *testing.T) {
	token := strings.Repeat("private-token", 4)
	h := MountPlugins(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }), app.New(pluginStoreFixture{}, pluginRuntimeFixture{}), token)
	route := "/api/v1/plugins/example.plugin/connections/connection/queries/"
	tests := []struct {
		name, method, path, body, credential string
		status                               int
	}{
		{"missing credential", "GET", "/api/v1/plugins", "", "", 401}, {"wrong credential", "GET", "/api/v1/plugins", "", "wrong", 401},
		{"descriptor", "GET", "/api/v1/plugins", "", token, 200}, {"query", "POST", route + "allowed", `{"input":{}}`, token, 200},
		{"direct bypass", "POST", route + "denied", `{"input":{}}`, token, 403},
		{"cross connection", "POST", strings.Replace(route, "/connection/", "/other/", 1) + "allowed", `{"input":{}}`, token, 404},
		{"unknown envelope field", "POST", route + "allowed", `{"input":{},"secret":"bad"}`, token, 400},
		{"multiple values", "POST", route + "allowed", `{"input":{}} {}`, token, 400},
		{"oversized", "POST", route + "allowed", `{"input":{"x":"` + strings.Repeat("x", 65<<10) + `"}}`, token, 413},
		{"oversized trailing", "POST", route + "allowed", `{"input":{}}` + strings.Repeat(" ", 66<<10), token, 413},
		{"legacy unchanged", "GET", "/legacy", "", "", 204},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.credential != "" {
				r.Header.Set("Authorization", "Bearer "+tt.credential)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status %d, expected %d: %s", w.Code, tt.status, w.Body.String())
			}
			for _, private := range []string{token, "private-config-hash", "bearer-token"} {
				if strings.Contains(w.Body.String(), private) {
					t.Fatal("private data in response")
				}
			}
		})
	}
}
