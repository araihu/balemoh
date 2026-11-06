package plugins

import (
	"context"
	"encoding/json"
	"slices"

	app "github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/backend"
)

func (r *Runtime) GrantedActions(plugin, id string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.plugins[plugin]
	if p == nil || p.client == nil || p.client.Exited() {
		return []string{}
	}
	c := p.connections[id]
	if c == nil || !c.ready || !slices.Contains(c.config.capabilityGrants, "data.action/v1") {
		return []string{}
	}
	result := []string{}
	for _, a := range p.pkg.Manifest.Actions {
		if slices.Contains(c.config.grants, a.ID) {
			result = append(result, a.ID)
		}
	}
	return result
}

func (r *Runtime) Action(ctx context.Context, plugin, id, operation string, input json.RawMessage) (app.ActionResult, error) {
	r.mu.RLock()
	p := r.plugins[plugin]
	if p == nil {
		r.mu.RUnlock()
		return app.ActionResult{}, app.Fail("unavailable")
	}
	c := p.connections[id]
	if c == nil || !slices.Contains(c.config.capabilityGrants, "data.action/v1") || !slices.Contains(c.config.grants, operation) {
		r.mu.RUnlock()
		return app.ActionResult{}, app.Fail("permission_denied")
	}
	a, declared := p.pkg.Manifest.Action(operation)
	child, ready, pkg := p.client, c.ready, p.pkg
	r.mu.RUnlock()
	if !declared || !ready || child == nil || child.Exited() {
		return app.ActionResult{}, app.Fail("unavailable")
	}
	if !backend.Object(input, backend.MaxInput) || pkg.ValidateJSON(a.InputSchema, input) != nil {
		return app.ActionResult{}, app.Fail("invalid_argument")
	}
	result, err := child.Action(ctx, backend.Request{ConnectionID: id, OperationID: operation, Input: input, RequestID: app.RequestID(ctx)})
	if err != nil {
		return app.ActionResult{}, app.Fail(backend.Code(err))
	}
	if result.Validate() != nil || pkg.ValidateJSON(a.OutputSchema, result.Data) != nil {
		return app.ActionResult{}, app.Fail("invalid_response")
	}
	return app.ActionResult{Data: result.Data}, nil
}
