package http

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	push "github.com/araihu/balemoh/internal/application/webpush"
)

type testPushSender struct{}

func (testPushSender) Send(_ context.Context, _ push.Delivery) (int, error) { return 201, nil }
func TestWebPushAPIAuthenticationOwnershipValidationAndRemoval(t *testing.T) {
	db, err := sqlite.Open(t.Context(), t.TempDir()+"/push.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := sqlite.NewWebPushStore(db)
	service := push.New(store, testPushSender{}, "public")
	h := NewHandler(nil, nil).WithNotifications(nil, NotificationAuth{OperatorToken: "operator", Publishers: map[string]string{"source": "publisher"}}).WithWebPush(service)
	handler := generated.HandlerFromMux(h, http.NewServeMux())
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	registration := push.Registration{Token: token, Subscription: push.Subscription{Endpoint: "https://push.example/device", Keys: push.Keys{P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}}
	call := func(method, bearer, device, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/notifications/push", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+bearer)
		if device != "" {
			r.Header.Set("X-Push-Token", device)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		for _, credential := range []string{"", "publisher", "invalid"} {
			if w := call(method, credential, token, `{}`); w.Code != 401 {
				t.Fatalf("%s accepted non-operator: %d", method, w.Code)
			}
		}
	}
	body, _ := json.Marshal(registration)
	if w := call("POST", "operator", "", string(body)); w.Code != 204 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	w := call("GET", "operator", token, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"subscribed":true`) || strings.Contains(w.Body.String(), "push.example") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe subscription settings")
	}
	for _, invalid := range []string{`{}`, `{"token":"bad"}`, string(body) + `{}`, strings.Repeat("x", 9000)} {
		if w := call("POST", "operator", "", invalid); w.Code != 400 {
			t.Fatalf("invalid payload accepted: %d", w.Code)
		}
	}
	registration.Token = base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	body, _ = json.Marshal(registration)
	if w := call("POST", "operator", "", string(body)); w.Code != 409 {
		t.Fatal("endpoint ownership bypass")
	}
	if w := call("DELETE", "operator", token, ""); w.Code != 204 {
		t.Fatal("unsubscribe failed")
	}
	exists, err := store.Exists(t.Context(), mustPushID(t, token), time.Now())
	if err != nil || exists {
		t.Fatal("subscription not removed")
	}
}
func mustPushID(t *testing.T, token string) string {
	t.Helper()
	id, err := push.TokenID(token)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
