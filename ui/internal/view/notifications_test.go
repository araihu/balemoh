package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestNotificationsEscapeProducerTextAndMarkExternalLinks(t *testing.T) {
	p := NotificationsPage{Items: []Notification{{ID: "test", Title: `<img src=x onerror=alert(1)>`, Body: `<script>bad()</script>`, Source: `<svg onload=bad()>`, Severity: "warning", ReceivedAt: time.Now(), HasDestination: true, External: true}}}
	var b bytes.Buffer
	if err := NotificationList(p, true).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	for _, bad := range []string{"<img src=x", "<script>bad", "<svg onload"} {
		if strings.Contains(html, bad) {
			t.Fatalf("unescaped producer text: %s", bad)
		}
	}
	for _, want := range []string{"&lt;img", "&lt;script&gt;", `rel="noopener noreferrer"`, `target="_blank"`, `/notifications/test/open`, "Mark read"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
}
func TestNotificationUnavailableIsDistinctFromZero(t *testing.T) {
	for _, count := range []int{0, 103} {
		var b bytes.Buffer
		if err := NotificationSummary(count, true).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		if count == 103 && !strings.Contains(b.String(), "99+") {
			t.Fatal("uncapped badge")
		}
		if !strings.Contains(b.String(), `data-notification-count="`) {
			t.Fatal("accessible count omitted")
		}
	}
	var b bytes.Buffer
	if err := NotificationSummary(0, false).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), ">0<") || !strings.Contains(b.String(), ">!</span>") {
		t.Fatal("unavailable rendered as zero")
	}
}
