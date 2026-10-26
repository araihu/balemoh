package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
)

func notificationPackage(t *testing.T) *manifest.Package {
	t.Helper()
	p := packageFixture(t)
	cmd := exec.Command("go", "build", "-o", filepath.Join(p.Root, "backend"), "./testdata/notification-backend")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build notification backend: %v: %s", err, out)
	}
	p.Manifest.Targets[runtime.GOOS+"/"+runtime.GOARCH] = manifest.Target{UI: "ui", Backend: "backend"}
	p.Manifest.Capabilities.Required = append(p.Manifest.Capabilities.Required, "notifications.publish/v1")
	p.Manifest.Queries = []manifest.Query{}
	data, err := json.Marshal(p.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "plugin.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "schema.json"), []byte(`{"type":"object","properties":{"publish":{"type":"boolean"},"delayMs":{"type":"integer","minimum":0,"maximum":1000},"attempts":{"type":"integer","minimum":1,"maximum":4}},"additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = manifest.Load(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type notificationRecorder struct {
	mu        sync.Mutex
	attempts  []notifications.Message
	accepted  []notifications.Message
	failures  int
	acceptedC chan notifications.Message
}

func (r *notificationRecorder) Publish(_ context.Context, message notifications.Message) (notifications.Receipt, error) {
	r.mu.Lock()
	r.attempts = append(r.attempts, message)
	if r.failures > 0 {
		r.failures--
		r.mu.Unlock()
		return notifications.Receipt{}, notifications.Fail("unavailable")
	}
	r.accepted = append(r.accepted, message)
	r.mu.Unlock()
	r.acceptedC <- message
	return notifications.Receipt{ID: "receipt-1", Created: true}, nil
}

func (r *notificationRecorder) acceptedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.accepted)
}

func TestBackendPublisherIsConnectionScopedAndOutlivesConfigureContext(t *testing.T) {
	p := notificationPackage(t)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	recorder := &notificationRecorder{acceptedC: make(chan notifications.Message, 4)}
	configureCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	config := backend.Config{ConnectionID: "connection", Revision: "1", Config: []byte(`{"publish":true,"delayMs":40}`)}
	if _, err := child.ConfigureWithNotifications(configureCtx, config, recorder); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case message := <-recorder.acceptedC:
		if message.EventID != "fixture-event" || message.Severity != "warning" || message.Title == "" {
			t.Fatalf("unexpected message: %+v", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publisher did not outlive Configure context")
	}
}

func TestBackendPublishersRemainScopedPerConnection(t *testing.T) {
	p := notificationPackage(t)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	first := &notificationRecorder{acceptedC: make(chan notifications.Message, 2)}
	second := &notificationRecorder{acceptedC: make(chan notifications.Message, 2)}
	for _, tc := range []struct {
		id       string
		recorder *notificationRecorder
	}{
		{"connection-a", first},
		{"connection-b", second},
	} {
		config := backend.Config{ConnectionID: tc.id, Revision: "1", Config: []byte(`{"publish":true,"delayMs":10}`)}
		if _, err := child.ConfigureWithNotifications(t.Context(), config, tc.recorder); err != nil {
			t.Fatal(err)
		}
	}
	for name, recorder := range map[string]*notificationRecorder{"first": first, "second": second} {
		t.Run(name, func(t *testing.T) {
			select {
			case message := <-recorder.acceptedC:
				if message.EventID != "fixture-event" {
					t.Fatalf("unexpected scoped message: %+v", message)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("scoped publisher did not receive message")
			}
		})
	}
}

func TestBackendPublisherRetryKeepsEventIdentityAndReceipt(t *testing.T) {
	p := notificationPackage(t)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	recorder := &notificationRecorder{acceptedC: make(chan notifications.Message, 4), failures: 1}
	config := backend.Config{ConnectionID: "connection", Revision: "1", Config: []byte(`{"publish":true,"delayMs":1,"attempts":2}`)}
	if _, err := child.ConfigureWithNotifications(t.Context(), config, recorder); err != nil {
		t.Fatal(err)
	}
	select {
	case <-recorder.acceptedC:
	case <-time.After(2 * time.Second):
		t.Fatal("retry did not receive receipt")
	}
	recorder.mu.Lock()
	attempts := append([]notifications.Message(nil), recorder.attempts...)
	recorder.mu.Unlock()
	if len(attempts) != 2 || attempts[0].EventID != attempts[1].EventID {
		t.Fatalf("retry changed event identity: %+v", attempts)
	}
}

func TestBackendPublisherRevokedOnReplacementAndCloseConnection(t *testing.T) {
	p := notificationPackage(t)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	old := &notificationRecorder{acceptedC: make(chan notifications.Message, 2)}
	first := backend.Config{ConnectionID: "connection", Revision: "1", Config: []byte(`{"publish":true,"delayMs":150}`)}
	if _, err := child.ConfigureWithNotifications(t.Context(), first, old); err != nil {
		t.Fatal(err)
	}
	newer := &notificationRecorder{acceptedC: make(chan notifications.Message, 2)}
	second := backend.Config{ConnectionID: "connection", Revision: "2", Config: []byte(`{"publish":true,"delayMs":10}`)}
	if _, err := child.ConfigureWithNotifications(t.Context(), second, newer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-newer.acceptedC:
	case <-time.After(2 * time.Second):
		t.Fatal("replacement publisher did not receive message")
	}
	time.Sleep(250 * time.Millisecond)
	if old.acceptedCount() != 0 {
		t.Fatal("replaced callback remained usable")
	}
	third := backend.Config{ConnectionID: "closed", Revision: "1", Config: []byte(`{"publish":true,"delayMs":150}`)}
	closed := &notificationRecorder{acceptedC: make(chan notifications.Message, 2)}
	if _, err := child.ConfigureWithNotifications(t.Context(), third, closed); err != nil {
		t.Fatal(err)
	}
	if err := child.CloseConnection(t.Context(), "closed"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	if closed.acceptedCount() != 0 {
		t.Fatal("closed connection callback remained usable")
	}
}

func TestOldBackendCannotReceivePublisherGrant(t *testing.T) {
	p := backendPackage(t)
	child, err := pluginruntime.OpenBackend(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	recorder := &notificationRecorder{acceptedC: make(chan notifications.Message, 1)}
	config := backend.Config{ConnectionID: "connection", Revision: "1", Config: []byte(`{}`), Secrets: map[string]string{"token": "declared-only"}}
	if _, err := child.ConfigureWithNotifications(t.Context(), config, recorder); notifications.Code(err) != "permission_denied" {
		t.Fatalf("old/ungranted backend accepted publisher: %v", err)
	}
}
