package bff

import (
	"context"
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pushFixture struct {
	fakeCatalog
	settings       api.WebPushSettings
	saves, deletes int
}

func (f *pushFixture) WebPushSettings(context.Context, string) (api.WebPushSettings, error) {
	return f.settings, nil
}
func (f *pushFixture) RegisterWebPush(context.Context, api.WebPushRegistration) error {
	f.saves++
	return nil
}
func (f *pushFixture) DeleteWebPush(context.Context, webPushToken) error { f.deletes++; return nil }
func TestWebPushBFFProtectionAndManifest(t *testing.T) {
	f := &pushFixture{settings: api.WebPushSettings{Enabled: true, PublicKey: "public"}}
	h, err := New(f, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"POST", "DELETE"} {
		r := httptest.NewRequest(method, "/notifications/push", strings.NewReader(`{"token":"private"}`))
		r.Header.Set("Origin", "https://other.example")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin subscription mutation accepted")
		}
	}
	for _, test := range []struct{ method, body string }{{"POST", `{"token":"private","subscription":{"endpoint":"https://push.example","keys":{"p256dh":"key","auth":"auth"}}}`}, {"DELETE", `{"token":"private"}`}} {
		r := httptest.NewRequest(test.method, "/notifications/push", strings.NewReader(test.body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("subscription mutation: %d", w.Code)
		}
	}
	if f.saves != 1 || f.deletes != 1 {
		t.Fatal("wrong mutation count")
	}
	w := request(t, h, "GET", "/notifications/push", "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "private") {
		t.Fatal("unsafe settings response")
	}
	w = request(t, h, "GET", "/manifest.webmanifest", "")
	var manifest map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &manifest) != nil || manifest["display"] != "standalone" {
		t.Fatal("install manifest unavailable")
	}
	for _, path := range []string{"/ui/browser-notifications.js", "/notifications-worker.js"} {
		w = request(t, h, "GET", path, "")
		if w.Code != 200 {
			t.Fatalf("script unavailable: %s", path)
		}
	}
}
func TestWebPushClientUsesOnlyNotificationAuthority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer operator-private" {
			t.Error("operator credential missing")
		}
		if r.URL.Path != "/api/v1/notifications/push" {
			t.Error("wrong upstream route")
		}
		if r.Method == "GET" || r.Method == "DELETE" {
			if r.Header.Get("X-Push-Token") != "device-private" {
				t.Error("device token missing")
			}
			if r.Method == "DELETE" {
				w.WriteHeader(204)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"enabled":true,"publicKey":"public","subscribed":false}`))
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	client, err := NewAPIClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.WebPushSettings(t.Context(), "device-private"); err == nil {
		t.Fatal("missing credential accepted")
	}
	client.SetNotificationCredential("operator-private")
	if _, err = client.WebPushSettings(t.Context(), "device-private"); err != nil {
		t.Fatal(err)
	}
	if err = client.RegisterWebPush(t.Context(), api.WebPushRegistration{}); err != nil {
		t.Fatal(err)
	}
	if err = client.DeleteWebPush(t.Context(), webPushToken{Token: "device-private"}); err != nil {
		t.Fatal(err)
	}
}
