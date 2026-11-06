package plugins

import (
	"context"
	"encoding/json"
)

type ActionRuntime interface {
	Action(context.Context, string, string, string, json.RawMessage) (ActionResult, error)
	GrantedActions(string, string) []string
}

type ActionResult struct {
	Data json.RawMessage `json:"data"`
}

func (s *Service) Action(ctx context.Context, plugin, connection, operation string, input json.RawMessage) (ActionResult, error) {
	c, err := s.store.Get(ctx, plugin, connection)
	if err != nil {
		return ActionResult{}, err
	}
	if !c.Enabled {
		return ActionResult{}, Fail("permission_denied")
	}
	runtime, ok := s.runtime.(ActionRuntime)
	if !ok {
		return ActionResult{}, Fail("permission_denied")
	}
	if len(input) > 64<<10 {
		return ActionResult{}, Fail("invalid_argument")
	}
	return runtime.Action(ctx, plugin, connection, operation, input)
}
