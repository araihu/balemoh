// Package plugintesting provides author fixtures without host internals or production credentials.
package plugintesting

import (
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type Services struct {
	ResolveServicesFunc func(context.Context, []string) ([]ui.ServiceIdentity, error)
	Current             ui.Placement
	Queries             map[string]ui.QueryResult
	QueryFunc           func(context.Context, string, json.RawMessage) (ui.QueryResult, error)
}

func (s Services) ResolveServices(ctx context.Context, ids []string) ([]ui.ServiceIdentity, error) {
	if s.ResolveServicesFunc == nil {
		return nil, backend.Fail("permission_denied")
	}
	return s.ResolveServicesFunc(ctx, ids)
}

func (s Services) Placement(context.Context) (ui.Placement, error) {
	if s.Current.ID == "" {
		return ui.Placement{}, backend.Fail("not_found")
	}
	return s.Current, nil
}
func (s Services) Query(ctx context.Context, operation string, input json.RawMessage) (ui.QueryResult, error) {
	if s.QueryFunc != nil {
		return s.QueryFunc(ctx, operation, input)
	}
	v, ok := s.Queries[operation]
	if !ok {
		return ui.QueryResult{}, backend.Fail("permission_denied")
	}
	return v, nil
}
