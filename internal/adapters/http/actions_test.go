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

type actionRuntimeFixture struct {
	pluginRuntimeFixture
	calls int
}

func (f *actionRuntimeFixture) GrantedActions(string, string) []string {
	return []string{"fixture.request"}
}
func (f *actionRuntimeFixture) Action(_ context.Context, _, _, operation string, input json.RawMessage) (app.ActionResult, error) {
	if operation != "fixture.request" {
		return app.ActionResult{}, app.Fail("permission_denied")
	}
	if string(input) != "{}" {
		return app.ActionResult{}, app.Fail("invalid_argument")
	}
	f.calls++
	return app.ActionResult{Data: json.RawMessage(`{"requestId":"one"}`)}, nil
}

func TestActionAPIAuthenticationScopeAndReadOnlyQueries(t *testing.T) {
	token := strings.Repeat("private-token", 4)
	runtime := &actionRuntimeFixture{}
	handler := MountPlugins(http.NotFoundHandler(), app.New(pluginStoreFixture{}, runtime), token, nil)
	for _, test := range []struct {
		path, token, body string
		status, calls     int
	}{
		{"actions/fixture.request", "", `{"input":{}}`, 401, 0},
		{"queries/fixture.request", token, `{"input":{}}`, 403, 0},
		{"actions/denied", token, `{"input":{}}`, 403, 0},
		{"actions/fixture.request", token, `{"input":{},"command":"merge"}`, 400, 0},
		{"actions/fixture.request", token, `{"input":{}}`, 200, 1},
	} {
		r := httptest.NewRequest("POST", "/api/v1/plugins/example.plugin/connections/connection/"+test.path, strings.NewReader(test.body))
		if test.token != "" {
			r.Header.Set("Authorization", "Bearer "+test.token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status || runtime.calls != test.calls {
			t.Fatalf("%s status=%d calls=%d", test.path, w.Code, runtime.calls)
		}
	}
}
