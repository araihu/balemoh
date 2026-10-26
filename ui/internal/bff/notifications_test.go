package bff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/ui/internal/view"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type notificationFixture struct {
	fakeCatalog
	calls, reads, clears, bulkReads atomic.Int32
	blocked                         bool
	failure                         error
	page                            view.NotificationsPage
	record                          NotificationRecord
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
func (f *notificationFixture) MarkAllNotificationsRead(context.Context) error {
	f.bulkReads.Add(1)
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
	if w := post(""); w.Code != 200 || w.Header().Get("HX-Retarget") != "#notification-tray" || strings.Contains(w.Body.String(), "<html") {
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

func TestBulkNotificationReadIsSeparateProtectedAndScoped(t *testing.T) {
	f := &notificationFixture{page: view.NotificationsPage{RetentionDays: 7, UnreadCount: 1, Items: []view.Notification{{ID: "new", Title: "After read", ReceivedAt: time.Now()}}}}
	h, _ := New(f, time.Second)
	post := func(origin, body string, partial bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/notifications/read-all", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if partial {
			r.Header.Set("HX-Request-Type", "partial")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post("https://bad.example", "returnTo=tray", true); w.Code != 403 || f.bulkReads.Load() != 0 {
		t.Fatal("cross-origin bulk read accepted")
	}
	w := post("", "returnTo=tray&unread=true&returnPage=%2Fnotifications%3Funread%3Dtrue", true)
	if w.Code != 200 || w.Header().Get("HX-Retarget") != "#notification-tray" || !strings.Contains(w.Body.String(), "After read") || !strings.Contains(w.Body.String(), "7 days") || !strings.Contains(w.Body.String(), `id="notification-inbox"`) || f.bulkReads.Load() != 1 || f.clears.Load() != 0 {
		t.Fatalf("bulk scope/effects %d %s", w.Code, w.Body.String())
	}
	f.failure = errors.New("timeout")
	w = post("", "returnTo=tray", true)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "Read state not confirmed") || strings.Contains(w.Body.String(), "No notifications.") {
		t.Fatal("bulk failure claims success")
	}
	f.failure = nil
	w = post("", "returnTo=%2Fnotifications%3Funread%3Dtrue", false)
	if w.Code != 303 || w.Header().Get("Location") != "/notifications?unread=true" {
		t.Fatal("missing native bulk PRG")
	}
}

func TestNotificationUsesProducerPageIconWithoutPluginRendering(t *testing.T) {
	for _, tc := range []struct {
		name, symbol, kind, connection, page, want string
		nested, precedingIcon, logo                bool
	}{
		{name: "declared", symbol: "hi-16-solid-bell", want: "hi-16-solid-bell"},
		{name: "producer logo", symbol: "hi-16-solid-bell", logo: true, want: "logo.png"},
		{name: "producer logo without destination", symbol: "hi-16-solid-bell", logo: true, page: "absent", want: "logo.png"},
		{name: "logo other connection", logo: true, connection: "00000000-0000-4000-8000-000000000099", want: "hi-16-solid-cube"},
		{name: "logo external source", logo: true, kind: "external", want: "hi-16-solid-arrow-down-tray"},
		{name: "nested", symbol: "hi-16-solid-bell", want: "hi-16-solid-bell", nested: true},
		{name: "detail without navigation", symbol: "hi-16-solid-bell", page: "detail", want: "hi-16-solid-bell"},
		{name: "detail with nested producer", symbol: "hi-16-solid-bell", page: "detail", want: "hi-16-solid-bell", nested: true},
		{name: "page icon takes precedence", symbol: "hi-16-solid-bell", want: "hi-16-solid-bell", precedingIcon: true},
		{name: "detail unknown icon", symbol: "future-icon", page: "detail", want: "hi-16-solid-cube"},
		{name: "unknown", symbol: "future-icon", want: "hi-16-solid-cube"},
		{name: "URL rejected", symbol: "https://invalid.example/icon.svg", want: "hi-16-solid-cube"},
		{name: "missing icon", want: "hi-16-solid-cube"},
		{name: "other connection", symbol: "hi-16-solid-bell", connection: "00000000-0000-4000-8000-000000000099", want: "hi-16-solid-cube"},
		{name: "other page", symbol: "hi-16-solid-bell", page: "absent", want: "hi-16-solid-cube"},
		{name: "external producer", symbol: "hi-16-solid-bell", kind: "external", want: "hi-16-solid-arrow-down-tray"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logo bytes.Buffer
			if err := png.Encode(&logo, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
				t.Fatal(err)
			}
			registry := configuredPreviewRegistry(t, manifest.UIProfile, func(m *manifest.Manifest) {
				m.Targets[runtime.GOOS+"/"+runtime.GOARCH] = manifest.Target{UI: "hung"}
				if tc.logo {
					m.Icon = "logo"
					m.Assets = append(m.Assets, manifest.Asset{ID: "logo", File: "logo.png", Kind: "image", MediaType: "image/png"})
				}
				m.Navigation[0].Icon = tc.symbol
				m.Pages = append(m.Pages, manifest.Page{ID: "detail", Title: "Detail", Path: "/detail/{id}", Methods: []string{"GET"}, Capabilities: []string{}})
				if tc.precedingIcon {
					m.Navigation = append([]manifest.Navigation{{ID: "detail", Title: "Detail", Page: "detail", Icon: "hi-16-solid-cube"}}, m.Navigation...)
				}
				if tc.nested {
					m.Navigation = []manifest.Navigation{{ID: "group", Title: "Group", Children: m.Navigation}}
				}
			}, map[string][]byte{"logo.png": logo.Bytes()})
			// Wait only for metadata publication, never the hung UI handshake.
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			for registry.Connection("example.fixture", notificationClientTestID) == nil {
				select {
				case <-tick.C:
				case <-deadline.C:
					t.Fatal("plugin metadata not published")
				}
			}
			kind := tc.kind
			if kind == "" {
				kind = "plugin"
			}
			connection := tc.connection
			if connection == "" {
				connection = notificationClientTestID
			}
			page := tc.page
			if page == "" {
				page = "current"
			}
			f := &notificationFixture{page: view.NotificationsPage{Items: []view.Notification{{ID: "icon-fixture", Title: "Event", Read: true, SourceKind: kind, ProducerID: "example.fixture", ProducerConnectionID: connection, ProducerPage: page}}}}
			h, err := NewWithPlugins(f, time.Second, nil, registry)
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"/notifications/tray", "/notifications"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route, nil))
				if w.Code != 200 {
					t.Fatalf("%s status %d", route, w.Code)
				}
				id := "notification-item-icon-fixture"
				if strings.HasSuffix(route, "tray") {
					id = "notification-tray-item-icon-fixture"
				}
				_, row, ok := strings.Cut(w.Body.String(), `id="`+id+`"`)
				if !ok {
					t.Fatal("notification row missing")
				}
				row, _, _ = strings.Cut(row, "</li>")
				if tc.want == "logo.png" {
					if !strings.Contains(row, `/logo.png"`) {
						t.Fatalf("%s missing producer logo", route)
					}
					_, source, _ := strings.Cut(row, `src="`)
					source, _, _ = strings.Cut(source, `"`)
					if !strings.HasPrefix(source, "/plugins/assets/example.fixture/") {
						t.Fatal("logo is not local/content-addressed")
					}
					asset := request(t, h, "GET", source, "")
					if asset.Code != 200 || asset.Header().Get("Content-Type") != "image/png" || asset.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(asset.Body.Bytes(), logo.Bytes()) {
						t.Fatal("declared logo not served intact")
					}
				} else if !strings.Contains(row, "#"+tc.want) {
					t.Fatalf("%s missing producer/fallback icon %s", route, tc.want)
				}
				if strings.Contains(row, "https://invalid.example") {
					t.Fatal("producer URL reached notification markup")
				}
			}
			if f.page.Items[0].ProducerIcon != "" {
				t.Fatal("render mutated catalog-owned notification slice")
			}
			if f.reads.Load()+f.clears.Load()+f.bulkReads.Load() != 0 {
				t.Fatal("icon rendering changed read state")
			}
		})
	}
}

