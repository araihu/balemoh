package bff

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	sdkui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/balemoh/ui/internal/view"
)

func TestNotificationIconsUseGrantedCatalogWithoutPluginCalls(t *testing.T) {
	id := strings.Repeat("a", 64)
	registry := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) {
		m.Capabilities.Optional = append(m.Capabilities.Optional, sdkui.ServiceCatalogCapability)
	})
	entry := registry.Connection("example.fixture", notificationClientTestID)
	entry.Installation.ServiceIDs = []string{id}
	registry.Close() // Retained inbox identity does not depend on a running child.
	fixture := &notificationFixture{page: view.NotificationsPage{Items: []view.Notification{
		{ID: "allowed", Title: "Original alert", SourceKind: "plugin", ProducerID: "example.fixture", ProducerConnectionID: notificationClientTestID, ServiceID: id},
		{ID: "denied", Title: "Other connection", SourceKind: "plugin", ProducerID: "example.fixture", ProducerConnectionID: "00000000-0000-4000-8000-000000000099", ServiceID: id},
	}}}
	icon := "selfhst:pi-hole"
	fixture.staging = []api.ServiceCandidate{{Id: id, DisplayName: "Pi-hole", Status: "live", Icon: &icon}}
	handler, err := NewWithPlugins(fixture, time.Second, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/notifications", "/notifications/tray"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		body := response.Body.String()
		if response.Code != 200 || strings.Count(body, `data-service-id="`+id+`"`) != 1 || !strings.Contains(body, `title="Pi-hole"`) {
			t.Fatal("lost service identity or crossed connection grant", path, response.Code)
		}
	}
	if fixture.reads.Load() != 0 || fixture.page.Items[0].ServiceName != "" {
		t.Fatal("render modified retained state")
	}
	calls, _ := filepath.Glob(filepath.Join(entry.Package.Root, "called-*"))
	if len(calls) != 0 {
		t.Fatal("inbox invoked child", calls)
	}
	fixture.staging[0].Status = "hidden"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/notifications", nil))
	if strings.Contains(response.Body.String(), `data-service-id=`) {
		t.Fatal("hidden service leaked")
	}
}
