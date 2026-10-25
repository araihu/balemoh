package ui_test

import (
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func TestPageURLAndPlacementIDs(t *testing.T) {
	m := manifest.Manifest{ID: "example.fixture", Pages: []manifest.Page{{ID: "detail", Path: "/current/{fingerprint}"}}}
	r := ui.Request{BasePath: "/lab", ConnectionID: "connection", DOMNamespace: "placement-one"}
	href, err := r.PageURL(m, "detail", map[string]string{"fingerprint": "alert 1"})
	if err != nil || !strings.HasPrefix(href, "/lab/plugins/example.fixture/connections/connection/pages/current/alert%201") {
		t.Fatalf("page URL %q: %v", href, err)
	}
	for _, bad := range []string{"../x", "a/b", "%2f", ".."} {
		if _, err := r.PageURL(m, "detail", map[string]string{"fingerprint": bad}); err == nil {
			t.Fatalf("unsafe parameter %q", bad)
		}
	}
	first := r.DOMID("summary")
	if first != r.DOMID("summary") {
		t.Fatal("unstable ID")
	}
	r.PlacementID = "placement-two"
	if first == r.DOMID("summary") {
		t.Fatal("placement ID collision")
	}
}
