package bff

import (
	"context"
	"encoding/json"
	"errors"
	api "github.com/araihu/balemoh/client"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const notificationClientTestID = "00000000-0000-4000-8000-000000000001"

func TestAPIClientScopesNotificationCredentialToNotificationRequests(t *testing.T) {
	const credential = "notification-operator-test-credential"
	type observedRequest struct {
		method string
		path   string
		auth   string
	}
	var observed []observedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = append(observed, observedRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/notifications":
			_, _ = io.WriteString(w, `{"retentionDays":30,"notifications":[`+notificationClientTestJSON()+`],"unreadCount":1,"nextCursor":"cursor-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/notifications/"+notificationClientTestID:
			_, _ = io.WriteString(w, notificationClientTestJSONWithExternalDestination())
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/notifications/"+notificationClientTestID+"/read":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/notifications/read":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/notifications":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/homepage/services":
			_, _ = io.WriteString(w, `{"services":[]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/staging/services":
			_, _ = io.WriteString(w, `{"services":[]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/hosts":
			_, _ = io.WriteString(w, `{"hosts":[]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/dashboard":
			_, _ = io.WriteString(w, `{"items":[],"unavailable":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/discovery/sync":
			_, _ = io.WriteString(w, `{"sources":0,"candidates":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewAPIClient(server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client.SetNotificationCredential(credential)

	page, err := client.Notifications(context.Background(), "", false, 5)
	if err != nil {
		t.Fatalf("Notifications() error = %v", err)
	}
	if len(page.Items) != 1 || page.UnreadCount != 1 || page.NextCursor != "cursor-1" {
		t.Fatalf("Notifications() page = %#v", page)
	}
	record, err := client.Notification(context.Background(), notificationClientTestID)
	if err != nil {
		t.Fatalf("Notification() error = %v", err)
	}
	if record.Item.ID != notificationClientTestID || record.Target == nil || record.Target.URL != "https://example.test/incident" {
		t.Fatalf("Notification() record = %#v", record)
	}
	if err := client.MarkNotificationRead(context.Background(), notificationClientTestID); err != nil {
		t.Fatalf("MarkNotificationRead() error = %v", err)
	}
	if err := client.MarkAllNotificationsRead(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.ClearNotifications(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Homepage(context.Background()); err != nil {
		t.Fatalf("Homepage() error = %v", err)
	}
	if _, err := client.Staging(context.Background()); err != nil {
		t.Fatalf("Staging() error = %v", err)
	}
	if _, err := client.Hosts(context.Background()); err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	if _, err := client.Dashboard(context.Background()); err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if _, err := client.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(observed) != 10 {
		t.Fatalf("observed %d requests, want 10", len(observed))
	}
	for _, request := range observed {
		isNotification := strings.HasPrefix(request.path, "/api/v1/notifications")
		if isNotification && request.auth != "Bearer "+credential {
			t.Errorf("%s %s Authorization = %q, want notification credential", request.method, request.path, request.auth)
		}
		if !isNotification && request.auth != "" {
			t.Errorf("%s %s unexpectedly carried Authorization = %q", request.method, request.path, request.auth)
		}
	}
}

func TestAPIClientNotificationMethodsFailClosedWithoutCredential(t *testing.T) {
	var requests atomic.Int32
	transport := notificationClientFailIfCalledTransport{requests: &requests}
	client, err := NewAPIClient("http://notification-client.invalid", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		call func(context.Context) error
	}{
		{name: "list", call: func(ctx context.Context) error {
			_, err := client.Notifications(ctx, "", false, 5)
			return err
		}},
		{name: "get", call: func(ctx context.Context) error {
			_, err := client.Notification(ctx, notificationClientTestID)
			return err
		}},
		{name: "mark all read", call: func(ctx context.Context) error { return client.MarkAllNotificationsRead(ctx) }},
		{name: "clear", call: func(ctx context.Context) error { return client.ClearNotifications(ctx) }},
		{name: "mark read", call: func(ctx context.Context) error {
			return client.MarkNotificationRead(ctx, notificationClientTestID)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(context.Background()); err == nil {
				t.Fatal("notification call succeeded without a credential")
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("notification call reached transport %d times", got)
			}
		})
	}
}

func TestAPIClientNotificationErrorsHideUnexpectedUpstreamBody(t *testing.T) {
	const upstreamSecret = "upstream-response-secret"
	for _, test := range []struct {
		name      string
		operation string
		status    int
		body      string
	}{
		{name: "list status", operation: "list", status: http.StatusBadGateway, body: `{"message":"` + upstreamSecret + `"}`},
		{name: "get status", operation: "get", status: http.StatusInternalServerError, body: `{"debug":"` + upstreamSecret + `"}`},
		{name: "read status", operation: "read", status: http.StatusServiceUnavailable, body: `{"token":"` + upstreamSecret + `"}`},
		{name: "malformed list success", operation: "list", status: http.StatusOK, body: `{"notifications":`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.operation == "get" && r.URL.Path != "/api/v1/notifications/"+notificationClientTestID {
					http.NotFound(w, r)
					return
				}
				if test.operation == "read" && r.URL.Path != "/api/v1/notifications/"+notificationClientTestID+"/read" {
					http.NotFound(w, r)
					return
				}
				if test.operation == "list" && r.URL.Path != "/api/v1/notifications" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			client, err := NewAPIClient(server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			client.SetNotificationCredential("notification-operator-test-credential")
			var callErr error
			switch test.operation {
			case "list":
				_, callErr = client.Notifications(context.Background(), "", false, 5)
			case "get":
				_, callErr = client.Notification(context.Background(), notificationClientTestID)
			case "read":
				callErr = client.MarkNotificationRead(context.Background(), notificationClientTestID)
			}
			if callErr == nil {
				t.Fatal("notification call succeeded for unexpected upstream response")
			}
			if strings.Contains(callErr.Error(), upstreamSecret) {
				t.Fatal("notification error leaked upstream body")
			}
			var upstream *upstreamError
			if !errors.As(callErr, &upstream) {
				t.Fatalf("notification error type = %T, want upstreamError", callErr)
			}
			if test.status != http.StatusOK && upstream.status != test.status {
				t.Fatalf("upstream status = %d, want %d", upstream.status, test.status)
			}
		})
	}
}

type notificationClientFailIfCalledTransport struct {
	requests *atomic.Int32
}

func (t notificationClientFailIfCalledTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.requests.Add(1)
	return nil, errors.New("unexpected network request")
}

func notificationClientTestJSON() string {
	return `{"id":"` + notificationClientTestID + `","source":{"kind":"core","id":"core"},"eventId":"event-1","severity":"info","title":"Title","body":"Body","occurredAt":"2026-10-25T12:00:00Z","receivedAt":"2026-10-25T12:01:00Z"}`
}

func notificationClientTestJSONWithExternalDestination() string {
	return `{"id":"` + notificationClientTestID + `","source":{"kind":"core","id":"core"},"eventId":"event-1","severity":"info","title":"Title","body":"Body","occurredAt":"2026-10-25T12:00:00Z","receivedAt":"2026-10-25T12:01:00Z","destination":{"kind":"external","url":"https://example.test/incident"}}`
}

func TestNotificationProducerPageUsesOwnDestinationOnly(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		raw := `{"id":"` + notificationClientTestID + `","source":{"kind":"plugin","id":"example.fixture","connectionId":"` + notificationClientTestID + `"},"destination":{"kind":"plugin","pluginId":"example.fixture","connectionId":"` + notificationClientTestID + `","page":"current"}}`
		if foreign {
			raw = strings.Replace(raw, `"pluginId":"example.fixture"`, `"pluginId":"other.plugin"`, 1)
		}
		var item api.Notification
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			t.Fatal(err)
		}
		got := notificationView(item)
		want := "current"
		if foreign {
			want = ""
		}
		if got.ProducerPage != want || got.ProducerID != "example.fixture" || got.ProducerConnectionID != notificationClientTestID {
			t.Fatalf("producer identity/page = %#v", got)
		}
	}
}
