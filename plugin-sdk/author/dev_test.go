package author

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
)

func TestPreviewStateInvalidatesRenderMetadataAndRetainsObservation(t *testing.T) {
	p := newPreview(&manifest.Package{Manifest: manifest.Manifest{ID: "example.fixture"}}, "preview-token", "http://127.0.0.1:1", nil)
	query := func() map[string]any {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/v1/plugins/example.fixture/connections/"+previewConnection+"/queries/example.read", nil)
		r.Header.Set("Authorization", "Bearer preview-token")
		w := httptest.NewRecorder()
		p.ServeHTTP(w, r)
		var value map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil {
			t.Fatalf("query status %d", w.Code)
		}
		return value
	}
	first := query()
	p.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/preview/stale", nil))
	stale := query()
	if stale["state"] != "stale" || first["observationRevision"] == stale["observationRevision"] || first["observedAt"] != stale["observedAt"] {
		t.Fatal("state change did not invalidate or changed retained time")
	}
	r := httptest.NewRequest("GET", "/api/v1/plugins", nil)
	r.Header.Set("Authorization", "Bearer preview-token")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	var catalog struct {
		Plugins []struct {
			Connections []struct {
				Revision string `json:"observationRevision"`
			}
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &catalog) != nil || catalog.Plugins[0].Connections[0].Revision != stale["observationRevision"] {
		t.Fatal("descriptor cache key missed preview change")
	}
	p.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/preview/empty", nil))
	empty := query()
	if empty["state"] != "fresh" || empty["observationRevision"] == stale["observationRevision"] {
		t.Fatal("empty preview did not advance")
	}
	var data struct{ Message string }
	raw, _ := json.Marshal(empty["data"])
	_ = json.Unmarshal(raw, &data)
	if data.Message != "" {
		t.Fatal("empty fixture had content")
	}
	if _, err := time.Parse(time.RFC3339Nano, empty["observedAt"].(string)); err != nil {
		t.Fatal(err)
	}
}
