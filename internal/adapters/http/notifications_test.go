package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/notifications"
)

func notificationHTTPHandler(t *testing.T) (http.Handler, *notifications.Service, func()) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlite.RunMigrations(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	service := notifications.NewServiceWithClock(sqlite.NewNotificationStore(db), func() time.Time { return now })
	auth := NotificationAuth{OperatorToken: "operator-secret", Publishers: map[string]string{"monitor": "publisher-secret"}}
	handler := NewHandler(nil, nil).WithNotifications(service, auth)
	return generated.HandlerFromMux(handler, http.NewServeMux()), service, func() { _ = db.Close() }
}

func TestNotificationHTTPSeparatesOperatorAndPublisherCredentials(t *testing.T) {
	handler, _, cleanup := notificationHTTPHandler(t)
	defer cleanup()
	for _, test := range []struct {
		name   string
		method string
		path   string
		token  string
		body   string
		want   int
	}{
		{name: "publisher cannot list", method: http.MethodGet, path: "/api/v1/notifications", token: "publisher-secret", want: http.StatusUnauthorized},
		{name: "operator cannot publish", method: http.MethodPost, path: "/api/v1/notifications", token: "operator-secret", body: validNotificationJSON(), want: http.StatusUnauthorized},
		{name: "unknown publisher", method: http.MethodPost, path: "/api/v1/notifications", token: "other", body: validNotificationJSON(), want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer "+test.token)
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestNotificationHTTPPublishListGetAndRead(t *testing.T) {
	handler, _, cleanup := notificationHTTPHandler(t)
	defer cleanup()
	publish := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(validNotificationJSON()))
	publish.Header.Set("Authorization", "Bearer publisher-secret")
	publish.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, publish)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("publish status = %d, want 201; body = %s", recorder.Code, recorder.Body.String())
	}
	var receipt generated.NotificationReceipt
	if err := json.NewDecoder(recorder.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	if !receipt.Accepted || receipt.Id == [16]byte{} {
		t.Fatalf("receipt = %#v", receipt)
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	list.Header.Set("Authorization", "Bearer operator-secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, list)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	listBody := recorder.Body.String()
	var page generated.NotificationPage
	if err := json.NewDecoder(recorder.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Notifications) != 1 || page.UnreadCount != 1 || page.Notifications[0].ReadAt != nil {
		t.Fatalf("page = %#v", page)
	}
	if page.Notifications[0].Source.Id != "monitor" || page.Notifications[0].Source.Kind != generated.NotificationSourceKindExternal {
		t.Fatalf("source = %#v", page.Notifications[0].Source)
	}
	if !strings.Contains(listBody, `\u003cscript\u003e`) {
		t.Fatalf("hostile title was not JSON escaped: %s", listBody)
	}

	getPath := "/api/v1/notifications/" + receipt.Id.String()
	get := httptest.NewRequest(http.MethodGet, getPath, nil)
	get.Header.Set("Authorization", "Bearer operator-secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, get)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	read := httptest.NewRequest(http.MethodPut, getPath+"/read", nil)
	read.Header.Set("Authorization", "Bearer operator-secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, read)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("read status = %d, want 204; body = %s", recorder.Code, recorder.Body.String())
	}
	readAgain := httptest.NewRecorder()
	handler.ServeHTTP(readAgain, read)
	if readAgain.Code != http.StatusNoContent {
		t.Fatalf("repeated read status = %d, want 204", readAgain.Code)
	}
}

func TestNotificationHTTPRejectsInvalidURLAndOversizedBodyWithoutFetching(t *testing.T) {
	handler, _, cleanup := notificationHTTPHandler(t)
	defer cleanup()
	var fetched atomic.Int32
	invalid := validNotificationJSONWithURL("https://user:pass@example.test")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(invalid))
	request.Header.Set("Authorization", "Bearer publisher-secret")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid URL status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if fetched.Load() != 0 {
		t.Fatal("notification adapter fetched destination URL")
	}
	oversized := `{"eventId":"large","severity":"info","title":"title","body":"` + strings.Repeat("x", 17<<10) + `","occurredAt":"2026-10-25T12:00:00Z"}`
	request = httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(oversized))
	request.Header.Set("Authorization", "Bearer publisher-secret")
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want 413; body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestNotificationHTTPUsesDistinctCapacityError(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		status     int
		code       string
		retryAfter string
	}{
		{name: "capacity", err: notifications.ErrCapacity, status: http.StatusTooManyRequests, code: "capacity_exceeded"},
		{name: "envelope", err: notifications.ErrTooLarge, status: http.StatusRequestEntityTooLarge, code: "payload_too_large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeNotificationError(recorder, test.err)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.status, recorder.Body.String())
			}
			var response generated.ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.code {
				t.Fatalf("error code = %q, want %q", response.Code, test.code)
			}
			if got := recorder.Header().Get("Retry-After"); got != test.retryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, test.retryAfter)
			}
		})
	}
}

func TestNotificationHTTPBusyUsesResourceExhausted(t *testing.T) {
	db, err := sqlite.Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	service := notifications.NewService(sqlite.NewNotificationStore(db))
	base := NewHandler(nil, nil).WithNotifications(service, NotificationAuth{
		OperatorToken: "operator-secret",
		Publishers:    map[string]string{"monitor": "publisher-secret"},
	})
	for i := 0; i < cap(base.notificationSlots["monitor"]); i++ {
		base.notificationSlots["monitor"] <- struct{}{}
	}
	handler := generated.HandlerFromMux(base, http.NewServeMux())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(validNotificationJSON()))
	request.Header.Set("Authorization", "Bearer publisher-secret")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	var response generated.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "resource_exhausted" {
		t.Fatalf("error code = %q, want resource_exhausted", response.Code)
	}
}

func TestNotificationCursorRoundTripAndValidation(t *testing.T) {
	for _, sequence := range []uint64{1, 99, 1 << 32} {
		cursor := encodeNotificationCursor(sequence)
		got, err := decodeNotificationCursor(cursor)
		if err != nil || got != sequence {
			t.Fatalf("cursor %q = %d, %v; want %d", cursor, got, err, sequence)
		}
	}
	for _, cursor := range []string{"", "not-base64", "bm90aWZpY2F0aW9uOjA="} {
		if _, err := decodeNotificationCursor(cursor); err == nil {
			t.Fatalf("decodeNotificationCursor(%q) succeeded", cursor)
		}
	}
}

func validNotificationJSON() string {
	return validNotificationJSONWithURL("https://example.test/incident")
}

func validNotificationJSONWithURL(url string) string {
	return `{"eventId":"event-1","severity":"warning","title":"<script>alert(1)</script>","body":"body","occurredAt":"2026-10-25T12:00:00Z","destination":{"kind":"external","url":"` + url + `"}}`
}
