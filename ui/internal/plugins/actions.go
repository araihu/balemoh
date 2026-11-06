package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/araihu/balemoh/plugin-sdk/backend"
)

func (h scopedServices) Action(ctx context.Context, operation string, input json.RawMessage) (backend.ActionResult, error) {
	a, ok := h.entry.Package.Manifest.Action(operation)
	if !h.allowActions || !ok || !h.grants[operation] {
		return backend.ActionResult{}, backend.Fail("permission_denied")
	}
	if !backend.Object(input, backend.MaxInput) || h.entry.Package.ValidateJSON(a.InputSchema, input) != nil {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	body, _ := json.Marshal(struct {
		Input json.RawMessage `json:"input"`
	}{input})
	response, err := h.source.client.ActionPluginWithBody(ctx, h.entry.Package.Manifest.ID, h.entry.Installation.ConnectionID, operation, "application/json", strings.NewReader(string(body)), func(_ context.Context, r *http.Request) error {
		r.Header.Set("X-Request-ID", h.requestID)
		return nil
	})
	if err != nil {
		return backend.ActionResult{}, backend.Fail("unavailable")
	}
	var value backend.ActionResult
	if err := readResponse(response, &value); err != nil {
		return backend.ActionResult{}, err
	}
	if value.Validate() != nil || h.entry.Package.ValidateJSON(a.OutputSchema, value.Data) != nil {
		return backend.ActionResult{}, backend.Fail("invalid_response")
	}
	return value, nil
}
