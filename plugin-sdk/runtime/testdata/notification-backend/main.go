package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
)

type config struct {
	Publish            bool `json:"publish"`
	DelayMS            int  `json:"delayMs"`
	Attempts           int  `json:"attempts"`
	ConfigureDelayMS   int  `json:"configureDelayMs"`
	RequireNoPublisher bool `json:"requireNoPublisher"`
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
	previous, alreadyConfigured := h.connections[c.ConnectionID]
	if alreadyConfigured && previous.Revision == c.Revision && sameJSON(previous.Config, c.Config) && sameSecrets(previous.Secrets, c.Secrets) {
		h.mu.Unlock()
		return fixtureStatus(c), nil
	}
	if value.RequireNoPublisher && c.Notifications != nil {
		h.mu.Unlock()
		return backend.Status{}, backend.Fail("permission_denied")
	}
	h.connections[c.ConnectionID] = c
	h.mu.Unlock()
	if value.ConfigureDelayMS > 0 {
		time.Sleep(time.Duration(value.ConfigureDelayMS) * time.Millisecond)
	}
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
	return fixtureStatus(c), nil
}

func fixtureStatus(c backend.Config) backend.Status {
	now := time.Now().UTC()
	return backend.Status{ConnectionID: c.ConnectionID, AppliedRevision: c.Revision, State: "fresh", ObservationRevision: "1", ObservedAt: &now, LastAttemptAt: &now, LastSuccessAt: &now}
}

func sameSecrets(first, second map[string]string) bool {
	if len(first) != len(second) {
		return false
	}
	for key, value := range first {
		if second[key] != value {
			return false
		}
	}
	return true
}

func sameJSON(first, second []byte) bool {
	decode := func(data []byte) (any, error) {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
	firstValue, firstErr := decode(first)
	secondValue, secondErr := decode(second)
	return firstErr == nil && secondErr == nil && reflect.DeepEqual(firstValue, secondValue)
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
