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

func TestClearConfirmationCoalescesQueuedSubmissions(t *testing.T) {
	var b bytes.Buffer
	if err := NotificationClearForm("", true).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	if !strings.Contains(html, `hx-sync="#balemoh-notifications:queue last"`) || strings.Contains(html, "queue all") {
		t.Fatal("destructive clear must coalesce while a refresh owns the lock")
	}
	b.Reset()
	if err := NotificationClearForm("", false).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "hx-post") || strings.Contains(b.String(), "click.prevent") {
		t.Fatal("native confirmation must retain normal POST/cancel navigation")
	}
}

func TestNotificationGroupingUsesReceiptDayAndUniqueTrayTargets(t *testing.T) {
	now := time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC)
	items := []Notification{{ID: "today", ReceivedAt: now}, {ID: "yesterday", ReceivedAt: now.Add(-2 * time.Hour)}, {ID: "older", ReceivedAt: now.Add(-3 * 24 * time.Hour)}}
	groups := notificationGroups(items, now)
	if len(groups) != 3 || groups[0].Label != "Today · UTC" || groups[1].Label != "Yesterday · UTC" || groups[2].Label != "23 Oct 2026 · UTC" {
		t.Fatalf("groups=%#v", groups)
	}
	if notificationReadTarget("n", "tray") == notificationReadTarget("n", "/notifications") {
		t.Fatal("tray and page read controls share IDs")
	}
	var b bytes.Buffer
	if err := NotificationList(NotificationsPage{UnreadOnly: true, RetentionDays: 7}, true).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "No unread notifications.") || !strings.Contains(b.String(), "7 days") {
		t.Fatal("empty unread state/retention missing")
	}
}

func TestMarkAllCoalescesQueuedSubmissions(t *testing.T) {
	var b bytes.Buffer
	if err := NotificationTray().Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	for _, want := range []string{`hx-sync:inherited="#balemoh-notifications:queue last"`, `hx-request:inherited="timeout:4s"`, `hx-include:inherited="#notification-filter"`, `id="notification-mark-all"`, `hx-post="/notifications/read-all"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("tray mark-all missing queue admission contract %s", want)
		}
	}
	b.Reset()
	if err := NotificationMarkAllForm("/notifications?unread=true").Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `hx-sync="#balemoh-notifications:queue last"`) || strings.Contains(b.String(), "queue all") {
		t.Fatal("page mark-all must coalesce while a refresh owns the lock")
	}
}
