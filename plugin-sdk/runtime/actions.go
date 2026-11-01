package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func actionFromWire(r *pb.ActionResult) (backend.ActionResult, error) {
	if r == nil {
		return backend.ActionResult{}, backend.Fail("invalid_response")
	}
	result := backend.ActionResult{Data: r.DataJson}
	return result, result.Validate()
}

func (c *BackendClient) Action(ctx context.Context, r backend.Request) (backend.ActionResult, error) {
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	default:
		return backend.ActionResult{}, backend.Fail("resource_exhausted")
	}
	if !backend.Object(r.Input, backend.MaxInput) {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	value, err := c.process.wire.backend.Action(ctx, &pb.BackendQueryRequest{ConnectionId: r.ConnectionID, OperationId: r.OperationID, InputJson: r.Input, RequestId: r.RequestID})
	if err != nil {
		return backend.ActionResult{}, fromRPC(err)
	}
	return actionFromWire(value)
}

func (s *rpcServer) Action(ctx context.Context, r *pb.BackendQueryRequest) (*pb.ActionResult, error) {
	a, ok := s.pkg.Manifest.Action(r.OperationId)
	handler, supported := s.backend.(backend.ActionHandler)
	if !ok || !supported {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	if !backend.Object(r.InputJson, backend.MaxInput) || s.pkg.ValidateJSON(a.InputSchema, r.InputJson) != nil {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	result, err := handler.Action(ctx, backend.Request{ConnectionID: r.ConnectionId, OperationID: r.OperationId, Input: r.InputJson, RequestID: r.RequestId})
	if err != nil {
		return nil, toRPC(err)
	}
	if result.Validate() != nil || s.pkg.ValidateJSON(a.OutputSchema, result.Data) != nil {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	return &pb.ActionResult{DataJson: result.Data}, nil
}

func (h *hostClient) Action(ctx context.Context, operation string, input json.RawMessage) (backend.ActionResult, error) {
	if !backend.Object(input, backend.MaxInput) {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	r, err := h.client.Action(ctx, &pb.QueryRequest{OperationId: operation, InputJson: input})
	if err != nil {
		return backend.ActionResult{}, fromRPC(err)
	}
	return actionFromWire(r)
}

func (h *hostServer) Action(ctx context.Context, r *pb.QueryRequest) (*pb.ActionResult, error) {
	if !h.actionAllowed || h.pkg == nil {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	a, declared := h.pkg.Manifest.Action(r.OperationId)
	if !declared {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	if h.pkg.ValidateJSON(a.InputSchema, r.InputJson) != nil {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, toRPC(backend.Fail("resource_exhausted"))
	}
	if h.ctx.Err() != nil {
		return nil, toRPC(h.ctx.Err())
	}
	handler, ok := h.services.(ui.ActionServices)
	if !ok {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	if !backend.Object(r.InputJson, backend.MaxInput) {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	result, err := handler.Action(ctx, r.OperationId, r.InputJson)
	if err != nil {
		return nil, toRPC(err)
	}
	if result.Validate() != nil || h.pkg.ValidateJSON(a.OutputSchema, result.Data) != nil {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	return &pb.ActionResult{DataJson: result.Data}, nil
}
