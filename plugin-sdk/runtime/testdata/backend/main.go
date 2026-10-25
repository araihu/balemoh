package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
)

type handler struct {
	mu      sync.Mutex
	configs map[string]backend.Config
}

func (h *handler) Configure(_ context.Context, c backend.Config) (backend.Status, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.Secrets["token"] != "declared-only" || os.Getenv("BALEMOH_TEST_SECRET") != "" {
		return backend.Status{}, backend.Fail("invalid_argument")
	}
	h.configs[c.ConnectionID] = c
	return backend.Status{ConnectionID: c.ConnectionID, AppliedRevision: c.Revision, State: "unavailable", ObservationRevision: "1"}, nil
}
func (h *handler) Status(_ context.Context, id string) (backend.Status, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.configs[id]
	if !ok {
		return backend.Status{}, backend.Fail("not_found")
	}
	return backend.Status{ConnectionID: id, AppliedRevision: c.Revision, State: "unavailable", ObservationRevision: "1"}, nil
}
func (h *handler) CloseConnection(_ context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.configs, id)
	return nil
}
func (h *handler) Query(ctx context.Context, r backend.Request) (backend.Result, error) {
	if r.OperationID == "fixture.wait" {
		<-ctx.Done()
		return backend.Result{}, ctx.Err()
	}
	if r.OperationID == "fixture.bad" {
		return backend.Result{Data: json.RawMessage(`{"unexpected":true}`), State: "unavailable", ObservationRevision: "1"}, nil
	}
	return backend.Result{Data: json.RawMessage(`{"value":"safe-data"}`), State: "unavailable", ObservationRevision: "1"}, nil
}
func main() {
	p, err := manifest.Load(".")
	if err != nil {
		os.Exit(1)
	}
	runtime.ServeBackend(p, &handler{configs: map[string]backend.Config{}})
}
