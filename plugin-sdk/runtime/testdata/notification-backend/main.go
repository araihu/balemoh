package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
)

type config struct {
	Publish  bool `json:"publish"`
	DelayMS  int  `json:"delayMs"`
	Attempts int  `json:"attempts"`
}

type handler struct {
	mu          sync.Mutex
	connections map[string]backend.Config
}

func (h *handler) Configure(_ context.Context, c backend.Config) (backend.Status, error) {
	var value config
	if err := json.Unmarshal(c.Config, &value); err != nil {
		return backend.Status{}, backend.Fail("invalid_argument")
	}
	h.mu.Lock()
	h.connections[c.ConnectionID] = c
	h.mu.Unlock()
	if c.Notifications != nil && value.Publish {
		attempts := value.Attempts
		if attempts < 1 {
			attempts = 1
		}
		delay := time.Duration(value.DelayMS) * time.Millisecond
		publisher := c.Notifications
		go func() {
			time.Sleep(delay)
			message := notifications.Message{EventID: "fixture-event", Severity: "warning", Title: "Fixture notification", Body: "safe fixture body", OccurredAt: time.Now().UTC()}
			for range attempts {
				_, _ = publisher.Publish(context.Background(), message)
			}
		}()
	}
	now := time.Now().UTC()
	return backend.Status{ConnectionID: c.ConnectionID, AppliedRevision: c.Revision, State: "fresh", ObservationRevision: "1", ObservedAt: &now, LastAttemptAt: &now, LastSuccessAt: &now}, nil
}

func (h *handler) Status(_ context.Context, id string) (backend.Status, error) {
	h.mu.Lock()
	_, ok := h.connections[id]
	h.mu.Unlock()
	if !ok {
		return backend.Status{}, backend.Fail("not_found")
	}
	now := time.Now().UTC()
	return backend.Status{ConnectionID: id, AppliedRevision: "1", State: "fresh", ObservationRevision: "1", ObservedAt: &now, LastAttemptAt: &now, LastSuccessAt: &now}, nil
}

func (h *handler) Query(context.Context, backend.Request) (backend.Result, error) {
	return backend.Result{}, backend.Fail("permission_denied")
}

func (h *handler) CloseConnection(_ context.Context, id string) error {
	h.mu.Lock()
	delete(h.connections, id)
	h.mu.Unlock()
	return nil
}

func main() {
	p, err := manifest.Load(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	runtime.ServeBackend(p, &handler{connections: map[string]backend.Config{}})
}
