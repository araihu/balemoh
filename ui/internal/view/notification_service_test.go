package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNotificationPreservesProducerAlongsideServiceContext(t *testing.T) {
	for _, tray := range []bool{false, true} {
		item := Notification{ID: "notification", Title: "Backup warning", ProducerIconURL: "/alertmanager.png", ServiceID: strings.Repeat("a", 64), ServiceName: "Pi-hole", ServiceIcon: LibraryIcon{URL: "/pi-hole.png"}}
		var out bytes.Buffer
		if err := NotificationItem(item, NotificationsPage{}, tray).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		if !strings.Contains(body, `class="balemoh-notification-source-icon"><img src="/alertmanager.png"`) || !strings.Contains(body, `src="/pi-hole.png"`) || !strings.Contains(body, `<span>Pi-hole</span>`) {
			t.Fatal("producer icon and visible service identity must coexist")
		}
		item.ServiceName = ""
		out.Reset()
		if err := NotificationItem(item, NotificationsPage{}, tray).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), `data-service-id=`) || !strings.Contains(out.String(), `src="/alertmanager.png"`) {
			t.Fatal("unresolved service must not replace producer or render context")
		}
	}
}
