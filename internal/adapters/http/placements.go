package http

import (
	"encoding/json"
	api "github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/plugins"
	openapitypes "github.com/oapi-codegen/runtime/types"
	"io"
	"net/http"
)

func (h Handler) GetPluginWidgets(w http.ResponseWriter, r *http.Request) {
	if !h.pluginAuth(w, r) {
		return
	}
	rows, err := h.plugins.Placements(r.Context())
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"definitions": h.plugins.Definitions(), "placements": rows})
}
func (h Handler) PutPluginWidget(w http.ResponseWriter, r *http.Request, id openapitypes.UUID, params api.PutPluginWidgetParams) {
	if !h.pluginAuth(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (64<<10)+1024)
	var b struct {
		PluginID       string          `json:"pluginId"`
		ConnectionID   string          `json:"connectionId"`
		ContributionID string          `json:"contributionId"`
		Options        json.RawMessage `json:"options"`
		Enabled        *bool           `json:"enabled"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&b) != nil || d.Decode(new(any)) != io.EOF || b.Enabled == nil {
		writePluginFailure(w, plugins.Fail("invalid_argument"))
		return
	}
	match, err := parsePluginETag(params.IfMatch)
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	value, created, err := h.plugins.PutPlacement(r.Context(), id.String(), plugins.PlacementBody{PluginID: b.PluginID, ConnectionID: b.ConnectionID, ContributionID: b.ContributionID, Options: b.Options, Enabled: *b.Enabled}, match)
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	w.Header().Set("ETag", `"`+value.Revision+`"`)
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, value)
}
func (h Handler) DeletePluginWidget(w http.ResponseWriter, r *http.Request, id openapitypes.UUID, params api.DeletePluginWidgetParams) {
	if !h.pluginAuth(w, r) {
		return
	}
	match, err := parsePluginETag(params.IfMatch)
	if err == nil {
		err = h.plugins.DeletePlacement(r.Context(), id.String(), match)
	}
	if err != nil {
		writePluginFailure(w, err)
		return
	}
	w.WriteHeader(204)
}
func parsePluginETag(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	v := *value
	if len(v) < 3 || v[0] != '"' || v[len(v)-1] != '"' {
		return "", plugins.Fail("precondition_failed")
	}
	v = v[1 : len(v)-1]
	if len(v) > 19 || v[0] == '0' {
		return "", plugins.Fail("precondition_failed")
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return "", plugins.Fail("precondition_failed")
		}
	}
	return v, nil
}
