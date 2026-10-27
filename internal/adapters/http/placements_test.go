package http

import (
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	app "github.com/araihu/balemoh/internal/application/plugins"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

type placementRuntime struct{ pluginRuntimeFixture }

func (placementRuntime) Definitions() []app.Definition { return []app.Definition{} }
func (placementRuntime) ValidateOptions(_, _ string, raw json.RawMessage) error {
	if !json.Valid(raw) {
		return app.Fail("invalid_argument")
	}
	return nil
}
func TestPlacementHTTPRevisionContract(t *testing.T) {
	db, e := sqlite.Open(t.Context(), t.TempDir()+"/api.db")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = sqlite.RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	store := sqlite.NewPluginStore(db)
	cs, e := store.Reconcile(t.Context(), []app.Provision{{PluginID: "example.plugin", Key: "main", Name: "Fixture", Digest: "config", ManifestDigest: "manifest", Version: "0.0.1"}})
	if e != nil {
		t.Fatal(e)
	}
	token := strings.Repeat("a", 32)
	service := app.New(store, placementRuntime{})
	handler := MountPlugins(http.NotFoundHandler(), service, token)
	id := uuid.NewV4().String()
	path := "/api/v1/plugin-widgets/" + id
	body := `{"pluginId":"example.plugin","connectionId":"` + cs[0].ID + `","contributionId":"summary","options":{"b":2,"a":1},"enabled":true}`
	send := func(method, body, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		if etag != "" {
			r.Header.Set("If-Match", etag)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := send("PUT", body, ""); w.Code != 201 || w.Header().Get("ETag") != `"1"` {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	if w := send("PUT", strings.Replace(body, `"b":2,"a":1`, `"a":1,"b":2`, 1), ""); w.Code != 200 {
		t.Fatalf("replay %d %s", w.Code, w.Body)
	}
	changed := strings.Replace(body, `"enabled":true`, `"enabled":false`, 1)
	for _, tt := range []struct {
		method, body, etag string
		status             int
	}{{"PUT", changed, "", 409}, {"PUT", changed, `"1"`, 200}, {"PUT", changed, `"1"`, 412}, {"DELETE", "", "", 428}, {"DELETE", "", `"1"`, 412}, {"DELETE", "", `"2"`, 204}, {"DELETE", "", `"2"`, 404}} {
		if w := send(tt.method, tt.body, tt.etag); w.Code != tt.status {
			t.Fatalf("%s wanted %d got %d %s", tt.method, tt.status, w.Code, w.Body)
		}
	}
	if rows, e := service.Placements(context.Background()); e != nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
}
