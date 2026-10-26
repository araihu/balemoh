package bff

import (
	"context"
	"errors"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/view"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type notificationFixture struct {
	fakeCatalog
	calls, reads, clears atomic.Int32
	blocked              bool
	failure              error
	page                 view.NotificationsPage
	record               NotificationRecord
}

func (f *notificationFixture) Notifications(ctx context.Context, _ string, _ bool, _ int) (view.NotificationsPage, error) {
	f.calls.Add(1)
	if f.blocked {
		<-ctx.Done()
		return view.NotificationsPage{}, ctx.Err()
	}
	return f.page, f.failure
}
func (f *notificationFixture) Notification(context.Context, string) (NotificationRecord, error) {
	return f.record, f.failure
}
func (f *notificationFixture) MarkNotificationRead(context.Context, string) error {
	f.reads.Add(1)
	return f.failure
}
func (f *notificationFixture) ClearNotifications(context.Context) error {
	f.clears.Add(1)
	return f.failure
}
func TestNotificationClientCannotBlockBaseShell(t *testing.T) {
	f := &notificationFixture{blocked: true}
	h, _ := New(f, 25*time.Millisecond)
	start := time.Now()
	w := request(t, h, "GET", "/", " ")
	if w.Code != 200 || time.Since(start) > time.Second || f.calls.Load() != 0 {
		t.Fatalf("base render consulted notifications: %d calls, status %d", f.calls.Load(), w.Code)
	}
	if !strings.Contains(w.Body.String(), `id="notification-summary"`) {
		t.Fatal("missing async loading target")
	}
	w = request(t, h, "GET", "/notifications/summary", "")
	if w.Code != 503 || f.calls.Load() != 1 || strings.Contains(w.Body.String(), "<html") || strings.Contains(w.Body.String(), ">0<") {
		t.Fatal("notification timeout escaped its target")
	}
	f.blocked = false
	f.failure = errors.New("storage unavailable")
	w = request(t, h, "GET", "/", " ")
	if w.Code != 200 || f.calls.Load() != 1 {
		t.Fatal("unavailable notifications affected base shell")
	}
}
func TestNotificationFragmentsNeverInvokePlugin(t *testing.T) {
	r := previewRegistry(t, manifest.UIProfile)
	root := r.Entries()[0].Package.Root
	if err := os.WriteFile(filepath.Join(root, "slow"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f := &notificationFixture{}
	h, err := NewWithPlugins(f, 100*time.Millisecond, nil, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/notifications", "/notifications/summary", "/notifications/tray"} {
		start := time.Now()
		w := request(t, h, "GET", path, "")
		if w.Code != 200 || time.Since(start) > time.Second {
			t.Fatalf("%s blocked", path)
		}
	}
	if calls, _ := filepath.Glob(filepath.Join(root, "called-*")); len(calls) != 0 {
		t.Fatalf("notification requests invoked plugin: %v", calls)
	}
	r.Close()
	if w := request(t, h, "GET", "/notifications/summary", ""); w.Code != 200 {
		t.Fatal("stopped plugins broke notifications")
	}
}
func TestNotificationReadProtectionAndAcknowledgement(t *testing.T) {
	f := &notificationFixture{}
	h, _ := New(f, time.Second)
	id := "00000000-0000-4000-8000-000000000001"
	path := "/notifications/" + id + "/read"
	post := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(url.Values{"returnTo": {"tray"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request-Type", "partial")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post("https://evil.example"); w.Code != 403 || f.reads.Load() != 0 {
		t.Fatal("cross-origin read mutation accepted")
	}
	f.failure = errors.New("write failed")
	if w := post(""); w.Code != 503 || strings.Contains(w.Body.String(), ">Read</span>") || !strings.Contains(w.Body.String(), "Mark read") {
		t.Fatal("failed read action claims success")
	}
	f.failure = nil
	if w := post(""); w.Code != 200 || !strings.Contains(w.Body.String(), ">Read</span>") || strings.Contains(w.Body.String(), "<html") {
		t.Fatal("acknowledged read action not scoped")
	}
	_ = http.StatusOK
}

func TestNotificationClearConfirmationProtectionAndResult(t *testing.T) {
	f := &notificationFixture{}
	h, _ := New(f, time.Second)
	post := func(body, origin string, partial bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/notifications/clear", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if partial {
			r.Header.Set("HX-Request-Type", "partial")
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post("confirm=true", "https://attacker.test", true); w.Code != 403 || f.clears.Load() != 0 {
		t.Fatal("cross-origin clear reached client")
	}
	if w := post("", "", true); w.Code != 400 || f.clears.Load() != 0 || !strings.Contains(w.Body.String(), "Confirm permanent deletion") {
		t.Fatal("missing confirmation accepted")
	}
	f.failure = errors.New("storage unavailable")
	w := post("confirm=true", "", true)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "Clear not confirmed") || strings.Contains(w.Body.String(), "No notifications.") {
		t.Fatal("failed clear rendered empty inbox")
	}
	f.failure = nil
	// A message delivered after DELETE must be rendered, not overwritten with zero.
	f.page = view.NotificationsPage{UnreadCount: 1, Items: []view.Notification{{ID: "new", Title: "After clear", ReceivedAt: time.Now()}}}
	w = post("confirm=true", "", true)
	if w.Code != 200 || w.Header().Get("HX-Retarget") != "#notification-inbox" || !strings.Contains(w.Body.String(), "After clear") || !strings.Contains(w.Body.String(), "data-notifications-cleared") {
		t.Fatalf("clear response %d: %s", w.Code, w.Body.String())
	}
	w = post("confirm=true", "", false)
	if w.Code != 303 || w.Header().Get("Location") != "/notifications" {
		t.Fatal("native clear lacks PRG")
	}
	for _, path := range []string{"/notifications/clear", "/notifications"} {
		w = request(t, h, "GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Clear all notifications") {
			t.Fatalf("missing clear control: %s", path)
		}
	}
	if f.clears.Load() != 3 {
		t.Fatalf("clear effect count %d", f.clears.Load())
	}
}
