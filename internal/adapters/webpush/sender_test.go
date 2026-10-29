package webpush

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	protocol "github.com/SherClockHolmes/webpush-go"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/notifications"
	push "github.com/araihu/balemoh/internal/application/webpush"
	"time"
)

type pushHTTP func(*http.Request) (*http.Response, error)

func (f pushHTTP) Do(r *http.Request) (*http.Response, error) { return f(r) }
func TestSenderEncryptsBoundsPayloadAndSignsVAPID(t *testing.T) {
	private, public, err := protocol.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	sender, err := New(private, "mailto:ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if sender.PublicKey() != public {
		t.Fatal("derived public key mismatch")
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	sender.client = pushHTTP(func(r *http.Request) (*http.Response, error) {
		called = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > 4096 || strings.Contains(string(body), "PRIVATE MESSAGE") {
			t.Fatal("payload not encrypted or too large")
		}
		if r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.Contains(r.Header.Get("Authorization"), "vapid ") || r.Header.Get("TTL") != "86400" {
			t.Fatalf("protocol headers missing")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	d := push.Delivery{NotificationID: "00000000-0000-4000-8000-000000000001", Title: strings.Repeat("<", 120), Body: strings.Repeat("PRIVATE MESSAGE<&", 1000), Subscription: push.Subscription{Endpoint: "https://push.example/device", Keys: push.Keys{P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}}
	status, err := sender.Send(t.Context(), d)
	if err != nil || status != 201 || !called {
		t.Fatalf("send: %d %v", status, err)
	}
}
func TestPushTransportRejectsPrivateDestinationsAndRedirects(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "0.1.2.3", "240.0.0.1", "192.0.2.1", "64:ff9b::a00:1", "2002:a00:1::", "2001:db8::1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.168.1.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("private address allowed: %s", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("public address blocked: %s", raw)
		}
	}
	if conn, err := publicDial(context.Background(), "tcp", "127.0.0.1:443"); err == nil {
		conn.Close()
		t.Fatal("loopback dial allowed")
	}
	private, _, _ := protocol.GenerateVAPIDKeys()
	sender, err := New(private, "mailto:ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	client := sender.client.(*http.Client)
	if client.CheckRedirect(&http.Request{}, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects permitted")
	}
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("proxy bypasses public-address validation")
	}
}

func TestDurableDeliveryWithoutAnyBrowserTabOrPublisherProcess(t *testing.T) {
	path := t.TempDir() + "/closed-tab.db"
	db, err := sqlite.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err = sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	private, _, _ := protocol.GenerateVAPIDKeys()
	transport, err := New(private, "mailto:ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	store := sqlite.NewWebPushStore(db)
	if err = store.Configure(t.Context(), transport.PublicKey(), true); err != nil {
		t.Fatal(err)
	}
	pushService := push.New(store, transport, transport.PublicKey())
	browserKey, _ := ecdh.P256().GenerateKey(rand.Reader)
	registration := push.Registration{Token: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), Subscription: push.Subscription{Endpoint: "https://push.example/device", Keys: push.Keys{P256dh: base64.RawURLEncoding.EncodeToString(browserKey.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}}
	if err = pushService.Register(t.Context(), registration); err != nil {
		t.Fatal(err)
	}
	publisher := notifications.NewService(sqlite.NewNotificationStore(db))
	request, cancel := context.WithCancel(t.Context())
	if _, err = publisher.Publish(request, notifications.Source{Kind: notifications.SourceExternal, ID: "qa"}, notifications.Message{EventID: "closed-tab", Title: "Background event", Severity: notifications.SeverityInfo, OccurredAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Recover using a fresh database connection and worker, with no UI process.
	db, err = sqlite.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sends := 0
	transport.client = pushHTTP(func(r *http.Request) (*http.Response, error) {
		sends++
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	pushService = push.New(sqlite.NewWebPushStore(db), transport, transport.PublicKey())
	pushService.Drain(t.Context())
	pushService.Drain(t.Context())
	if sends != 1 {
		t.Fatalf("want one push after restart, got %d", sends)
	}
}