func TestUnfilteredNotificationReadOwnsAllUnreadIndicators(t *testing.T) {
	for _, blockedCount := range []bool{false, true} {
		t.Run(fmt.Sprint(blockedCount), func(t *testing.T) {
			id := "00000000-0000-4000-8000-000000000001"
			f := &notificationFixture{page: view.NotificationsPage{Items: []view.Notification{{ID: id, Title: "Event", ReceivedAt: time.Now()}}}}
			h, _ := New(f, 25*time.Millisecond)
			initial := request(t, h, "GET", "/notifications", "").Body.String()
			_, target, found := strings.Cut(initial, `id="notification-read-`+id+`"`)
			if !found || !strings.Contains(strings.SplitN(target, "</form>", 2)[0], "balemoh-notification-unread-dot") {
				t.Fatal("unread indicator lives outside the read swap target")
			}
			f.blocked = blockedCount
			r := httptest.NewRequest("POST", "/notifications/"+id+"/read", strings.NewReader("returnTo=%2Fnotifications"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("HX-Request-Type", "partial")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			body := w.Body.String()
			if w.Code != 200 || f.reads.Load() != 1 || w.Header().Get("HX-Retarget") != "" || strings.Contains(body, "balemoh-notification-unread-dot") || strings.Contains(body, "Mark read") || !strings.Contains(body, ">Read</span>") || !strings.Contains(body, `id="notification-summary"`) {
				t.Fatalf("read acknowledgement retained unread state: %d %s", w.Code, body)
			}
		})
	}
}
