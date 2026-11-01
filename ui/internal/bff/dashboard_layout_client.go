package bff

import (
	"bytes"
	"context"
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"io"
	"net/http"
)

type LayoutCatalog interface {
	DashboardLayout(context.Context) (api.LayoutState, error)
	PreviewLayout(context.Context, json.RawMessage) (api.LayoutPreview, error)
	SaveLayout(context.Context, string, json.RawMessage) (api.LayoutState, error)
}
type layoutFailure struct {
	status    int
	detail    api.ErrorResponse
	uncertain bool
}

func (e *layoutFailure) Error() string { return e.detail.Message }
func layoutResponse[T any](r *http.Response, err error) (T, error) {
	var out T
	if err != nil {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Result not confirmed. Preserve the draft and read the saved version."}, uncertain: true}
	}
	defer r.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Upstream response was not confirmed."}, uncertain: true}
	}
	if r.StatusCode != 200 {
		failure := &layoutFailure{status: r.StatusCode, detail: api.ErrorResponse{Code: "unavailable", Message: "Layout request could not be completed."}}
		_ = json.Unmarshal(raw, &failure.detail)
		return out, failure
	}
	if json.Unmarshal(raw, &out) != nil {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Invalid upstream layout response."}, uncertain: true}
	}
	return out, nil
}
func (c *APIClient) DashboardLayout(ctx context.Context) (api.LayoutState, error) {
	return layoutResponse[api.LayoutState](c.client.GetDashboardLayout(ctx))
}
func (c *APIClient) PreviewLayout(ctx context.Context, r json.RawMessage) (api.LayoutPreview, error) {
	return layoutResponse[api.LayoutPreview](c.client.PreviewDashboardLayoutWithBody(ctx, "application/json", bytes.NewReader(r)))
}
func (c *APIClient) SaveLayout(ctx context.Context, revision string, d json.RawMessage) (api.LayoutState, error) {
	return layoutResponse[api.LayoutState](c.client.PutDashboardLayoutWithBody(ctx, &api.PutDashboardLayoutParams{IfMatch: `"layout-` + revision + `"`}, "application/json", bytes.NewReader(d)))
}
func (c *APIClient) DashboardView(ctx context.Context, defaults bool) (api.DashboardSnapshot, error) {
	var params *api.GetDashboardParams
	if defaults {
		v := api.GetDashboardParamsLayoutView("default")
		params = &api.GetDashboardParams{LayoutView: &v}
	}
	return layoutResponse[api.DashboardSnapshot](c.client.GetDashboard(ctx, params))
}
