package bff

import (
	"bytes"
	"context"
	"encoding/json"
	api "github.com/araihu/balemoh/client"
	"io"
	"math"
	"net/http"
	"reflect"
	"strconv"
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
	limit := int64(1 << 20)
	if _, state := any(out).(api.LayoutState); r.StatusCode == 200 && !state {
		// Preview repeats IDs/labels and expands JSON escapes; active defaults add members.
		limit = 16 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if e != nil {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Upstream response was not confirmed."}, uncertain: true}
	}
	if int64(len(raw)) > limit {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "response_too_large", Message: "Layout response is too large for this editor. Preserve your draft and manage this catalog through the API."}, uncertain: true}
	}
	if !uniqueLayoutResponseJSON(raw) {
		return out, unconfirmedLayout()
	}
	if r.StatusCode != 200 {
		failure := &layoutFailure{status: r.StatusCode, detail: api.ErrorResponse{Code: "unavailable", Message: "Layout request could not be completed."}}
		var detail api.ErrorResponse
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		decoded := decoder.Decode(&detail) == nil && decoder.Decode(new(any)) == io.EOF && detail.Message != "" && detail.Code != ""
		if decoded {
			failure.detail = detail
		}
		failure.uncertain = !decoded || !documentedLayoutFailure(r.StatusCode, detail.Code)
		return out, failure
	}
	if json.Unmarshal(raw, &out) != nil {
		return out, &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Invalid upstream layout response."}, uncertain: true}
	}
	if state, ok := any(out).(api.LayoutState); ok && (!completeLayoutJSON(raw, reflect.TypeFor[api.LayoutState]()) || !validLayoutState(state)) {
		return out, unconfirmedLayout()
	}
	if preview, ok := any(out).(api.LayoutPreview); ok && (!completeLayoutJSON(raw, reflect.TypeFor[api.LayoutPreview]()) || !validLayoutPreview(preview)) {
		return out, unconfirmedLayout()
	}
	return out, nil
}
func (c *APIClient) DashboardLayout(ctx context.Context) (api.LayoutState, error) {
	return layoutResponse[api.LayoutState](c.client.GetDashboardLayout(ctx))
}
func (c *APIClient) PreviewLayout(ctx context.Context, r json.RawMessage) (api.LayoutPreview, error) {
	preview, err := layoutResponse[api.LayoutPreview](c.client.PreviewDashboardLayoutWithBody(ctx, "application/json", bytes.NewReader(r)))
	if err != nil {
		return preview, err
	}
	var request api.LayoutPreviewRequest
	if json.Unmarshal(r, &request) != nil || preview.RequestId != request.RequestId || preview.DraftGeneration != request.DraftGeneration || preview.Stale != (preview.LayoutRevision != request.ExpectedRevision) || !sameLayoutDocument(request.Document, preview.Document) {
		return api.LayoutPreview{}, unconfirmedLayout()
	}
	return preview, nil
}
func (c *APIClient) SaveLayout(ctx context.Context, revision string, d json.RawMessage) (api.LayoutState, error) {
	state, err := layoutResponse[api.LayoutState](c.client.PutDashboardLayoutWithBody(ctx, &api.PutDashboardLayoutParams{IfMatch: `"layout-` + revision + `"`}, "application/json", bytes.NewReader(d)))
	if err != nil {
		return state, err
	}
	before, e := strconv.ParseInt(revision, 10, 64)
	after, _ := strconv.ParseInt(state.Revision, 10, 64)
	var submitted api.LayoutDocument
	if e != nil || (after != before && (before == math.MaxInt64 || after != before+1)) || json.Unmarshal(d, &submitted) != nil || !sameLayoutDocument(submitted, state.Document) {
		return api.LayoutState{}, unconfirmedLayout()
	}
	return state, nil
}
func (c *APIClient) DashboardView(ctx context.Context, defaults bool) (api.DashboardSnapshot, error) {
	var params *api.GetDashboardParams
	if defaults {
		v := api.GetDashboardParamsLayoutView("default")
		params = &api.GetDashboardParams{LayoutView: &v}
	}
	return layoutResponse[api.DashboardSnapshot](c.client.GetDashboard(ctx, params))
}
